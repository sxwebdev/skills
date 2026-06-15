package source

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeArchivePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"SKILL.md", "SKILL.md", true},
		{"refs/a.md", "refs/a.md", true},
		{"./a/./b.md", "a/b.md", true},
		{`a\b.md`, "a/b.md", true},
		{"../escape", "", false},
		{"a/../../escape", "", false},
		{"/abs", "", false},
		{"C:/win", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, ok := safeArchivePath(tt.in)
			if ok != tt.ok {
				t.Fatalf("safeArchivePath(%q) ok = %v, want %v", tt.in, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("safeArchivePath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func makeTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractTarGz(t *testing.T) {
	t.Parallel()

	dst := t.TempDir()
	data := makeTarGz(t, map[string]string{
		"SKILL.md":      "---\nname: x\n---\n",
		"refs/guide.md": "guide",
	})
	if err := extractTarGz(data, dst); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dst, "SKILL.md")); err != nil || len(b) == 0 {
		t.Errorf("SKILL.md not extracted: %v", err)
	}
	if _, err := os.ReadFile(filepath.Join(dst, "refs", "guide.md")); err != nil {
		t.Errorf("refs/guide.md not extracted: %v", err)
	}
}

func TestExtractTarGzRejectsTraversal(t *testing.T) {
	t.Parallel()

	dst := t.TempDir()
	data := makeTarGz(t, map[string]string{"../evil.md": "x"})
	if err := extractTarGz(data, dst); err == nil {
		t.Fatal("extractTarGz accepted a path-traversal entry")
	}
}
