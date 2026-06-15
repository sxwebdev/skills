package gitutil

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha1"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// defaultCloneTimeout bounds a clone. Overridable via SKILLS_CLONE_TIMEOUT_MS.
const defaultCloneTimeout = 5 * time.Minute

func cloneTimeout() time.Duration {
	if raw := os.Getenv("SKILLS_CLONE_TIMEOUT_MS"); raw != "" {
		if ms, err := strconv.Atoi(raw); err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return defaultCloneTimeout
}

// cloneEnv hardens git for non-interactive, LFS-tolerant clones.
func cloneEnv() []string {
	return append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", // never block on a credential prompt
		"GIT_LFS_SKIP_SMUDGE=1", // don't pull LFS content during checkout
	)
}

// lfsConfigFlags disable the LFS filter so repos with LFS-tracked files clone
// even when git-lfs is not installed. Skills are plain text, never LFS-tracked.
var lfsConfigFlags = []string{
	"-c", "filter.lfs.required=false",
	"-c", "filter.lfs.smudge=",
	"-c", "filter.lfs.clean=",
	"-c", "filter.lfs.process=",
}

// CloneShallow clones a repo to a temp directory with depth=1 using system git.
// This ensures all user-configured authentication (SSH keys, credential helpers) works.
// Returns the path to the cloned repo. Caller must clean up.
func CloneShallow(ctx context.Context, url string) (string, error) {
	return Clone(ctx, url, "")
}

// Clone shallow-clones url (optionally at ref) into a temp directory with a
// hardened, non-interactive git invocation. On a GitHub HTTPS auth failure it
// retries via `gh repo clone` and then SSH. Caller must clean up the returned dir.
func Clone(ctx context.Context, url, ref string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, cloneTimeout())
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "skills-clone-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	args := []string{"clone", "--depth", "1", "--single-branch"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url, tmpDir)

	errMsg, err := runGit(ctx, args)
	if err == nil {
		return tmpDir, nil
	}

	// Auth fallbacks for GitHub HTTPS.
	if isAuthFailure(errMsg) && isGitHubHTTPS(url) {
		if owner, repo, ok := parseGitHubOwnerRepo(url); ok {
			if dir, ok := retryAuth(ctx, owner, repo, ref); ok {
				_ = os.RemoveAll(tmpDir)
				return dir, nil
			}
		}
	}

	if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
		return "", fmt.Errorf("clone %s: %w\n%s(also failed to clean up temp dir: %v)", url, err, errMsg, removeErr)
	}
	return "", fmt.Errorf("clone %s: %w\n%s", url, err, errMsg)
}

func runGit(ctx context.Context, args []string) (string, error) {
	full := append(append([]string{}, lfsConfigFlags...), args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Env = cloneEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stderr.String(), err
}

func retryAuth(ctx context.Context, owner, repo, ref string) (string, bool) {
	slug := owner + "/" + repo

	// Try `gh repo clone` (uses GitHub CLI auth) if gh is available.
	if _, err := exec.LookPath("gh"); err == nil {
		dir, derr := os.MkdirTemp("", "skills-clone-*")
		if derr == nil {
			ghArgs := []string{"repo", "clone", slug, dir, "--", "--depth=1"}
			if ref != "" {
				ghArgs = append(ghArgs, "--branch", ref)
			}
			cmd := exec.CommandContext(ctx, "gh", ghArgs...)
			cmd.Env = cloneEnv()
			if cmd.Run() == nil {
				return dir, true
			}
			_ = os.RemoveAll(dir)
		}
	}

	// Fall back to SSH.
	sshURL := fmt.Sprintf("git@github.com:%s.git", slug)
	dir, derr := os.MkdirTemp("", "skills-clone-*")
	if derr != nil {
		return "", false
	}
	args := []string{"clone", "--depth", "1", "--single-branch"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, sshURL, dir)
	if _, err := runGit(ctx, args); err == nil {
		return dir, true
	}
	_ = os.RemoveAll(dir)
	return "", false
}

func isAuthFailure(msg string) bool {
	lower := strings.ToLower(msg)
	for _, s := range []string{
		"authentication failed", "could not read username", "permission denied",
		"repository not found", "403", "saml sso", "enforced sso",
	} {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

func isGitHubHTTPS(url string) bool {
	return strings.HasPrefix(url, "https://github.com/")
}

func parseGitHubOwnerRepo(url string) (owner, repo string, ok bool) {
	rest := strings.TrimPrefix(url, "https://github.com/")
	rest = strings.TrimSuffix(rest, ".git")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// ComputeFolderHash computes a deterministic SHA1 hash of all files in a directory.
func ComputeFolderHash(dir string) (string, error) {
	type fileEntry struct {
		relPath string
		hash    string
	}

	var entries []fileEntry

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip .git directory
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		h := sha1.Sum(data)
		entries = append(entries, fileEntry{
			relPath: rel,
			hash:    fmt.Sprintf("%x", h),
		})
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk dir: %w", err)
	}

	slices.SortFunc(entries, func(a, b fileEntry) int {
		return cmp.Compare(a.relPath, b.relPath)
	})

	h := sha1.New()
	for _, e := range entries {
		if _, err := fmt.Fprintf(h, "%s:%s\n", e.relPath, e.hash); err != nil {
			return "", fmt.Errorf("hash write: %w", err)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
