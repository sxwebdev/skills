package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/sxwebdev/skills/internal/config"
)

const fetchTimeout = 15 * time.Second

type githubProvider struct{}

func (githubProvider) Match(raw string) bool {
	base, _, _ := splitFragment(raw)
	if strings.HasPrefix(base, "github:") {
		return true
	}
	if strings.Contains(base, "github.com") {
		return true
	}
	if strings.Contains(base, ":") || strings.HasPrefix(base, ".") || strings.HasPrefix(base, "/") {
		return false
	}
	// Bare shorthand defaults to GitHub: owner/repo[/subpath][@skill].
	return atSkillRe.MatchString(base) || subpathRe.MatchString(base) || shorthandRe.MatchString(base)
}

func (githubProvider) Parse(raw string) (Source, error) {
	base, ref, skill := splitFragment(raw)
	base = strings.TrimPrefix(base, "github:")

	if m := ghTreePathRe.FindStringSubmatch(base); m != nil {
		return githubSource(raw, m[1], m[2], firstNonEmpty(m[3], ref), m[4], skill), nil
	}
	if m := ghTreeRe.FindStringSubmatch(base); m != nil {
		return githubSource(raw, m[1], m[2], firstNonEmpty(m[3], ref), "", skill), nil
	}
	if m := ghRepoRe.FindStringSubmatch(base); m != nil {
		return githubSource(raw, m[1], m[2], ref, "", skill), nil
	}
	if m := atSkillRe.FindStringSubmatch(base); m != nil {
		return githubSource(raw, m[1], m[2], ref, "", firstNonEmpty(skill, m[3])), nil
	}
	if m := subpathRe.FindStringSubmatch(base); m != nil {
		return githubSource(raw, m[1], m[2], ref, m[3], skill), nil
	}
	if m := shorthandRe.FindStringSubmatch(base); m != nil {
		return githubSource(raw, m[1], m[2], ref, "", skill), nil
	}
	return Source{}, fmt.Errorf("unrecognized github source: %q", raw)
}

func githubSource(raw, owner, repo, ref, subpath, skill string) Source {
	repo = strings.TrimSuffix(repo, ".git")
	ownerRepo := owner + "/" + repo
	return Source{
		Kind:        KindGitHub,
		Raw:         raw,
		CloneURL:    "https://github.com/" + ownerRepo + ".git",
		Host:        "github.com",
		OwnerRepo:   ownerRepo,
		Ref:         ref,
		Subpath:     subpath,
		SkillFilter: skill,
		Alias:       ownerRepo,
	}
}

// Fetch tries the GitHub Trees API fast-path (no full clone). On any failure
// it falls back to a shallow clone.
func (githubProvider) Fetch(ctx context.Context, src Source) (Fetched, error) {
	if f, err := fetchGitHubFast(ctx, src); err == nil {
		return f, nil
	}
	return cloneFetch(ctx, src)
}

type ghTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "blob" | "tree"
	SHA  string `json:"sha"`
}

type ghTree struct {
	SHA  string        `json:"sha"`
	Tree []ghTreeEntry `json:"tree"`
}

// fetchGitHubFast materializes skill folders from a GitHub repo using the Trees
// API + raw.githubusercontent.com, avoiding a full clone. The returned Fetched
// uses git-tree SHAs for change detection.
func fetchGitHubFast(ctx context.Context, src Source) (Fetched, error) {
	branches := []string{"HEAD", "main", "master"}
	if src.Ref != "" {
		branches = []string{src.Ref}
	}

	var tree *ghTree
	var branch string
	for _, b := range branches {
		t, err := fetchTree(ctx, src.OwnerRepo, b)
		if err == nil && t != nil {
			tree, branch = t, b
			break
		}
	}
	if tree == nil {
		return Fetched{}, fmt.Errorf("github fast-path: tree unavailable")
	}

	// Map skill folder path -> tree SHA, and collect blob paths to download.
	folderSHA := map[string]string{}
	shaByPath := map[string]string{}
	var skillFolders []string
	for _, e := range tree.Tree {
		shaByPath[e.Path] = e.SHA
	}
	for _, e := range tree.Tree {
		if e.Type == "blob" && strings.EqualFold(path.Base(e.Path), "SKILL.md") {
			folder := path.Dir(e.Path)
			if folder == "." {
				folder = ""
			}
			if !isSkillContainerPath(folder) {
				continue
			}
			skillFolders = append(skillFolders, folder)
			if folder == "" {
				folderSHA[folder] = tree.SHA
			} else if sha, ok := shaByPath[folder]; ok {
				folderSHA[folder] = sha
			}
		}
	}
	if len(skillFolders) == 0 {
		return Fetched{}, fmt.Errorf("github fast-path: no skills found")
	}

	tmpDir, err := os.MkdirTemp("", "skills-gh-*")
	if err != nil {
		return Fetched{}, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	// Download every blob that lives under a discovered skill folder.
	prefixes := make([]string, 0, len(skillFolders))
	for _, f := range skillFolders {
		prefixes = append(prefixes, f+"/")
	}
	for _, e := range tree.Tree {
		if e.Type != "blob" || !underAnyPrefix(e.Path, prefixes) {
			continue
		}
		data, err := fetchRaw(ctx, src.OwnerRepo, branch, e.Path)
		if err != nil {
			cleanup()
			return Fetched{}, err
		}
		dst := filepath.Join(tmpDir, filepath.FromSlash(e.Path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			cleanup()
			return Fetched{}, err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			cleanup()
			return Fetched{}, err
		}
	}

	return Fetched{
		Dir:      tmpDir,
		Cleanup:  cleanup,
		HashKind: config.HashKindTreeSHA,
		FolderHash: func(skillPathInRepo string) (string, error) {
			key := strings.TrimSuffix(filepath.ToSlash(skillPathInRepo), "/")
			if sha, ok := folderSHA[key]; ok {
				return sha, nil
			}
			return "", fmt.Errorf("no tree sha for %q", skillPathInRepo)
		},
	}, nil
}

// isSkillContainerPath reports whether a skill folder lives in a recognized
// container (repo root, skills/, .agents/skills/, .<agent>/skills/).
func isSkillContainerPath(folder string) bool {
	if folder == "" {
		return true
	}
	if strings.HasPrefix(folder, "skills/") || folder == "skills" {
		return true
	}
	if strings.Contains(folder, ".agents/skills/") || strings.HasSuffix(folder, ".agents/skills") {
		return true
	}
	// .<agent>/skills/<name>
	parts := strings.Split(folder, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, ".") && i+1 < len(parts) && parts[i+1] == "skills" {
			return true
		}
	}
	return false
}

func underAnyPrefix(p string, prefixes []string) bool {
	for _, pre := range prefixes {
		if strings.HasPrefix(p, pre) {
			return true
		}
	}
	return false
}

func fetchTree(ctx context.Context, ownerRepo, branch string) (*ghTree, error) {
	u := fmt.Sprintf("https://api.github.com/repos/%s/git/trees/%s?recursive=1", ownerRepo, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "skills-cli")

	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github trees api: status %d", resp.StatusCode)
	}
	var t ghTree
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

func fetchRaw(ctx context.Context, ownerRepo, branch, p string) ([]byte, error) {
	u := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", ownerRepo, branch, p)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "skills-cli")
	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("raw fetch %s: status %d", p, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 5<<20))
}
