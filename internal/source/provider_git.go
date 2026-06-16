package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sxwebdev/skills/internal/config"
	"github.com/sxwebdev/skills/internal/gitutil"
)

// cloneFetch clones src.CloneURL (optionally at src.Ref) and returns a Fetched
// whose FolderHash hashes the on-disk skill folder. Shared by the generic git
// and GitLab providers, and used as the GitHub fast-path fallback.
func cloneFetch(ctx context.Context, src Source) (Fetched, error) {
	dir, err := gitutil.Clone(ctx, src.CloneURL, src.Ref)
	if err != nil {
		return Fetched{}, err
	}
	return clonedDirFetched(dir, func() { _ = os.RemoveAll(dir) }), nil
}

// clonedDirFetched builds a Fetched over an on-disk repo directory.
func clonedDirFetched(dir string, cleanup func()) Fetched {
	return Fetched{
		Dir:     dir,
		Cleanup: cleanup,
		FolderHash: func(skillPathInRepo string) (string, string, error) {
			h, err := gitutil.ComputeFolderHash(filepath.Join(dir, skillPathInRepo))
			return h, config.HashKindSHA1, err
		},
	}
}

// ---- local ----

type localProvider struct{}

func (localProvider) Match(raw string) bool { return isLocalPath(raw) }

func (localProvider) Parse(raw string) (Source, error) {
	base, skill := splitSkillSuffix(raw)
	abs, err := filepath.Abs(base)
	if err != nil {
		return Source{}, fmt.Errorf("resolve local path: %w", err)
	}
	return Source{
		Kind:        KindLocal,
		Raw:         raw,
		LocalDir:    abs,
		SkillFilter: skill,
		Alias:       filepath.Base(abs),
	}, nil
}

func (localProvider) Fetch(_ context.Context, src Source) (Fetched, error) {
	if info, err := os.Stat(src.LocalDir); err != nil || !info.IsDir() {
		return Fetched{}, fmt.Errorf("local source %q is not a directory", src.LocalDir)
	}
	// No temp resources to clean up for a local directory.
	return clonedDirFetched(src.LocalDir, func() {}), nil
}

// ---- gitlab ----

type gitlabProvider struct{}

func (gitlabProvider) Match(raw string) bool {
	base, _, _ := splitFragment(raw)
	// SSH/scp-style URLs (git@host:..., ssh://...) are handled by the generic
	// git provider, which clones via system git and honors the user's SSH keys.
	if strings.HasPrefix(base, "git@") || strings.HasPrefix(base, "ssh://") {
		return false
	}
	if strings.HasPrefix(base, "gitlab:") {
		return true
	}
	if strings.Contains(base, "gitlab.com") {
		return true
	}
	// Self-hosted GitLab is recognized by the unique "/-/tree/" path pattern.
	if (glTreePathRe.MatchString(base) || glTreeRe.MatchString(base)) && !strings.Contains(base, "github.com") {
		return true
	}
	return false
}

func (gitlabProvider) Parse(raw string) (Source, error) {
	base, ref, skill := splitFragment(raw)

	if rest, ok := strings.CutPrefix(base, "gitlab:"); ok {
		base = "https://gitlab.com/" + rest
	}

	if m := glTreePathRe.FindStringSubmatch(base); m != nil {
		repoPath := strings.TrimSuffix(m[3], ".git")
		return gitlabSource(raw, m[2], repoPath, firstNonEmpty(m[4], ref), m[5], skill), nil
	}
	if m := glTreeRe.FindStringSubmatch(base); m != nil {
		repoPath := strings.TrimSuffix(m[3], ".git")
		return gitlabSource(raw, m[2], repoPath, firstNonEmpty(m[4], ref), "", skill), nil
	}
	if m := glComRepoRe.FindStringSubmatch(base); m != nil {
		repoPath := m[1]
		if !strings.Contains(repoPath, "/") {
			return Source{}, fmt.Errorf("gitlab source must be group/repo: %q", raw)
		}
		return gitlabSource(raw, "gitlab.com", repoPath, ref, "", skill), nil
	}
	return Source{}, fmt.Errorf("unrecognized gitlab source: %q", raw)
}

func (gitlabProvider) Fetch(ctx context.Context, src Source) (Fetched, error) {
	return cloneFetch(ctx, src)
}

func gitlabSource(raw, host, repoPath, ref, subpath, skill string) Source {
	return Source{
		Kind:        KindGitLab,
		Raw:         raw,
		CloneURL:    fmt.Sprintf("https://%s/%s.git", host, repoPath),
		Host:        host,
		OwnerRepo:   repoPath,
		Ref:         ref,
		Subpath:     subpath,
		SkillFilter: skill,
		Alias:       repoPath,
	}
}

// ---- generic git (fallback): Gitea, OneDev, self-hosted, SSH ----

type gitProvider struct{}

func (gitProvider) Match(raw string) bool {
	base, _, _ := splitFragment(raw)
	base, _ = splitSkillSuffix(base)
	if strings.HasPrefix(base, "git@") || strings.HasPrefix(base, "ssh://") {
		return true
	}
	if (strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://")) && strings.HasSuffix(base, ".git") {
		return true
	}
	return false
}

func (gitProvider) Parse(raw string) (Source, error) {
	base, ref, skill := splitFragment(raw)
	base, suffixSkill := splitSkillSuffix(base)
	skill = firstNonEmpty(skill, suffixSkill)

	if m := sshGitRe.FindStringSubmatch(base); m != nil {
		return gitSource(raw, base, m[1], m[2], ref, skill), nil
	}
	if m := sshSchemeRe.FindStringSubmatch(base); m != nil {
		return gitSource(raw, base, m[1], m[2], ref, skill), nil
	}
	// HTTP(S) .git URL.
	host := hostOf(base)
	ownerRepo := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(base, "https://"+host+"/"), "http://"+host+"/"), ".git")
	return gitSource(raw, base, host, ownerRepo, ref, skill), nil
}

func (gitProvider) Fetch(ctx context.Context, src Source) (Fetched, error) {
	return cloneFetch(ctx, src)
}

func gitSource(raw, cloneURL, host, ownerRepo, ref, skill string) Source {
	alias := ownerRepo
	if alias == "" {
		alias = host
	}
	return Source{
		Kind:        KindGit,
		Raw:         raw,
		CloneURL:    cloneURL,
		Host:        host,
		OwnerRepo:   ownerRepo,
		Ref:         ref,
		SkillFilter: skill,
		Alias:       alias,
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
