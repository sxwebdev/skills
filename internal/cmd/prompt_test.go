package cmd

import (
	"strings"
	"testing"
)

func TestPromptBuild(t *testing.T) {
	t.Run("substitutes name and default description", func(t *testing.T) {
		out := captureStdout(t, func() {
			if err := PromptCmd().Run(t.Context(), []string{"prompt", "build", "my-thing"}); err != nil {
				t.Fatal(err)
			}
		})
		if strings.Contains(out, "{SKILL_NAME}") {
			t.Error("SKILL_NAME placeholder not substituted")
		}
		if !strings.Contains(out, "my-thing") {
			t.Error("skill name missing from output")
		}
		if !strings.Contains(out, "[Describe what this skill does") {
			t.Error("default description placeholder missing")
		}
	})

	t.Run("uses provided description", func(t *testing.T) {
		out := captureStdout(t, func() {
			if err := PromptCmd().Run(t.Context(), []string{"prompt", "build", "thing", "-d", "does a thing"}); err != nil {
				t.Fatal(err)
			}
		})
		if !strings.Contains(out, "does a thing") {
			t.Error("provided description missing from output")
		}
		if strings.Contains(out, "{SKILL_DESCRIPTION}") {
			t.Error("SKILL_DESCRIPTION placeholder not substituted")
		}
	})

	t.Run("missing name errors", func(t *testing.T) {
		err := PromptCmd().Run(t.Context(), []string{"prompt", "build"})
		if err == nil || !strings.Contains(err.Error(), "usage") {
			t.Errorf("err = %v, want usage error", err)
		}
	})
}
