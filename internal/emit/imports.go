package emit

import (
	"go/types"
	"path"
	"sort"
	"strconv"

	"github.com/mickamy/injector/internal/ir"
)

// Imports tracks the packages an emitted file references and assigns each a
// non-conflicting alias. The container's own package is never included.
type Imports struct {
	containerPkg string
	aliases      map[string]string // pkgPath → alias
	used         map[string]struct{}
}

// ImportEntry is a single line in the generated import block.
type ImportEntry struct {
	Path  string
	Alias string
}

// New constructs an Imports tracker for an emitted file in containerPkg.
// Reserved aliases are pre-claimed so the generated source can use them
// without collision (e.g. "log" when must mode emits log.Fatal).
func New(containerPkg string, reserved ...string) *Imports {
	im := &Imports{
		containerPkg: containerPkg,
		aliases:      map[string]string{},
		used:         map[string]struct{}{},
	}
	for _, r := range reserved {
		im.used[r] = struct{}{}
	}
	return im
}

// AddType records the package of every named type referenced by t,
// recursing into pointers, slices, arrays, maps, channels, signatures, and
// generic type arguments.
func (im *Imports) AddType(t types.Type) {
	if t == nil {
		return
	}
	for _, p := range collectPkgs(t) {
		im.addPackage(p)
	}
}

// AddProvider records the provider's declaring package.
func (im *Imports) AddProvider(p *ir.Provider) {
	if p == nil {
		return
	}
	im.addByPath(p.PkgPath, p.PkgName)
}

// QualifyType renders t as it should appear in source code, prefixing types
// from imported packages with their assigned alias and leaving same-package
// types unqualified.
func (im *Imports) QualifyType(t types.Type) string {
	return types.TypeString(t, im.qualify)
}

// QualifyProvider returns the call expression for p (e.g. "foo.NewBar"),
// or just the function name when p lives in the container's package.
func (im *Imports) QualifyProvider(p *ir.Provider) string {
	if p == nil {
		return ""
	}
	if p.PkgPath == "" || p.PkgPath == im.containerPkg {
		return p.FuncName
	}
	alias := im.aliases[p.PkgPath]
	if alias == "" {
		alias = path.Base(p.PkgPath)
	}
	return alias + "." + p.FuncName
}

// Sorted returns one entry per imported package, sorted by path. The
// container's own package is excluded.
func (im *Imports) Sorted() []ImportEntry {
	out := make([]ImportEntry, 0, len(im.aliases))
	for p, a := range im.aliases {
		out = append(out, ImportEntry{Path: p, Alias: a})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func (im *Imports) addPackage(p *types.Package) {
	if p == nil {
		return
	}
	im.addByPath(p.Path(), p.Name())
}

func (im *Imports) addByPath(pkgPath, baseName string) {
	if pkgPath == "" || pkgPath == im.containerPkg {
		return
	}
	if _, ok := im.aliases[pkgPath]; ok {
		return
	}

	base := baseName
	if base == "" {
		base = path.Base(pkgPath)
	}
	if base == "" || base == "." || base == "/" {
		base = "pkg"
	}

	alias := base
	if _, taken := im.used[alias]; taken {
		for i := 2; ; i++ {
			try := base + strconv.Itoa(i)
			if _, taken := im.used[try]; !taken {
				alias = try
				break
			}
		}
	}
	im.aliases[pkgPath] = alias
	im.used[alias] = struct{}{}
}

// qualify is the package-qualification function passed to types.TypeString.
func (im *Imports) qualify(p *types.Package) string {
	if p == nil {
		return ""
	}
	if p.Path() == im.containerPkg {
		return ""
	}
	if alias, ok := im.aliases[p.Path()]; ok {
		return alias
	}
	return p.Name()
}

// collectPkgs walks t and returns each named-type package it references.
//
// The walk descends through the type shapes that may appear in the generated
// source: pointers, slices/arrays/maps/channels, signature parameters and
// results, and named types' type arguments. It deliberately does NOT descend
// into a *types.Named's Underlying form, nor into *types.Interface methods
// or *types.Struct fields — those internals are not written verbatim into
// the generated file (only the named type itself is qualified), so adding
// their packages would cause "imported and not used" errors. If we ever
// inline anonymous interface/struct shapes into the output, those cases
// would need to be added here.
func collectPkgs(t types.Type) []*types.Package {
	var out []*types.Package
	seen := map[*types.Package]struct{}{}

	var walk func(types.Type)
	walk = func(t types.Type) {
		if t == nil {
			return
		}
		switch tt := t.(type) {
		case *types.Named:
			if obj := tt.Obj(); obj != nil {
				if pkg := obj.Pkg(); pkg != nil {
					if _, ok := seen[pkg]; !ok {
						seen[pkg] = struct{}{}
						out = append(out, pkg)
					}
				}
			}
			if ta := tt.TypeArgs(); ta != nil {
				for arg := range ta.Types() {
					walk(arg)
				}
			}
		case *types.Pointer:
			walk(tt.Elem())
		case *types.Slice:
			walk(tt.Elem())
		case *types.Array:
			walk(tt.Elem())
		case *types.Map:
			walk(tt.Key())
			walk(tt.Elem())
		case *types.Chan:
			walk(tt.Elem())
		case *types.Signature:
			if params := tt.Params(); params != nil {
				for v := range params.Variables() {
					walk(v.Type())
				}
			}
			if results := tt.Results(); results != nil {
				for v := range results.Variables() {
					walk(v.Type())
				}
			}
		}
	}
	walk(t)
	return out
}
