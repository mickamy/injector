package plan_test

import (
	"testing"

	"github.com/mickamy/injector/internal/ir"
	"github.com/mickamy/injector/internal/plan"
)

func TestIndex_LookupByRef(t *testing.T) {
	t.Parallel()

	a := ir.Provider{PkgPath: "github.com/example/foo", PkgName: "foo", FuncName: "NewBar"}
	b := ir.Provider{PkgPath: "github.com/example/foo2", PkgName: "foo", FuncName: "NewBar"}
	c := ir.Provider{PkgPath: "github.com/example/baz", PkgName: "baz", FuncName: "NewBaz"}

	idx := plan.NewIndex([]ir.Provider{a, b, c})

	tests := []struct {
		name string
		ref  string
		want int
	}{
		{name: "fully qualified", ref: "github.com/example/foo.NewBar", want: 1},
		{name: "package short ambiguous", ref: "foo.NewBar", want: 2},
		{name: "bare unique", ref: "NewBaz", want: 1},
		{name: "bare ambiguous", ref: "NewBar", want: 2},
		{name: "not found", ref: "NoSuch", want: 0},
		{name: "empty ref", ref: "", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := idx.LookupByRef(tt.ref)
			if len(got) != tt.want {
				t.Errorf("LookupByRef(%q) returned %d, want %d", tt.ref, len(got), tt.want)
			}
		})
	}
}

func TestProviderName(t *testing.T) {
	t.Parallel()

	if got := plan.ProviderName(nil); got != "<nil>" {
		t.Errorf("ProviderName(nil) = %q, want <nil>", got)
	}

	p := ir.Provider{PkgPath: "github.com/x/y", FuncName: "F"}
	if got, want := plan.ProviderName(&p), "github.com/x/y.F"; got != want {
		t.Errorf("ProviderName = %q, want %q", got, want)
	}

	p2 := ir.Provider{FuncName: "F"}
	if got, want := plan.ProviderName(&p2), "F"; got != want {
		t.Errorf("ProviderName(no pkg) = %q, want %q", got, want)
	}
}

func TestFormatCandidates(t *testing.T) {
	t.Parallel()

	a := ir.Provider{PkgPath: "x", FuncName: "A"}
	b := ir.Provider{PkgPath: "y", FuncName: "B"}

	got := plan.FormatCandidates([]*ir.Provider{&a, &b})
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2", len(got))
	}
	if got[0] != "- x.A" {
		t.Errorf("line[0] = %q, want %q", got[0], "- x.A")
	}
	if got[1] != "- y.B" {
		t.Errorf("line[1] = %q, want %q", got[1], "- y.B")
	}
}

func TestIndex_All_PreservesOrder(t *testing.T) {
	t.Parallel()

	in := []ir.Provider{
		{PkgPath: "a", FuncName: "X"},
		{PkgPath: "b", FuncName: "Y"},
		{PkgPath: "c", FuncName: "Z"},
	}
	idx := plan.NewIndex(in)

	got := idx.All()
	if len(got) != len(in) {
		t.Fatalf("len = %d, want %d", len(got), len(in))
	}
	for i := range in {
		if got[i].PkgPath != in[i].PkgPath || got[i].FuncName != in[i].FuncName {
			t.Errorf("got[%d] = %+v, want %+v", i, got[i], in[i])
		}
	}
}
