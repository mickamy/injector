package scan

import (
	"go/ast"
	"go/types"

	"github.com/mickamy/injector/internal/diag"
	"github.com/mickamy/injector/internal/ir"
	"github.com/mickamy/injector/internal/packages"
)

// Providers scans the given packages for top-level functions that can serve
// as dependency providers and returns the resulting ir.Provider list.
//
// A provider is a top-level (no receiver) function whose result tuple is one
// of:
//   - (T)        where T is a named type, pointer to named type, or interface
//   - (T, error) same shape as above plus a trailing error result
//
// Functions with any other result shape are silently skipped — they may exist
// in the package for non-injection purposes.
func Providers(pkgs []*packages.Package) ([]ir.Provider, []diag.Diag) {
	var (
		providers []ir.Provider
		diags     []diag.Diag
	)
	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		ps, ds := providersInPackage(pkg)
		providers = append(providers, ps...)
		diags = append(diags, ds...)
	}
	return providers, diags
}

// providersInPackage returns the provider list and a slice of diagnostics for
// the given package. The diagnostics slot is currently always empty but kept
// to mirror Containers' signature for symmetry and future use.
//
//nolint:unparam // diagnostics are part of the contract even when unused today
func providersInPackage(pkg *packages.Package) ([]ir.Provider, []diag.Diag) {
	var (
		providers []ir.Provider
		diags     []diag.Diag
	)
	for _, file := range pkg.Syntax {
		if file == nil || isGeneratedFile(file) {
			continue
		}
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd == nil {
				continue
			}
			if fd.Recv != nil {
				// Methods are not providers.
				continue
			}
			if fd.Name == nil || fd.Name.Name == "" {
				continue
			}
			if fd.Type == nil || fd.Type.Results == nil {
				continue
			}
			results := fd.Type.Results.List
			if n := len(results); n != 1 && n != 2 {
				continue
			}

			firstRes := results[0]
			if firstRes == nil || firstRes.Type == nil {
				continue
			}
			if pkg.TypesInfo == nil {
				continue
			}
			resType := pkg.TypesInfo.TypeOf(firstRes.Type)
			if resType == nil {
				continue
			}
			if isBuiltinError(resType) {
				// func Foo() error is not a provider.
				continue
			}
			if !isProviderResultType(resType) {
				continue
			}

			returnsErr := false
			if len(results) == 2 {
				second := results[1]
				if second == nil || second.Type == nil {
					continue
				}
				secondType := pkg.TypesInfo.TypeOf(second.Type)
				if secondType == nil || !isBuiltinError(secondType) {
					// Second result must be exactly error.
					continue
				}
				returnsErr = true
			}

			obj := pkg.TypesInfo.Defs[fd.Name]
			if obj == nil {
				continue
			}
			sig, ok := obj.Type().(*types.Signature)
			if !ok || sig == nil {
				continue
			}

			providers = append(providers, ir.Provider{
				PkgPath:      pkg.PkgPath,
				PkgName:      pkg.Name,
				FuncName:     fd.Name.Name,
				Result:       resType,
				Params:       paramTypes(sig),
				ReturnsError: returnsErr,
				Pos:          positionOf(pkg, fd.Pos()),
			})
		}
	}
	return providers, diags
}

// paramTypes extracts the parameter types of a function signature in
// declaration order, dropping nil entries defensively.
func paramTypes(sig *types.Signature) []types.Type {
	if sig == nil {
		return nil
	}
	tup := sig.Params()
	if tup == nil || tup.Len() == 0 {
		return nil
	}
	out := make([]types.Type, 0, tup.Len())
	for v := range tup.Variables() {
		if v == nil {
			continue
		}
		out = append(out, v.Type())
	}
	return out
}

// isProviderResultType reports whether t is a valid first-result type for a
// provider: a named type, a pointer to a named type, or an interface
// (anonymous or named). Type aliases are resolved to their target before the
// check so that providers returning an alias are still recognized.
func isProviderResultType(t types.Type) bool {
	t = types.Unalias(t)
	if isNamedOrPtrToNamed(t) {
		return true
	}
	return isInterfaceType(t)
}

func isInterfaceType(t types.Type) bool {
	switch tt := t.(type) {
	case *types.Interface:
		return true
	case *types.Named:
		_, ok := tt.Underlying().(*types.Interface)
		return ok
	default:
		return false
	}
}

func isNamedOrPtrToNamed(t types.Type) bool {
	switch tt := t.(type) {
	case *types.Named:
		return true
	case *types.Pointer:
		_, ok := tt.Elem().(*types.Named)
		return ok
	default:
		return false
	}
}

func isBuiltinError(t types.Type) bool {
	obj := types.Universe.Lookup("error")
	if obj == nil {
		return false
	}
	return types.Identical(t, obj.Type())
}
