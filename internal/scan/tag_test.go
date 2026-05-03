package scan_test

import (
	"testing"

	"github.com/mickamy/injector/internal/scan"
)

func TestParseTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    scan.ParsedTag
		wantErr string
	}{
		{name: "marker", in: "", want: scan.ParsedTag{Kind: scan.TagMarker}},
		{name: "marker with whitespace", in: "  ", want: scan.ParsedTag{Kind: scan.TagMarker}},
		{name: "with qualified", in: "with=foo.NewBar", want: scan.ParsedTag{Kind: scan.TagWith, With: "foo.NewBar"}},
		{name: "with bare func", in: "with=NewBar", want: scan.ParsedTag{Kind: scan.TagWith, With: "NewBar"}},
		{name: "with empty value", in: "with=", wantErr: `inject:"with=..." requires a provider reference`},
		{name: "with no equals", in: "with", wantErr: `inject:"with=..." requires a provider reference`},
		{name: "arg", in: "arg", want: scan.ParsedTag{Kind: scan.TagArg}},
		{name: "arg with name", in: "arg=primary", want: scan.ParsedTag{Kind: scan.TagArg, ArgName: "primary"}},
		{name: "arg empty name", in: "arg=", wantErr: `inject:"arg=..." requires a name`},
		{name: "returns", in: "returns", want: scan.ParsedTag{Kind: scan.TagReturns}},
		{name: "returns with value", in: "returns=foo", wantErr: `inject:"returns" does not take a value`},
		{name: "unknown bare", in: "xyz", wantErr: `unknown inject tag form "xyz"`},
		{name: "unknown kv", in: "xyz=1", wantErr: `unknown inject tag form "xyz=1"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := scan.ParseTag(tt.in)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseTag(%q) returned nil error, want %q", tt.in, tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Errorf("ParseTag(%q) error = %q, want %q", tt.in, err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseTag(%q) returned unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseTag(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}
