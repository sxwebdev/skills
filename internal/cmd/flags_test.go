package cmd

import (
	"path/filepath"
	"testing"

	"github.com/sxwebdev/skills/internal/agents"
	"github.com/urfave/cli/v3"
)

func TestResolveAgents(t *testing.T) {
	t.Run("explicit flag wins and is canonicalized", func(t *testing.T) {
		cmdWith(t, []cli.Flag{agentFlag()}, []string{"--agent", "claude-code"}, func(c *cli.Command) {
			got, err := resolveAgents(c, newConfig())
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0] != "claude-code" {
				t.Errorf("got %v, want [claude-code]", got)
			}
		})
	})

	t.Run("unknown agent is an error", func(t *testing.T) {
		cmdWith(t, []cli.Flag{agentFlag()}, []string{"--agent", "nope"}, func(c *cli.Command) {
			_, err := resolveAgents(c, newConfig())
			if err == nil {
				t.Fatal("expected error for unknown agent")
			}
		})
	})

	t.Run("falls back to config agents", func(t *testing.T) {
		cmdWith(t, []cli.Flag{agentFlag()}, nil, func(c *cli.Command) {
			cfg := newConfig()
			cfg.Agents = []string{"cursor"}
			got, err := resolveAgents(c, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0] != "cursor" {
				t.Errorf("got %v, want [cursor]", got)
			}
		})
	})

	t.Run("no flag and empty config yields known agents", func(t *testing.T) {
		cmdWith(t, []cli.Flag{agentFlag()}, nil, func(c *cli.Command) {
			cfg := newConfig()
			cfg.Agents = nil
			got, err := resolveAgents(c, cfg)
			if err != nil {
				t.Fatal(err)
			}
			// Either auto-detected agents or the claude-code default; every name
			// must be a real agent regardless of which branch the environment hits.
			if len(got) == 0 {
				t.Fatal("expected at least one agent")
			}
			for _, name := range got {
				if _, ok := agents.Get(name); !ok {
					t.Errorf("resolved unknown agent %q", name)
				}
			}
		})
	})
}

func TestResolveScope(t *testing.T) {
	scopeFlags := func() []cli.Flag {
		return []cli.Flag{
			globalFlag(),
			&cli.StringFlag{Name: "project"},
			&cli.BoolFlag{Name: "local"},
		}
	}

	t.Run("global flag forces global scope", func(t *testing.T) {
		cmdWith(t, scopeFlags(), []string{"--global", "--project", "/somewhere"}, func(c *cli.Command) {
			got, err := resolveScope(c)
			if err != nil {
				t.Fatal(err)
			}
			if got != "" {
				t.Errorf("got %q, want global (empty)", got)
			}
		})
	})

	t.Run("project path resolves to an absolute root", func(t *testing.T) {
		cmdWith(t, scopeFlags(), []string{"--project", "some/rel/path"}, func(c *cli.Command) {
			got, err := resolveScope(c)
			if err != nil {
				t.Fatal(err)
			}
			want, _ := filepath.Abs("some/rel/path")
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	})

	t.Run("default is global", func(t *testing.T) {
		cmdWith(t, scopeFlags(), nil, func(c *cli.Command) {
			got, err := resolveScope(c)
			if err != nil {
				t.Fatal(err)
			}
			if got != "" {
				t.Errorf("got %q, want global (empty)", got)
			}
		})
	})
}
