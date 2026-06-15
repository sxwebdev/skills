package sanitize

import "testing"

func TestStripTerminalEscapes(t *testing.T) {
	t.Parallel()

	c1 := "a" + string(rune(0x9b)) + "b"

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello world", "hello world"},
		{"csi color", "\x1b[31mred\x1b[0m", "red"},
		{"csi cursor", "a\x1b[2Kb", "ab"},
		{"osc title bel", "x\x1b]0;evil\x07y", "xy"},
		{"osc title st", "x\x1b]8;;http://e\x1b\\y", "xy"},
		{"control chars", "a\x00b\x07c", "abc"},
		{"keeps tab newline", "a\tb\nc", "a\tb\nc"},
		{"c1 encoded", c1, "ab"},
		{"keeps unicode", "café déjà", "café déjà"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := StripTerminalEscapes(tt.in); got != tt.want {
				t.Errorf("StripTerminalEscapes(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"simple", "my-skill", "my-skill", false},
		{"dots and underscores", "my_skill.v2", "my_skill.v2", false},
		{"trims space", "  trimmed  ", "trimmed", false},
		{"empty", "", "", true},
		{"whitespace only", "   ", "", true},
		{"dot", ".", "", true},
		{"dotdot", "..", "", true},
		{"forward slash", "a/b", "", true},
		{"back slash", `a\b`, "", true},
		{"traversal", "../etc", "", true},
		{"null byte", "a\x00b", "", true},
		{"space inside", "a b", "", true},
		{"escape", "a\x1b[31mb", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Name(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Name(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Name(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsPathSafe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		base   string
		target string
		want   bool
	}{
		{"inside", "/home/u/.agents/skills", "/home/u/.agents/skills/foo", true},
		{"same dir", "/home/u/skills", "/home/u/skills", true},
		{"nested", "/base", "/base/a/b/c", true},
		{"escape parent", "/home/u/skills", "/home/u/skills/../evil", false},
		{"sibling", "/home/u/skills", "/home/u/other", false},
		{"absolute elsewhere", "/base", "/etc/passwd", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsPathSafe(tt.base, tt.target); got != tt.want {
				t.Errorf("IsPathSafe(%q, %q) = %v, want %v", tt.base, tt.target, got, tt.want)
			}
		})
	}
}
