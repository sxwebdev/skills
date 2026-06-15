// Package source parses skill source specifiers (GitHub/GitLab/generic git/
// well-known/local) and resolves them into a local directory that can be
// scanned for skills. It is organized as a provider registry so new hosts can
// be added without touching call sites.
package source

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// fetchTimeout bounds individual HTTP fetches (tree/raw/well-known).
const fetchTimeout = 15 * time.Second

// httpClient is shared so connections are pooled across the many sequential
// raw/tree/artifact fetches a single install performs.
var httpClient = &http.Client{Timeout: fetchTimeout}

// Kind identifies the type of a parsed source.
type Kind int

const (
	KindLocal Kind = iota
	KindGitHub
	KindGitLab
	KindGit // generic git host (Gitea, OneDev, self-hosted, ...)
	KindWellKnown
)

func (k Kind) String() string {
	switch k {
	case KindLocal:
		return "local"
	case KindGitHub:
		return "github"
	case KindGitLab:
		return "gitlab"
	case KindGit:
		return "git"
	case KindWellKnown:
		return "well-known"
	default:
		return "unknown"
	}
}

// Source is a parsed, normalized skill source.
type Source struct {
	Kind        Kind
	Raw         string // original input
	CloneURL    string // git clone URL (git kinds)
	LocalDir    string // absolute path (KindLocal)
	Host        string // hostname for git/well-known kinds
	OwnerRepo   string // "owner/repo" (or group/subgroup/repo) for git kinds
	Ref         string // pinned branch/tag (optional)
	Subpath     string // subpath within the repo (optional)
	SkillFilter string // single skill name to narrow to (optional)
	Alias       string // human-readable identifier (owner/repo or basename)
}

// Fetched is the result of materializing a Source into a local directory.
type Fetched struct {
	// Dir is a directory that registry.ScanRepo can scan. It may be the source
	// directory itself (local) or a temp clone.
	Dir string
	// Cleanup removes any temporary resources. Always non-nil.
	Cleanup func()
	// FolderHash returns the change-detection hash and its kind ("sha1" or
	// "tree-sha", matching config.HashKind*) for a skill folder given its path
	// relative to the repo root (e.g. "skills/my-skill"). Clone/local sources
	// return a SHA1 of the on-disk folder; GitHub additionally attaches the
	// git-tree SHA so `update` can detect changes without re-cloning.
	FolderHash func(skillPathInRepo string) (hash, kind string, err error)
}

// Provider matches, parses, and fetches a class of sources.
type Provider interface {
	// Match reports whether this provider handles the given raw input.
	Match(raw string) bool
	// Parse turns raw input into a normalized Source.
	Parse(raw string) (Source, error)
	// Fetch materializes the source locally for scanning.
	Fetch(ctx context.Context, src Source) (Fetched, error)
}

// registry holds providers in priority order. The first match wins.
var registry []Provider

// Register adds a provider to the registry. Order matters: register more
// specific providers before fallbacks.
func Register(p Provider) { registry = append(registry, p) }

func init() {
	Register(&localProvider{})
	Register(&githubProvider{})
	Register(&gitlabProvider{})
	Register(&wellKnownProvider{})
	Register(&gitProvider{}) // generic git fallback (must be last)
}

// Find returns the provider that handles raw, or nil.
func Find(raw string) Provider {
	for _, p := range registry {
		if p.Match(raw) {
			return p
		}
	}
	return nil
}

// Parse resolves raw input into a Source using the matching provider.
func Parse(raw string) (Source, error) {
	p := Find(raw)
	if p == nil {
		return Source{}, fmt.Errorf("unsupported source: %q", raw)
	}
	return p.Parse(raw)
}

// Fetch parses and materializes raw input in one step.
func Fetch(ctx context.Context, raw string) (Source, Fetched, error) {
	p := Find(raw)
	if p == nil {
		return Source{}, Fetched{}, fmt.Errorf("unsupported source: %q", raw)
	}
	src, err := p.Parse(raw)
	if err != nil {
		return Source{}, Fetched{}, err
	}
	f, err := p.Fetch(ctx, src)
	return src, f, err
}

// ---- shared parsing helpers ----

var (
	shorthandRe  = regexp.MustCompile(`^([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+)$`)
	atSkillRe    = regexp.MustCompile(`^([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+)@(.+)$`)
	subpathRe    = regexp.MustCompile(`^([a-zA-Z0-9_.-]+)/([a-zA-Z0-9_.-]+)/(.+)$`)
	ghTreePathRe = regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/tree/([^/]+)/(.+)`)
	ghTreeRe     = regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/tree/([^/]+)/?$`)
	ghRepoRe     = regexp.MustCompile(`github\.com/([^/]+)/([^/]+?)(?:\.git)?/?$`)
	glTreePathRe = regexp.MustCompile(`^(https?)://([^/]+)/(.+?)/-/tree/([^/]+)/(.+)`)
	glTreeRe     = regexp.MustCompile(`^(https?)://([^/]+)/(.+?)/-/tree/([^/]+)/?$`)
	glComRepoRe  = regexp.MustCompile(`gitlab\.com/(.+?)(?:\.git)?/?$`)
	sshGitRe     = regexp.MustCompile(`^git@([^:]+):(.+?)(?:\.git)?$`)
	sshSchemeRe  = regexp.MustCompile(`^ssh://(?:git@)?([^/:]+)(?::\d+)?/(.+?)(?:\.git)?$`)
)

// splitFragment extracts a trailing "#ref" or "#ref@skill" fragment.
func splitFragment(raw string) (base, ref, skill string) {
	base = raw
	if i := strings.Index(raw, "#"); i >= 0 {
		base = raw[:i]
		frag := raw[i+1:]
		if j := strings.Index(frag, "@"); j >= 0 {
			ref, skill = frag[:j], frag[j+1:]
		} else {
			ref = frag
		}
	}
	return base, ref, skill
}

// splitSkillSuffix splits a trailing "@<skill>" selector from raw, but only
// when the suffix is a bare skill name (no "/" or ":"), so SSH URLs such as
// git@host:owner/repo and ssh://git@host/... are left intact.
func splitSkillSuffix(raw string) (base, skill string) {
	if i := strings.LastIndex(raw, "@"); i > 0 {
		if s := raw[i+1:]; s != "" && !strings.ContainsAny(s, "/:") {
			return raw[:i], s
		}
	}
	return raw, ""
}

func isLocalPath(raw string) bool {
	if raw == "." || raw == ".." {
		return true
	}
	if strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") ||
		strings.HasPrefix(raw, ".\\") || strings.HasPrefix(raw, "..\\") {
		return true
	}
	if filepath.IsAbs(raw) {
		return true
	}
	// Windows drive path like C:\ or C:/
	if len(raw) >= 3 && raw[1] == ':' && (raw[2] == '\\' || raw[2] == '/') {
		return true
	}
	return false
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
