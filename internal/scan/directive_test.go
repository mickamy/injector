package scan

import (
	"reflect"
	"slices"
	"testing"

	"github.com/mickamy/injector/internal/ir"
)

func TestParseDirective(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		lines    []string
		wantPD   ParsedDirective
		wantErrs []string
	}{
		{
			name: "no comments",
		},
		{
			name:  "no directive",
			lines: []string{"// this is just a doc"},
		},
		{
			name:   "marker only",
			lines:  []string{"//injector:container"},
			wantPD: ParsedDirective{Found: true},
		},
		{
			name:   "marker with leading space",
			lines:  []string{"// injector:container"},
			wantPD: ParsedDirective{Found: true},
		},
		{
			name:   "marker with trailing space",
			lines:  []string{"//injector:container  "},
			wantPD: ParsedDirective{Found: true},
		},
		{
			name:   "name=",
			lines:  []string{"//injector:container name=NewFoo"},
			wantPD: ParsedDirective{Found: true, Name: "NewFoo"},
		},
		{
			name:   "returns=",
			lines:  []string{"//injector:container returns=greeter.Greeter"},
			wantPD: ParsedDirective{Found: true, ReturnsExpr: "greeter.Greeter"},
		},
		{
			name:   "must shorthand",
			lines:  []string{"//injector:container must"},
			wantPD: ParsedDirective{Found: true, Must: ir.MustOn},
		},
		{
			name:   "must=true",
			lines:  []string{"//injector:container must=true"},
			wantPD: ParsedDirective{Found: true, Must: ir.MustOn},
		},
		{
			name:   "must=false",
			lines:  []string{"//injector:container must=false"},
			wantPD: ParsedDirective{Found: true, Must: ir.MustOff},
		},
		{
			name:   "all three",
			lines:  []string{"//injector:container name=NewFoo returns=foo.Foo must"},
			wantPD: ParsedDirective{Found: true, Name: "NewFoo", ReturnsExpr: "foo.Foo", Must: ir.MustOn},
		},
		{
			name:     "duplicate directive",
			lines:    []string{"//injector:container name=A", "//injector:container name=B"},
			wantPD:   ParsedDirective{Found: true, Name: "A"},
			wantErrs: []string{"duplicate //injector:container directive"},
		},
		{
			name:     "unknown key",
			lines:    []string{"//injector:container xyz=1"},
			wantPD:   ParsedDirective{Found: true},
			wantErrs: []string{`unknown directive key "xyz"`},
		},
		{
			name:     "name without value",
			lines:    []string{"//injector:container name="},
			wantPD:   ParsedDirective{Found: true},
			wantErrs: []string{"directive name= requires a value"},
		},
		{
			name:     "returns without =",
			lines:    []string{"//injector:container returns"},
			wantPD:   ParsedDirective{Found: true},
			wantErrs: []string{"directive returns= requires a value"},
		},
		{
			name:     "must invalid value",
			lines:    []string{"//injector:container must=panic"},
			wantPD:   ParsedDirective{Found: true},
			wantErrs: []string{`directive must= must be true or false, got "panic"`},
		},
		{
			name:     "name twice",
			lines:    []string{"//injector:container name=A name=B"},
			wantPD:   ParsedDirective{Found: true, Name: "A"},
			wantErrs: []string{"directive name= specified more than once"},
		},
		{
			name:   "non-injector tag is ignored",
			lines:  []string{"//go:generate something", "//injector:provider foo", "// injector:container name=X"},
			wantPD: ParsedDirective{Found: true, Name: "X"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotPD, gotErrs := ParseDirective(tt.lines)
			if !reflect.DeepEqual(gotPD, tt.wantPD) {
				t.Errorf("ParsedDirective = %+v, want %+v", gotPD, tt.wantPD)
			}
			if !slices.Equal(gotErrs, tt.wantErrs) {
				t.Errorf("errs = %v, want %v", gotErrs, tt.wantErrs)
			}
		})
	}
}
