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

	"github.com/mickamy/injector/internal/diag"
	"github.com/mickamy/injector/internal/ir"
	"github.com/mickamy/injector/internal/packages"
)

// Containers scans the given packages and returns containers detected by the
// presence of inject:"..." struct tags on struct fields. Generated files
// (those bearing the canonical "Code generated ... DO NOT EDIT." marker) are
// excluded.
func Containers(pkgs []*packages.Package) ([]ir.Container, []diag.Diag) {
	var (
		containers []ir.Container
		diags      []diag.Diag
	)

	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		cs, ds := containersInPackage(pkg)
		containers = append(containers, cs...)
		diags = append(diags, ds...)
	}
	return containers, diags
}

func containersInPackage(pkg *packages.Package) ([]ir.Container, []diag.Diag) {
	var (
		containers []ir.Container
		diags      []diag.Diag
	)

	for _, file := range pkg.Syntax {
		if file == nil || isGeneratedFile(file) {
			continue
		}

		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name == nil {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}

				fields, fdiags := parseFields(pkg, st.Fields)
				diags = append(diags, fdiags...)
				if len(fields) == 0 {
					continue
				}

				pos := positionOf(pkg, ts.Pos())

				pd, errs := parseStructDirective(gd, ts)
				for _, msg := range errs {
					diags = append(diags, diag.Errorf(pos, "%s", msg))
				}

				directive, drDiags := buildDirective(pd, pkg, pos)
				diags = append(diags, drDiags...)

				var structType types.Type
				if pkg.TypesInfo != nil {
					if obj := pkg.TypesInfo.Defs[ts.Name]; obj != nil {
						structType = obj.Type()
					}
				}

				containers = append(containers, ir.Container{
					PkgPath:    pkg.PkgPath,
					PkgName:    pkg.Name,
					StructName: ts.Name.Name,
					StructType: structType,
					Pos:        pos,
					Directive:  directive,
					Fields:     fields,
				})
			}
		}
	}
	return containers, diags
}

// parseFields walks fl and returns ir.Field entries for each field that
// carries an inject:"..." tag. Untagged fields are silently ignored.
func parseFields(pkg *packages.Package, fl *ast.FieldList) ([]ir.Field, []diag.Diag) {
	var (
		fields []ir.Field
		diags  []diag.Diag
	)
	if fl == nil {
		return nil, nil
	}

	for _, f := range fl.List {
		if f == nil {
			continue
		}

		_, injectVal, hasInject := extractInjectTag(f.Tag)
		if !hasInject {
			continue
		}

		fpos := positionOf(pkg, f.Pos())

		parsed, err := ParseTag(injectVal)
		if err != nil {
			diags = append(diags, diag.Errorf(fpos, "%s", err))
			continue
		}

		if f.Type == nil {
			diags = append(diags, diag.Errorf(fpos, "field has no type expression"))
			continue
		}

		var ftype types.Type
		if pkg.TypesInfo != nil {
			ftype = pkg.TypesInfo.TypeOf(f.Type)
		}
		if ftype == nil {
			diags = append(diags, diag.Errorf(fpos, "could not resolve type for field %s", types.ExprString(f.Type)))
			continue
		}

		if len(f.Names) == 0 {
			diags = append(diags, diag.Errorf(fpos, "embedded field with inject tag is not supported"))
			continue
		}

		for _, name := range f.Names {
			if name == nil || name.Name == "" {
				continue
			}
			role, d := decideRole(name.Name, parsed, fpos)
			if d != nil {
				diags = append(diags, *d)
				continue
			}
			fields = append(fields, ir.Field{
				Name:        name.Name,
				Type:        ftype,
				Role:        role,
				ProviderRef: ir.ProviderRef{Raw: parsed.With},
				ArgName:     parsed.ArgName,
				IsReturns:   parsed.Kind == TagReturns,
				Pos:         fpos,
			})
		}
	}
	return fields, diags
}

// decideRole maps a (field name, parsed tag) pair onto an ir.Role, returning
// a diagnostic when the combination is illegal in the v0.2 DSL.
func decideRole(fieldName string, pt ParsedTag, pos token.Position) (ir.Role, *diag.Diag) {
	blank := fieldName == "_"

	switch pt.Kind {
	case TagMarker:
		if blank {
			d := diag.Errorf(pos, `_ field requires inject:"with=...", inject:"arg" or inject:"returns"`)
			return 0, &d
		}
		return ir.RoleOut, nil

	case TagWith:
		if blank {
			return ir.RoleOverride, nil
		}
		return ir.RoleOut, nil

	case TagArg:
		if !blank {
			d := diag.Errorf(pos, `inject:"arg" requires a blank field (_)`)
			return 0, &d
		}
		return ir.RoleArg, nil

	case TagReturns:
		if blank {
			return ir.RoleReturnsOnly, nil
		}
		return ir.RoleOut, nil

	case TagInvalid:
		fallthrough
	default:
		d := diag.Errorf(pos, "internal: unrecognized inject tag form")
		return 0, &d
	}
}

// extractInjectTag extracts the inject value from a struct tag literal.
func extractInjectTag(tag *ast.BasicLit) (rawTag string, injectVal string, hasInject bool) {
	if tag == nil || tag.Kind != token.STRING {
		return "", "", false
	}
	s, err := strconv.Unquote(tag.Value)
	if err != nil {
		return tag.Value, "", false
	}
	if val, ok := reflect.StructTag(s).Lookup("inject"); ok {
		return s, val, true
	}
	return s, "", false
}

// parseStructDirective walks the doc comment attached to ts (or its parent
// gd in grouped declarations) and returns the parsed directive.
func parseStructDirective(gd *ast.GenDecl, ts *ast.TypeSpec) (ParsedDirective, []string) {
	cg := ts.Doc
	if cg == nil {
		cg = gd.Doc
	}
	if cg == nil {
		return ParsedDirective{}, nil
	}
	lines := make([]string, 0, len(cg.List))
	for _, c := range cg.List {
		if c == nil {
			continue
		}
		lines = append(lines, c.Text)
	}
	return ParseDirective(lines)
}

// buildDirective converts a ParsedDirective into ir.Directive, resolving the
// returns expression to a concrete go/types Type when present.
func buildDirective(pd ParsedDirective, pkg *packages.Package, pos token.Position) (ir.Directive, []diag.Diag) {
	d := ir.Directive{
		Name: pd.Name,
		Must: pd.Must,
	}
	if pd.ReturnsExpr == "" {
		return d, nil
	}
	t, err := resolveTypeExpr(pkg, pd.ReturnsExpr)
	if err != nil {
		return d, []diag.Diag{
			diag.Errorf(pos, "directive returns=%s: %v", pd.ReturnsExpr, err),
		}
	}
	d.ReturnType = t
	return d, nil
}

// resolveTypeExpr evaluates a textual type expression in the package scope.
// The expression may reference any name that is in scope from the package
// (including imported packages), so callers are responsible for ensuring the
// directive uses references the container package can see.
func resolveTypeExpr(pkg *packages.Package, expr string) (types.Type, error) {
	if pkg.Types == nil || pkg.Fset == nil {
		return nil, errors.New("package types or fset unavailable")
	}
	tv, err := types.Eval(pkg.Fset, pkg.Types, token.NoPos, expr)
	if err != nil {
		return nil, fmt.Errorf("types.Eval: %w", err)
	}
	if !tv.IsType() {
		return nil, errors.New("expression is not a type")
	}
	return tv.Type, nil
}

// positionOf returns the token.Position for pos using the package's FileSet.
func positionOf(pkg *packages.Package, pos token.Pos) token.Position {
	if pkg.Fset == nil {
		return token.Position{}
	}
	return pkg.Fset.Position(pos)
}

// isGeneratedFile reports whether file has the canonical "Code generated ...
// DO NOT EDIT." comment as one of its leading comment lines (per
// https://golang.org/s/generatedcode). The marker must appear before the
// package clause.
func isGeneratedFile(file *ast.File) bool {
	if len(file.Comments) == 0 {
		return false
	}
	first := file.Comments[0]
	if first == nil || first.End() > file.Package {
		return false
	}
	for _, c := range first.List {
		if c == nil {
			continue
		}
		line := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if strings.HasPrefix(line, "Code generated") && strings.HasSuffix(line, "DO NOT EDIT.") {
			return true
		}
	}
	return false
}
