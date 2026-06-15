package source

import "testing"

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        string
		kind      Kind
		cloneURL  string
		ownerRepo string
		ref       string
		subpath   string
		skill     string
	}{
		{
			name: "github shorthand", in: "owner/repo",
			kind: KindGitHub, cloneURL: "https://github.com/owner/repo.git", ownerRepo: "owner/repo",
		},
		{
			name: "github shorthand with skill", in: "owner/repo@my-skill",
			kind: KindGitHub, cloneURL: "https://github.com/owner/repo.git", ownerRepo: "owner/repo", skill: "my-skill",
		},
		{
			name: "github shorthand with subpath", in: "owner/repo/skills/foo",
			kind: KindGitHub, cloneURL: "https://github.com/owner/repo.git", ownerRepo: "owner/repo", subpath: "skills/foo",
		},
		{
			name: "github prefix", in: "github:owner/repo",
			kind: KindGitHub, cloneURL: "https://github.com/owner/repo.git", ownerRepo: "owner/repo",
		},
		{
			name: "github url", in: "https://github.com/owner/repo",
			kind: KindGitHub, cloneURL: "https://github.com/owner/repo.git", ownerRepo: "owner/repo",
		},
		{
			name: "github url .git", in: "https://github.com/owner/repo.git",
			kind: KindGitHub, cloneURL: "https://github.com/owner/repo.git", ownerRepo: "owner/repo",
		},
		{
			name: "github tree branch", in: "https://github.com/owner/repo/tree/dev",
			kind: KindGitHub, cloneURL: "https://github.com/owner/repo.git", ownerRepo: "owner/repo", ref: "dev",
		},
		{
			name: "github tree branch + path", in: "https://github.com/owner/repo/tree/dev/skills/foo",
			kind: KindGitHub, cloneURL: "https://github.com/owner/repo.git", ownerRepo: "owner/repo", ref: "dev", subpath: "skills/foo",
		},
		{
			name: "github fragment ref", in: "owner/repo#v1.2.3",
			kind: KindGitHub, cloneURL: "https://github.com/owner/repo.git", ownerRepo: "owner/repo", ref: "v1.2.3",
		},
		{
			name: "gitlab prefix", in: "gitlab:group/repo",
			kind: KindGitLab, cloneURL: "https://gitlab.com/group/repo.git", ownerRepo: "group/repo",
		},
		{
			name: "gitlab.com subgroups", in: "https://gitlab.com/group/sub/repo",
			kind: KindGitLab, cloneURL: "https://gitlab.com/group/sub/repo.git", ownerRepo: "group/sub/repo",
		},
		{
			name: "gitlab self-hosted tree", in: "https://git.example.com/group/repo/-/tree/main/skills",
			kind: KindGitLab, cloneURL: "https://git.example.com/group/repo.git", ownerRepo: "group/repo", ref: "main", subpath: "skills",
		},
		{
			name: "generic git https .git", in: "https://git.example.com/o/r.git",
			kind: KindGit, cloneURL: "https://git.example.com/o/r.git", ownerRepo: "o/r",
		},
		{
			name: "generic git ssh", in: "git@gitea.example.com:o/r.git",
			kind: KindGit, cloneURL: "git@gitea.example.com:o/r.git", ownerRepo: "o/r",
		},
		{
			name: "generic git ssh scheme", in: "ssh://git@host:7999/o/r.git",
			kind: KindGit, cloneURL: "ssh://git@host:7999/o/r.git", ownerRepo: "o/r",
		},
		{
			name: "well-known", in: "https://example.com/docs",
			kind: KindWellKnown, cloneURL: "https://example.com/docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.in, err)
			}
			if got.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", got.Kind, tt.kind)
			}
			if tt.cloneURL != "" && got.CloneURL != tt.cloneURL {
				t.Errorf("CloneURL = %q, want %q", got.CloneURL, tt.cloneURL)
			}
			if tt.ownerRepo != "" && got.OwnerRepo != tt.ownerRepo {
				t.Errorf("OwnerRepo = %q, want %q", got.OwnerRepo, tt.ownerRepo)
			}
			if got.Ref != tt.ref {
				t.Errorf("Ref = %q, want %q", got.Ref, tt.ref)
			}
			if got.Subpath != tt.subpath {
				t.Errorf("Subpath = %q, want %q", got.Subpath, tt.subpath)
			}
			if got.SkillFilter != tt.skill {
				t.Errorf("SkillFilter = %q, want %q", got.SkillFilter, tt.skill)
			}
		})
	}
}

func TestParseLocal(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"./local", "../up", "/abs/path", "."} {
		got, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", in, err)
		}
		if got.Kind != KindLocal {
			t.Errorf("Parse(%q).Kind = %v, want KindLocal", in, got.Kind)
		}
		if got.LocalDir == "" {
			t.Errorf("Parse(%q).LocalDir empty", in)
		}
	}
}

func TestParseSkillSuffix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		in    string
		kind  Kind
		skill string
	}{
		{"local @skill", "./src@demo", KindLocal, "demo"},
		{"github shorthand @skill", "owner/repo@pdf", KindGitHub, "pdf"},
		{"generic git .git @skill", "https://git.example.com/o/r.git@thing", KindGit, "thing"},
		{"ssh not split", "git@github.com:o/r.git", KindGit, ""},
		{"ssh scheme not split", "ssh://git@host:22/o/r.git", KindGit, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.in, err)
			}
			if got.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", got.Kind, tt.kind)
			}
			if got.SkillFilter != tt.skill {
				t.Errorf("SkillFilter = %q, want %q", got.SkillFilter, tt.skill)
			}
		})
	}
}

func TestFindNoMatch(t *testing.T) {
	t.Parallel()
	// An empty string matches nothing.
	if p := Find(""); p != nil {
		t.Errorf("Find(\"\") = %T, want nil", p)
	}
}
