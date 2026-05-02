package plan

import (
	"go/types"

	"github.com/mickamy/injector/internal/ir"
)

// Index is an indexed view over a set of providers, supporting lookup by
// fully qualified or short name and by result type.
//
// Pointers returned by lookup methods refer to entries owned by the Index;
// callers must not mutate them.
type Index struct {
	all     []*ir.Provider
	byFull  map[string]*ir.Provider   // pkgPath.FuncName
	byShort map[string][]*ir.Provider // pkgName.FuncName + bare FuncName
	byType  map[string][]*ir.Provider // TypeKey(Result)
}

// NewIndex builds an Index from the given providers. Each entry is copied
// into the index so that pointer identity is preserved across lookups.
func NewIndex(providers []ir.Provider) *Index {
	idx := &Index{
		byFull:  make(map[string]*ir.Provider, len(providers)),
		byShort: make(map[string][]*ir.Provider),
		byType:  make(map[string][]*ir.Provider),
	}
	for i := range providers {
		p := &providers[i]
		idx.all = append(idx.all, p)

		full := p.PkgPath + "." + p.FuncName
		idx.byFull[full] = p

		if p.PkgName != "" {
			short := p.PkgName + "." + p.FuncName
			idx.byShort[short] = append(idx.byShort[short], p)
		}
		idx.byShort[p.FuncName] = append(idx.byShort[p.FuncName], p)

		if p.Result != nil {
			tk := TypeKey(p.Result)
			idx.byType[tk] = append(idx.byType[tk], p)
		}
	}
	return idx
}

// All returns every provider in the order originally passed to NewIndex.
func (idx *Index) All() []*ir.Provider {
	return idx.all
}

// LookupByRef resolves a textual provider reference written by the user in
// inject:"with=<ref>". The reference may be a fully qualified path/name
// (e.g. "github.com/me/x.NewFoo"), a short package.name pair (e.g.
// "x.NewFoo"), or a bare function name (e.g. "NewFoo").
//
// Returns all matching providers. Callers treat zero results as "not found"
// and multiple results as "ambiguous" and produce candidate diagnostics
// using FormatCandidates.
func (idx *Index) LookupByRef(ref string) []*ir.Provider {
	if ref == "" {
		return nil
	}
	if p, ok := idx.byFull[ref]; ok {
		return []*ir.Provider{p}
	}
	return idx.byShort[ref]
}

// LookupByType returns all providers whose result type is identical to t.
// Equality is decided by TypeKey, which qualifies named types by their
// package path.
func (idx *Index) LookupByType(t types.Type) []*ir.Provider {
	if t == nil {
		return nil
	}
	return idx.byType[TypeKey(t)]
}

// TypeKey returns a stable string key for a type that distinguishes named
// types by their fully qualified package path. Suitable for map keys.
func TypeKey(t types.Type) string {
	return types.TypeString(t, qualifyByPath)
}

// TypeString renders a type with package paths included, for use in
// diagnostics where an unambiguous type rendering is preferred.
func TypeString(t types.Type) string {
	return types.TypeString(t, qualifyByPath)
}

// ProviderName returns "<pkgPath>.<FuncName>" when a package path is
// available, falling back to just the function name.
func ProviderName(p *ir.Provider) string {
	if p == nil {
		return "<nil>"
	}
	if p.PkgPath == "" {
		return p.FuncName
	}
	return p.PkgPath + "." + p.FuncName
}

// FormatCandidates renders a list of providers as bullet lines suitable for
// use as Diag.Hints when reporting ambiguity.
func FormatCandidates(ps []*ir.Provider) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		line := "- " + ProviderName(p)
		if p.Pos.IsValid() {
			line += " (" + p.Pos.String() + ")"
		}
		out = append(out, line)
	}
	return out
}

func qualifyByPath(p *types.Package) string {
	if p == nil {
		return ""
	}
	return p.Path()
}
