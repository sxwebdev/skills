package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout for the duration of fn and returns what was
// written. runUse writes the prompt straight to os.Stdout, so this is how we
// observe its output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return capture(t, &os.Stdout, fn)
}

// captureStderr is the os.Stderr counterpart; ui.Error writes there (e.g. the
// issues `doctor` reports).
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return capture(t, &os.Stderr, fn)
}

func capture(t *testing.T, stream **os.File, fn func()) string {
	t.Helper()
	orig := *stream
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	*stream = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	_ = w.Close()
	*stream = orig
	return <-done
}

func TestRunUse(t *testing.T) {
	t.Run("single skill prints its SKILL.md", func(t *testing.T) {
		repo := writeSkillRepo(t, map[string]string{"only": "the only one"})
		var err error
		out := captureStdout(t, func() { err = runUseArgs(t, repo) })
		if err != nil {
			t.Fatalf("runUse: %v", err)
		}
		if !strings.Contains(out, "name: only") {
			t.Errorf("output missing skill body:\n%s", out)
		}
		if !strings.HasSuffix(out, "\n") {
			t.Error("output should end with a newline")
		}
	})

	t.Run("selects by @skill among many", func(t *testing.T) {
		repo := writeSkillRepo(t, map[string]string{"a": "A", "b": "B"})
		out := captureStdout(t, func() {
			if err := runUseArgs(t, repo+"@b"); err != nil {
				t.Errorf("runUse: %v", err)
			}
		})
		if !strings.Contains(out, "name: b") || strings.Contains(out, "name: a") {
			t.Errorf("expected only skill b, got:\n%s", out)
		}
	})

	t.Run("ambiguous source without selector errors", func(t *testing.T) {
		repo := writeSkillRepo(t, map[string]string{"a": "A", "b": "B"})
		err := runUseArgs(t, repo)
		if err == nil || !strings.Contains(err.Error(), "multiple skills") {
			t.Errorf("err = %v, want multiple-skills error", err)
		}
	})

	t.Run("unknown skill name errors", func(t *testing.T) {
		repo := writeSkillRepo(t, map[string]string{"a": "A"})
		err := runUseArgs(t, repo+"@nope")
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("err = %v, want not-found error", err)
		}
	})

	t.Run("empty source errors", func(t *testing.T) {
		empty := t.TempDir()
		err := runUseArgs(t, empty)
		if err == nil || !strings.Contains(err.Error(), "no skills found") {
			t.Errorf("err = %v, want no-skills error", err)
		}
	})

	t.Run("missing argument errors", func(t *testing.T) {
		err := runUseArgs(t)
		if err == nil || !strings.Contains(err.Error(), "usage") {
			t.Errorf("err = %v, want usage error", err)
		}
	})
}

// runUseArgs invokes the real `use` command with the given positional args.
func runUseArgs(t *testing.T, args ...string) error {
	t.Helper()
	return UseCmd().Run(t.Context(), append([]string{"use"}, args...))
}
