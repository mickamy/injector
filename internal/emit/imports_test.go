package emit_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/mickamy/injector/internal/emit"
	"github.com/mickamy/injector/internal/ir"
)

func TestImports_QualifyProvider_SamePackage(t *testing.T) {
	t.Parallel()

	im := emit.New("github.com/me/app")
	p := &ir.Provider{PkgPath: "github.com/me/app", PkgName: "app", FuncName: "NewFoo"}
	if got := im.QualifyProvider(p); got != "NewFoo" {
		t.Errorf("Qualify = %q, want NewFoo", got)
	}
	if entries := im.Sorted(); len(entries) != 0 {
		t.Errorf("expected no imports, got %v", entries)
	}
}

func TestImports_QualifyProvider_DifferentPackage(t *testing.T) {
	t.Parallel()

	im := emit.New("github.com/me/app")
	p := &ir.Provider{PkgPath: "github.com/me/foo", PkgName: "foo", FuncName: "NewBar"}
	im.AddProvider(p)

	if got := im.QualifyProvider(p); got != "foo.NewBar" {
		t.Errorf("Qualify = %q, want foo.NewBar", got)
	}

	es := im.Sorted()
	if len(es) != 1 || es[0].Path != "github.com/me/foo" || es[0].Alias != "foo" {
		t.Errorf("Sorted = %+v", es)
	}
}

func TestImports_AliasCollision(t *testing.T) {
	t.Parallel()

	im := emit.New("github.com/me/app")
	p1 := &ir.Provider{PkgPath: "github.com/x/foo", PkgName: "foo", FuncName: "A"}
	p2 := &ir.Provider{PkgPath: "github.com/y/foo", PkgName: "foo", FuncName: "B"}

	im.AddProvider(p1)
	im.AddProvider(p2)

	if got := im.QualifyProvider(p1); got != "foo.A" {
		t.Errorf("Qualify(p1) = %q, want foo.A", got)
	}
	if got := im.QualifyProvider(p2); got != "foo2.B" {
		t.Errorf("Qualify(p2) = %q, want foo2.B", got)
	}

	es := im.Sorted()
	if len(es) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(es))
	}
	if es[0].Path != "github.com/x/foo" || es[0].Alias != "foo" {
		t.Errorf("entry[0] = %+v", es[0])
	}
	if es[1].Path != "github.com/y/foo" || es[1].Alias != "foo2" {
		t.Errorf("entry[1] = %+v, want foo2", es[1])
	}
}

func TestImports_ReservedAlias(t *testing.T) {
	t.Parallel()

	im := emit.New("github.com/me/app", "log")
	p := &ir.Provider{PkgPath: "github.com/me/log", PkgName: "log", FuncName: "Setup"}
	im.AddProvider(p)
	if got := im.QualifyProvider(p); got != "log2.Setup" {
		t.Errorf("Qualify = %q, want log2.Setup", got)
	}
}

func TestImports_AddTypeRecordsPackage(t *testing.T) {
	t.Parallel()

	// Type-check a small package that imports another package whose type
	// we'll feed into Imports.AddType.
	src := `package myapp

import "io"

type Foo struct {
	R io.Reader
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("myapp", fset, []*ast.File{f}, info)
	if err != nil {
		t.Fatalf("check: %v", err)
	}

	// Locate io.Reader through the field type of Foo.
	fooObj := pkg.Scope().Lookup("Foo")
	if fooObj == nil {
		t.Fatal("Foo not found")
	}
	st, ok := fooObj.Type().Underlying().(*types.Struct)
	if !ok {
		t.Fatalf("Foo is not a struct, got %T", fooObj.Type().Underlying())
	}
	if st.NumFields() == 0 {
		t.Fatal("Foo has no fields")
	}
	readerType := st.Field(0).Type() // io.Reader

	im := emit.New("myapp")
	im.AddType(readerType)

	es := im.Sorted()
	if len(es) != 1 || es[0].Path != "io" {
		t.Errorf("Sorted = %+v, want [io]", es)
	}

	if got := im.QualifyType(readerType); got != "io.Reader" {
		t.Errorf("QualifyType = %q, want io.Reader", got)
	}
}

func TestImports_AddTypeContainerPackageIgnored(t *testing.T) {
	t.Parallel()

	src := `package myapp
type Foo struct{}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("myapp", fset, []*ast.File{f}, info)
	if err != nil {
		t.Fatalf("check: %v", err)
	}

	fooType := pkg.Scope().Lookup("Foo").Type()

	im := emit.New("myapp")
	im.AddType(fooType)

	if entries := im.Sorted(); len(entries) != 0 {
		t.Errorf("expected no imports for container's own package, got %v", entries)
	}
	if got := im.QualifyType(fooType); got != "Foo" {
		t.Errorf("QualifyType = %q, want Foo", got)
	}
}
