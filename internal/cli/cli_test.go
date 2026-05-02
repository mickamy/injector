package cli

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestApp_Version(t *testing.T) {
	t.Parallel()

	var out, err bytes.Buffer
	app := &App{Out: &out, Err: &err, Version: "v0.2.0"}

	code := app.Run([]string{"--version"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got := strings.TrimSpace(out.String()); got != "v0.2.0" {
		t.Errorf("out = %q, want v0.2.0", got)
	}
}

func TestApp_Help(t *testing.T) {
	t.Parallel()

	tests := []string{"-h", "--help", "help"}
	for _, flag := range tests {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()

			var out, err bytes.Buffer
			app := &App{Out: &out, Err: &err, Version: "v"}

			code := app.Run([]string{flag})
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			if !strings.Contains(out.String(), "Usage:") {
				t.Errorf("expected usage in stdout, got %q", out.String())
			}
		})
	}
}

func TestApp_NoArgs_ShowsUsageOnStderr(t *testing.T) {
	t.Parallel()

	var out, err bytes.Buffer
	app := &App{Out: &out, Err: &err}

	code := app.Run(nil)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(err.String(), "Usage:") {
		t.Errorf("expected usage in stderr, got %q", err.String())
	}
}

func TestApp_UnknownFlag(t *testing.T) {
	t.Parallel()

	var out, err bytes.Buffer
	app := &App{Out: &out, Err: &err}

	code := app.Run([]string{"--no-such-flag"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestSplitTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: nil},
		{name: "whitespace only", in: " ", want: nil},
		{name: "single", in: "foo", want: []string{"foo"}},
		{name: "multiple", in: "foo,bar", want: []string{"foo", "bar"}},
		{name: "trim and skip empty", in: " foo , bar ,, baz", want: []string{"foo", "bar", "baz"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := splitTags(tt.in)
			if !slices.Equal(got, tt.want) {
				t.Errorf("splitTags(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
