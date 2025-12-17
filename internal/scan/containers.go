package scan

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ContainerSpec represents a discovered container struct.
type ContainerSpec struct {
	PkgPath  string
	PkgName  string
	Name     string
	Position string
	Fields   []ContainerField
}

// ContainerField represents a field within a container struct.
type ContainerField struct {
	Name     string
	TypeExpr string
	Type     types.Type

	TagRaw    string
	InjectRaw string
	Inject    InjectTag

	Position string
}

// CollectContainers scans loaded packages and collects container structs.
func CollectContainers(pkgs []*packages.Package) ([]ContainerSpec, error) {
	if len(pkgs) == 0 {
		return nil, errors.New("scan: no packages")
	}

	var out []ContainerSpec
	var errs []string

	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		decls, err := collectContainersInPackage(pkg)
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

func collectContainersInPackage(pkg *packages.Package) ([]ContainerSpec, error) {
	var out []ContainerSpec
	var errs []string

	for _, file := range pkg.Syntax {
		if file == nil {
			continue
		}

		for node := range ast.Preorder(file) {
			ts, ok := node.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if ts.Name == nil {
				return nil, fmt.Errorf("node has no name: %s", ts.Name)
			}

			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}

			fields, ferrs := collectContainerFields(pkg, st.Fields)
			if len(ferrs) > 0 {
				errs = append(errs, ferrs...)
				continue
			}

			if len(fields) == 0 {
				continue
			}

			spec := ContainerSpec{
				PkgPath:  pkg.PkgPath,
				PkgName:  pkg.Name,
				Name:     ts.Name.Name,
				Position: position(pkg.Fset, ts.Pos()),
				Fields:   fields,
			}

			out = append(out, spec)
		}
	}

	if len(errs) > 0 {
		return nil, errors.New(joinLines(errs))
	}
	return out, nil
}

func collectContainerFields(pkg *packages.Package, fl *ast.FieldList) ([]ContainerField, []string) {
	if fl == nil || len(fl.List) == 0 {
		return nil, nil
	}

	var out []ContainerField
	var errs []string

	for _, f := range fl.List {
		if f == nil || f.Type == nil {
			continue
		}

		typeExpr := types.ExprString(f.Type)
		typ := types.Type(nil)
		if pkg.TypesInfo != nil {
			typ = pkg.TypesInfo.TypeOf(f.Type)
		}

		tagRaw, injectRaw := parseStructTag(f.Tag)

		var parsed InjectTag
		if hasInjectKey(tagRaw) {
			t, err := parseInjectorTag(injectRaw)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: invalid inject tag: %v", position(pkg.Fset, f.Pos()), err))
			} else {
				parsed = t
			}
		} else {
			continue
		}

		// Anonymous fields are allowed; we treat the name as the type expression.
		if len(f.Names) == 0 {
			out = append(out, ContainerField{
				Name:      typeExpr,
				TypeExpr:  typeExpr,
				Type:      typ,
				TagRaw:    tagRaw,
				InjectRaw: injectRaw,
				Inject:    parsed,
				Position:  position(pkg.Fset, f.Pos()),
			})
			continue
		}

		for _, name := range f.Names {
			if name == nil || name.Name == "" {
				continue
			}
			out = append(out, ContainerField{
				Name:      name.Name,
				TypeExpr:  typeExpr,
				Type:      typ,
				TagRaw:    tagRaw,
				InjectRaw: injectRaw,
				Inject:    parsed,
				Position:  position(pkg.Fset, f.Pos()),
			})
		}
	}

	return out, errs
}

func parseStructTag(tag *ast.BasicLit) (raw string, inject string) {
	if tag == nil || tag.Kind != token.STRING {
		return "", ""
	}

	// Normalize struct tag literal to its actual content.
	// This supports both:
	// - raw string literal: `inject:"provider:Foo"`
	// - interpreted string: "inject:\"provider:Foo\""
	s, err := strconv.Unquote(tag.Value)
	if err != nil {
		// Fallback: keep previous behavior if unquote fails.
		s = tag.Value
		s = strings.TrimPrefix(s, "`")
		s = strings.TrimSuffix(s, "`")
		s = strings.TrimPrefix(s, `"`)
		s = strings.TrimSuffix(s, `"`)
	}

	raw = s

	// Primary: use reflect.StructTag when possible.
	if v, ok := reflect.StructTag(s).Lookup("inject"); ok {
		return raw, v
	}

	// Fallback: extract inject:"..." manually.
	if v, ok := extractTagValue(raw, "inject"); ok {
		return raw, v
	}

	return raw, ""
}

// extractTagValue extracts key:"value" from a raw struct tag string.
// It is intentionally minimal for MVP (does not handle escaped quotes inside the value).
func extractTagValue(tagRaw, key string) (string, bool) {
	prefix := key + `:"`
	i := strings.Index(tagRaw, prefix)
	if i < 0 {
		return "", false
	}
	start := i + len(prefix)
	end := strings.IndexByte(tagRaw[start:], '"')
	if end < 0 {
		return "", false
	}
	return tagRaw[start : start+end], true
}

// hasInjectKey reports whether the struct tag contains the `inject` key,
// including both marker-only (`inject:""`) and value forms (`inject:"..."`).
func hasInjectKey(tagRaw string) bool {
	if tagRaw == "" {
		return false
	}

	// Fast path: reflect parser detects key presence only if it is in key:"value" form.
	// For marker-only tags, reflect.StructTag won't help, so we also scan tokens.
	if _, ok := reflect.StructTag(tagRaw).Lookup("inject"); ok {
		return true
	}

	// Marker-only form: `inject:""` (possibly among other tags).
	// Also accept weird-but-valid spacing: `json:"x"  inject`
	parts := strings.Fields(tagRaw)
	for _, p := range parts {
		if p == "inject" {
			return true
		}
	}

	// Value form is already handled by Lookup, but keep a conservative fallback.
	return strings.Contains(tagRaw, `inject:"`)
}
