// Package ui provides small colorized output helpers for command output.
// All untrusted text (skill names/descriptions) is run through
// sanitize.StripTerminalEscapes before printing. Color is automatically
// disabled when stdout is not a TTY or NO_COLOR is set.
package ui

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/sxwebdev/skills/internal/sanitize"
)

var (
	green  = color.New(color.FgGreen)
	yellow = color.New(color.FgYellow)
	red    = color.New(color.FgRed)
	cyan   = color.New(color.FgCyan)
	bold   = color.New(color.Bold)
	dim    = color.New(color.Faint)
)

// Success prints a green check line to stdout.
func Success(format string, a ...any) {
	_, _ = green.Fprint(os.Stdout, "✓ ")
	_, _ = fmt.Fprintln(os.Stdout, sanitize.StripTerminalEscapes(fmt.Sprintf(format, a...)))
}

// Warn prints a yellow warning line to stdout.
func Warn(format string, a ...any) {
	_, _ = yellow.Fprint(os.Stdout, "⚠ ")
	_, _ = fmt.Fprintln(os.Stdout, sanitize.StripTerminalEscapes(fmt.Sprintf(format, a...)))
}

// Error prints a red error line to stderr.
func Error(format string, a ...any) {
	_, _ = red.Fprint(os.Stderr, "✗ ")
	_, _ = fmt.Fprintln(os.Stderr, sanitize.StripTerminalEscapes(fmt.Sprintf(format, a...)))
}

// Info prints a plain line to stdout.
func Info(format string, a ...any) {
	_, _ = fmt.Fprintln(os.Stdout, sanitize.StripTerminalEscapes(fmt.Sprintf(format, a...)))
}

// Heading prints a bold section header to stdout.
func Heading(s string) {
	_, _ = bold.Fprintln(os.Stdout, sanitize.StripTerminalEscapes(s))
}

// Skill prints a formatted, sanitized skill row. desc/extra may be empty.
func Skill(name, desc, extra string) {
	_, _ = cyan.Fprint(os.Stdout, "  "+sanitize.StripTerminalEscapes(name))
	if extra != "" {
		_, _ = dim.Fprint(os.Stdout, "  "+sanitize.StripTerminalEscapes(extra))
	}
	_, _ = fmt.Fprintln(os.Stdout)
	if desc != "" {
		_, _ = dim.Fprintln(os.Stdout, "    "+sanitize.StripTerminalEscapes(desc))
	}
}
