// Package sanitize provides defenses against untrusted skill metadata:
// terminal escape injection (CWE-150) and path traversal in skill names.
package sanitize

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Matches ANSI escape sequences we must strip before printing untrusted text:
//   - CSI:  ESC [ ... final-byte  (cursor movement, colors, etc.)
//   - OSC:  ESC ] ... (BEL | ESC \)  (window title, hyperlinks)
//   - other ESC-prefixed two-byte sequences.
var (
	csiRe = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	oscRe = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
	escRe = regexp.MustCompile(`\x1b[@-Z\\-_]`)
	// C0 (except \t \n \r) and C1 control characters.
	ctrlRe = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f\x7f-\x9f]`)
)

// StripTerminalEscapes removes ANSI/OSC escape sequences and control characters
// from a string so untrusted skill names/descriptions can be printed safely.
func StripTerminalEscapes(s string) string {
	s = oscRe.ReplaceAllString(s, "")
	s = csiRe.ReplaceAllString(s, "")
	s = escRe.ReplaceAllString(s, "")
	s = ctrlRe.ReplaceAllString(s, "")
	return s
}

var validName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// Name returns a path-traversal-safe, filesystem-safe skill name or an error.
// It rejects empty names, ".", "..", path separators, and control/escape characters.
func Name(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("empty skill name")
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("invalid skill name %q", raw)
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "\x00") {
		return "", fmt.Errorf("skill name %q contains path separators", raw)
	}
	if !validName.MatchString(name) {
		return "", fmt.Errorf("skill name %q contains invalid characters (allowed: letters, digits, . _ -)", raw)
	}
	return name, nil
}

// IsPathSafe reports whether target stays within base after cleaning,
// i.e. target does not escape base via ".." or an absolute path.
func IsPathSafe(base, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(base), filepath.Clean(target))
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return !filepath.IsAbs(rel)
}
