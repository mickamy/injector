package diag

import (
	"go/token"
	"strings"
	"testing"
)

func TestSeverity_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sev  Severity
		want string
	}{
		{name: "error", sev: SeverityError, want: "error"},
		{name: "warning", sev: SeverityWarning, want: "warning"},
		{name: "info", sev: SeverityInfo, want: "info"},
		{name: "unknown", sev: Severity(99), want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.sev.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDiag_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		diag Diag
		want string
	}{
		{
			name: "error with position",
			diag: Diag{
				Severity: SeverityError,
				Pos:      token.Position{Filename: "main.go", Line: 10, Column: 5},
				Message:  "something went wrong",
			},
			want: "main.go:10:5: error: something went wrong",
		},
		{
			name: "warning without position",
			diag: Diag{
				Severity: SeverityWarning,
				Message:  "be careful",
			},
			want: "warning: be careful",
		},
		{
			name: "with hints",
			diag: Diag{
				Severity: SeverityError,
				Pos:      token.Position{Filename: "x.go", Line: 1, Column: 1},
				Message:  "no provider for foo.Bar",
				Hints: []string{
					"candidates:",
					"  - foo.NewBar",
					"  - bar.NewBar",
				},
			},
			want: strings.Join([]string{
				"x.go:1:1: error: no provider for foo.Bar",
				"  candidates:",
				"    - foo.NewBar",
				"    - bar.NewBar",
			}, "\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.diag.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorf(t *testing.T) {
	t.Parallel()

	pos := token.Position{Filename: "x.go", Line: 1, Column: 2}
	d := Errorf(pos, "value=%d", 42)

	if d.Severity != SeverityError {
		t.Errorf("Severity = %v, want SeverityError", d.Severity)
	}
	if d.Pos != pos {
		t.Errorf("Pos = %v, want %v", d.Pos, pos)
	}
	if d.Message != "value=42" {
		t.Errorf("Message = %q, want %q", d.Message, "value=42")
	}
}

func TestDiag_WithHints_DoesNotMutateReceiver(t *testing.T) {
	t.Parallel()

	base := Diag{Hints: []string{"a"}}
	appended := base.WithHints("b", "c")

	if got, want := len(base.Hints), 1; got != want {
		t.Fatalf("base mutated: len(Hints) = %d, want %d", got, want)
	}
	if got := appended.Hints; len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("appended.Hints = %v, want [a b c]", got)
	}
}

func TestDiag_WithHints_NoArgsReturnsSame(t *testing.T) {
	t.Parallel()

	base := Diag{Message: "x"}
	got := base.WithHints()

	if got.Message != base.Message || len(got.Hints) != 0 {
		t.Errorf("WithHints() with no args modified the diag: %+v", got)
	}
}

func TestHasErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ds   []Diag
		want bool
	}{
		{name: "empty", ds: nil, want: false},
		{name: "warning only", ds: []Diag{{Severity: SeverityWarning}}, want: false},
		{name: "info only", ds: []Diag{{Severity: SeverityInfo}}, want: false},
		{name: "has error", ds: []Diag{{Severity: SeverityWarning}, {Severity: SeverityError}}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := HasErrors(tt.ds); got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ds   []Diag
		want string
	}{
		{name: "empty", ds: nil, want: ""},
		{
			name: "single",
			ds:   []Diag{{Severity: SeverityError, Message: "boom"}},
			want: "error: boom",
		},
		{
			name: "multiple",
			ds: []Diag{
				{Severity: SeverityError, Message: "first"},
				{Severity: SeverityWarning, Message: "second"},
			},
			want: "error: first\nwarning: second",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := Format(tt.ds); got != tt.want {
				t.Errorf("Format() = %q, want %q", got, tt.want)
			}
		})
	}
}
