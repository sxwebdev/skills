package source

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sxwebdev/skills/internal/sanitize"
)

const (
	maxArchiveBytes = 50 << 20 // 50 MiB unpacked
	maxArchiveFiles = 1000
)

type wellKnownProvider struct{}

func (wellKnownProvider) Match(raw string) bool {
	base, _, _ := splitFragment(raw)
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		return false
	}
	if strings.HasSuffix(base, ".git") {
		return false
	}
	host := hostOf(base)
	return host != "github.com" && host != "gitlab.com" && host != "raw.githubusercontent.com"
}

func (wellKnownProvider) Parse(raw string) (Source, error) {
	base, _, skill := splitFragment(raw)
	return Source{
		Kind:        KindWellKnown,
		Raw:         raw,
		CloneURL:    base,
		Host:        hostOf(base),
		SkillFilter: skill,
		Alias:       hostOf(base),
	}, nil
}

// index.json shapes (v0.1.0 legacy and v0.2.0).
type wkIndex struct {
	Schema string    `json:"$schema"`
	Skills []wkEntry `json:"skills"`
}

type wkEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"` // v2: "skill-md" | "archive"
	URL         string   `json:"url"`  // v2
	Digest      string   `json:"digest"`
	Files       []string `json:"files"` // v1
}

func (wellKnownProvider) Fetch(ctx context.Context, src Source) (Fetched, error) {
	idx, indexURL, err := fetchWellKnownIndex(ctx, src.CloneURL)
	if err != nil {
		return Fetched{}, err
	}

	tmpDir, err := os.MkdirTemp("", "skills-wk-*")
	if err != nil {
		return Fetched{}, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	installed := 0
	for _, e := range idx.Skills {
		name, nerr := sanitize.Name(e.Name)
		if nerr != nil {
			continue
		}
		skillDir := filepath.Join(tmpDir, "skills", name)
		if err := materializeWellKnownSkill(ctx, indexURL, e, skillDir); err != nil {
			continue
		}
		installed++
	}
	if installed == 0 {
		cleanup()
		return Fetched{}, fmt.Errorf("well-known: no installable skills at %s", src.CloneURL)
	}

	return clonedDirFetched(tmpDir, cleanup), nil
}

func fetchWellKnownIndex(ctx context.Context, base string) (*wkIndex, string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, "", err
	}
	basePath := strings.TrimSuffix(u.Path, "/")
	candidates := []string{
		fmt.Sprintf("%s://%s%s/.well-known/agent-skills/index.json", u.Scheme, u.Host, basePath),
		fmt.Sprintf("%s://%s/.well-known/agent-skills/index.json", u.Scheme, u.Host),
		fmt.Sprintf("%s://%s%s/.well-known/skills/index.json", u.Scheme, u.Host, basePath),
		fmt.Sprintf("%s://%s/.well-known/skills/index.json", u.Scheme, u.Host),
	}
	for _, c := range candidates {
		data, err := httpGet(ctx, c)
		if err != nil {
			continue
		}
		var idx wkIndex
		if err := json.Unmarshal(data, &idx); err != nil || len(idx.Skills) == 0 {
			continue
		}
		return &idx, c, nil
	}
	return nil, "", fmt.Errorf("well-known index not found under %s", base)
}

func materializeWellKnownSkill(ctx context.Context, indexURL string, e wkEntry, skillDir string) error {
	if e.Type == "archive" || e.Type == "skill-md" {
		return materializeArtifact(ctx, indexURL, e, skillDir)
	}
	return materializeLegacy(ctx, indexURL, e, skillDir)
}

// materializeArtifact handles the v0.2.0 single-artifact model with digest check.
func materializeArtifact(ctx context.Context, indexURL string, e wkEntry, skillDir string) error {
	artifactURL, err := resolveRef(indexURL, e.URL)
	if err != nil {
		return err
	}
	data, err := httpGet(ctx, artifactURL)
	if err != nil {
		return err
	}
	if e.Digest != "" {
		want := strings.TrimPrefix(e.Digest, "sha256:")
		got := hex.EncodeToString(sha256Sum(data))
		if want != got {
			return fmt.Errorf("digest mismatch for %s", e.Name)
		}
	}
	if e.Type == "skill-md" {
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(skillDir, "SKILL.md"), data, 0o644)
	}
	return extractArchive(artifactURL, data, skillDir)
}

// materializeLegacy handles the v0.1.0 files[] model.
func materializeLegacy(ctx context.Context, indexURL string, e wkEntry, skillDir string) error {
	if len(e.Files) == 0 {
		return fmt.Errorf("no files for %s", e.Name)
	}
	baseURL := strings.TrimSuffix(indexURL, "/index.json") + "/" + url.PathEscape(e.Name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}
	for _, f := range e.Files {
		clean, ok := safeArchivePath(f)
		if !ok {
			return fmt.Errorf("unsafe file path %q", f)
		}
		data, err := httpGet(ctx, baseURL+"/"+clean)
		if err != nil {
			return err
		}
		dst := filepath.Join(skillDir, filepath.FromSlash(clean))
		if !sanitize.IsPathSafe(skillDir, dst) {
			return fmt.Errorf("unsafe file path %q", f)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func extractArchive(artifactURL string, data []byte, skillDir string) error {
	lower := strings.ToLower(artifactURL)
	switch {
	case bytes.HasPrefix(data, []byte{0x1f, 0x8b}) || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(data, skillDir)
	case bytes.HasPrefix(data, []byte("PK")) || strings.HasSuffix(lower, ".zip"):
		return extractZip(data, skillDir)
	default:
		return fmt.Errorf("unsupported archive format")
	}
}

func extractTarGz(data []byte, dstDir string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)

	total, count := 0, 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("archive links are not supported")
		case tar.TypeDir:
			continue
		case tar.TypeReg:
		default:
			continue
		}
		clean, ok := safeArchivePath(hdr.Name)
		if !ok {
			return fmt.Errorf("unsafe archive path %q", hdr.Name)
		}
		count++
		if count > maxArchiveFiles {
			return fmt.Errorf("archive contains too many files")
		}
		buf := &bytes.Buffer{}
		n, err := io.CopyN(buf, tr, maxArchiveBytes+1)
		if err != nil && err != io.EOF {
			return err
		}
		total += int(n)
		if total > maxArchiveBytes {
			return fmt.Errorf("archive exceeds maximum unpacked size")
		}
		if err := writeUnder(dstDir, clean, buf.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

func extractZip(data []byte, dstDir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	total, count := 0, 0
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive links are not supported")
		}
		clean, ok := safeArchivePath(f.Name)
		if !ok {
			return fmt.Errorf("unsafe archive path %q", f.Name)
		}
		count++
		if count > maxArchiveFiles {
			return fmt.Errorf("archive contains too many files")
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		buf := &bytes.Buffer{}
		n, err := io.CopyN(buf, rc, maxArchiveBytes+1)
		_ = rc.Close()
		if err != nil && err != io.EOF {
			return err
		}
		total += int(n)
		if total > maxArchiveBytes {
			return fmt.Errorf("archive exceeds maximum unpacked size")
		}
		if err := writeUnder(dstDir, clean, buf.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

// safeArchivePath normalizes and rejects unsafe archive member paths.
func safeArchivePath(raw string) (string, bool) {
	if raw == "" || strings.Contains(raw, "\x00") {
		return "", false
	}
	p := strings.ReplaceAll(raw, "\\", "/")
	if strings.HasPrefix(p, "/") {
		return "", false
	}
	if len(p) >= 2 && p[1] == ':' { // windows drive
		return "", false
	}
	parts := strings.Split(p, "/")
	var clean []string
	for _, seg := range parts {
		if seg == "" || seg == "." {
			continue
		}
		if seg == ".." {
			return "", false
		}
		clean = append(clean, seg)
	}
	if len(clean) == 0 {
		return "", false
	}
	return path.Join(clean...), true
}

func writeUnder(baseDir, relPath string, data []byte) error {
	dst := filepath.Join(baseDir, filepath.FromSlash(relPath))
	if !sanitize.IsPathSafe(baseDir, dst) {
		return fmt.Errorf("unsafe path %q", relPath)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func resolveRef(baseURL, ref string) (string, error) {
	b, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	r, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return b.ResolveReference(r).String(), nil
}

func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func httpGet(ctx context.Context, u string) ([]byte, error) {
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
		return nil, fmt.Errorf("GET %s: status %d", u, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxArchiveBytes+1))
}
