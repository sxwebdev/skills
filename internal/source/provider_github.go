package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sxwebdev/skills/internal/config"
)

// githubToken returns a token to authenticate GitHub API calls, raising the
// unauthenticated 60 req/hr limit to 5000/hr. It prefers GITHUB_TOKEN/GH_TOKEN
// and falls back to `gh auth token`, resolved at most once per process.
var (
	ghTokenOnce sync.Once
	ghTokenVal  string
)

func githubToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	if t := os.Getenv("GH_TOKEN"); t != "" {
		return t
	}
	ghTokenOnce.Do(func() {
		out, err := exec.Command("gh", "auth", "token").Output()
		if err == nil {
			ghTokenVal = strings.TrimSpace(string(out))
		}
	})
	return ghTokenVal
}

type githubProvider struct{}

func (githubProvider) Match(raw string) bool {
	base, _, _ := splitFragment(raw)
	// SSH and scheme-qualified git URLs (incl. git@github.com:…) are handled by
	// the generic git provider, which clones them directly.
	if strings.HasPrefix(base, "git@") || strings.HasPrefix(base, "ssh://") {
		return false
	}
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

// Fetch clones the repo for content — one packfile is far faster than fetching
// every skill file individually over HTTP. Best-effort, it then attaches cheap
// git-tree-SHA change detection so `update` can check for changes without
// re-cloning; if the GitHub API is unavailable (rate limit, private repo
// without token) it falls back to the clone's SHA1 folder hash.
func (githubProvider) Fetch(ctx context.Context, src Source) (Fetched, error) {
	f, err := cloneFetch(ctx, src)
	if err != nil {
		return Fetched{}, err
	}
	if tree := githubTree(ctx, src); tree != nil {
		idx := folderSHAIndex(tree)
		sha1Hash := f.FolderHash
		f.FolderHash = func(skillPathInRepo string) (string, string, error) {
			if sha, ok := idx[normalizeFolder(skillPathInRepo)]; ok {
				return sha, config.HashKindTreeSHA, nil
			}
			return sha1Hash(skillPathInRepo) // fallback for an unmapped folder
		}
	}
	return f, nil
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

func normalizeFolder(p string) string {
	return strings.TrimSuffix(filepath.ToSlash(p), "/")
}

// githubTree fetches the recursive tree for src, trying the pinned ref then
// HEAD/main/master. Returns nil on any failure (caller falls back to clone+SHA1).
func githubTree(ctx context.Context, src Source) *ghTree {
	branches := []string{"HEAD", "main", "master"}
	if src.Ref != "" {
		branches = []string{src.Ref}
	}
	for _, b := range branches {
		if t, err := fetchTree(ctx, src.OwnerRepo, b); err == nil && t != nil {
			return t
		}
	}
	return nil
}

// folderSHAIndex maps each directory path to its git-tree SHA, plus the repo
// root ("") to the top-level tree SHA.
func folderSHAIndex(tree *ghTree) map[string]string {
	idx := map[string]string{"": tree.SHA}
	for _, e := range tree.Tree {
		if e.Type == "tree" {
			idx[e.Path] = e.SHA
		}
	}
	return idx
}

// PeekFolderHashes returns current change-detection hashes keyed by skill
// folder path (slash-form, relative to the repo root) WITHOUT downloading any
// content, when the provider supports it (GitHub). The bool is false when no
// cheap peek is available and the caller must Fetch and hash on disk.
func PeekFolderHashes(ctx context.Context, src Source) (map[string]string, string, bool) {
	if src.Kind != KindGitHub {
		return nil, "", false
	}
	tree := githubTree(ctx, src)
	if tree == nil {
		return nil, "", false
	}
	return folderSHAIndex(tree), config.HashKindTreeSHA, true
}

func fetchTree(ctx context.Context, ownerRepo, branch string) (*ghTree, error) {
	u := fmt.Sprintf("https://api.github.com/repos/%s/git/trees/%s?recursive=1", ownerRepo, branch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "skills-cli")
	if tok := githubToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := httpClient.Do(req)
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
