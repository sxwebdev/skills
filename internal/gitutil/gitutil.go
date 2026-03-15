package gitutil

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// CloneShallow clones a repo to a temp directory with depth=1 using system git.
// This ensures all user-configured authentication (SSH keys, credential helpers) works.
// Returns the path to the cloned repo. Caller must clean up.
func CloneShallow(ctx context.Context, url string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "skills-clone-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--single-branch", url, tmpDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			return "", fmt.Errorf("clone %s: %w (also failed to clean up temp dir: %v)", url, err, removeErr)
		}
		return "", fmt.Errorf("clone %s: %w", url, err)
	}

	return tmpDir, nil
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

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})

	h := sha1.New()
	for _, e := range entries {
		if _, err := fmt.Fprintf(h, "%s:%s\n", e.relPath, e.hash); err != nil {
			return "", fmt.Errorf("hash write: %w", err)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
