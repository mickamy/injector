package packages

import (
	"strings"
	"testing"
)

func TestLoad_NoPatterns(t *testing.T) {
	t.Parallel()

	_, err := Load(nil, Config{})
	if err == nil {
		t.Fatal("Load(nil) returned nil error")
	}
	if !strings.Contains(err.Error(), "no package patterns") {
		t.Errorf("error = %q, want substring %q", err, "no package patterns")
	}
}

func TestJoinTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want string
	}{
		{name: "nil", in: nil, want: ""},
		{name: "empty slice", in: []string{}, want: ""},
		{name: "single", in: []string{"foo"}, want: "foo"},
		{name: "multiple", in: []string{"foo", "bar"}, want: "foo bar"},
		{name: "trim spaces", in: []string{" foo ", "bar"}, want: "foo bar"},
		{name: "skip empty", in: []string{"", "foo", "", "bar"}, want: "foo bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := joinTags(tt.in); got != tt.want {
				t.Errorf("joinTags(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
