package cmd

import (
	"fmt"
	"regexp"
	"strings"
)

var shorthandRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$`)

// NormalizeRepoURL normalizes a repo URL.
// "owner/repo" → "https://github.com/owner/repo.git"
// Ensures .git suffix for https URLs.
func NormalizeRepoURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty URL")
	}

	// Shorthand: owner/repo
	if shorthandRe.MatchString(raw) {
		return "https://github.com/" + raw + ".git", nil
	}

	// SSH format: git@github.com:owner/repo.git
	if strings.HasPrefix(raw, "git@") {
		return raw, nil
	}

	// HTTPS: ensure .git suffix
	if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") {
		if !strings.HasSuffix(raw, ".git") {
			raw += ".git"
		}
		return raw, nil
	}

	return "", fmt.Errorf("unsupported URL format: %s", raw)
}

// AliasFromURL extracts "owner/repo" from a git URL.
func AliasFromURL(url string) string {
	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Handle SSH URLs
	if strings.HasPrefix(url, "git@") {
		// git@github.com:owner/repo
		parts := strings.SplitN(url, ":", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}

	// Handle HTTPS URLs
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}

	return url
}
