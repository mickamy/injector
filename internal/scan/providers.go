package scan

import (
	"errors"
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// ProviderSpec represents a discovered provider function.
type ProviderSpec struct {
	PkgPath      string
	PkgName      string
	Name         string
	ResultType   types.Type
	ResultString string
	Params       []types.Type
	Position     string
}

// CollectProviders scans loaded packages and collects provider functions.
//
// Rule:
// - Top-level functions only (func Foo(...))
// - Exactly 1 result
// - Result type can be any named type, pointer to named type, or interface type
// - Parameters are recorded as dependency requirements
func CollectProviders(pkgs []*packages.Package) ([]ProviderSpec, error) {
	if len(pkgs) == 0 {
		return nil, errors.New("scan: no packages")
	}

	var out []ProviderSpec
	var errs []string

	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		decls, err := collectProvidersInPackage(pkg)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		out = append(out, decls...)
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("scan: %s", joinLines(errs))
	}
	return out, nil
}

func collectProvidersInPackage(pkg *packages.Package) ([]ProviderSpec, error) {
	var out []ProviderSpec
	var errs []string

	for _, file := range pkg.Syntax {
		if file == nil {
			continue
		}

		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd == nil {
				continue
			}
			if fd.Recv != nil {
				// Methods are not providers in MVP.
				continue
			}
			if fd.Name == nil || fd.Name.Name == "" {
				continue
			}
			if fd.Type == nil || fd.Type.Results == nil {
				continue
			}
			if len(fd.Type.Results.List) != 1 {
				// Require exactly 1 result.
				continue
			}

			res := fd.Type.Results.List[0]
			if res == nil || res.Type == nil {
				continue
			}

			if pkg.TypesInfo == nil {
				continue
			}

			resType := pkg.TypesInfo.TypeOf(res.Type)
			if resType == nil {
				continue
			}

			if isBuiltinError(resType) {
				// func Foo() error is not a provider.
				continue
			}

			if !isProviderResultType(resType) {
				// Skip unsupported result shapes.
				continue
			}

			sig, _ := pkg.TypesInfo.Defs[fd.Name].Type().(*types.Signature)
			if sig == nil {
				// Fallback: try to obtain signature from types info on FuncDecl.
				if obj, ok := pkg.TypesInfo.Defs[fd.Name]; ok && obj != nil {
					sig, _ = obj.Type().(*types.Signature)
				}
			}
			if sig == nil {
				continue
			}

			params := extractParamTypes(sig)

			out = append(out, ProviderSpec{
				PkgPath:    pkg.PkgPath,
				PkgName:    pkg.Name,
				Name:       fd.Name.Name,
				ResultType: resType,
				ResultString: types.TypeString(resType, func(p *types.Package) string {
					if p == nil {
						return ""
					}
					return p.Name()
				}),
				Params:   params,
				Position: position(pkg.Fset, fd.Pos()),
			})
		}
	}

	if len(errs) > 0 {
		return nil, errors.New(joinLines(errs))
	}
	return out, nil
}

func extractParamTypes(sig *types.Signature) []types.Type {
	if sig == nil {
		return nil
	}
	tup := sig.Params()
	if tup == nil || tup.Len() == 0 {
		return nil
	}

	out := make([]types.Type, 0, tup.Len())
	for i := 0; i < tup.Len(); i++ {
		v := tup.At(i)
		if v == nil {
			continue
		}
		out = append(out, v.Type())
	}
	return out
}

func isProviderResultType(t types.Type) bool {
	// Allowed:
	// - named types: T
	// - pointers to named types: *T
	// - interface types (including named interfaces): interface{...} or type I interface{...}
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
	errType := obj.Type()
	return types.Identical(t, errType)
}
