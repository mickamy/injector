package plan_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/mickamy/injector/internal/diag"
	"github.com/mickamy/injector/internal/ir"
	"github.com/mickamy/injector/internal/packages"
	"github.com/mickamy/injector/internal/plan"
	"github.com/mickamy/injector/internal/scan"
)

func TestBuild_SimpleChain(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
type User struct{}
func NewDB() *DB { return nil }
func NewUser(db *DB) *User { return nil }
type Container struct {
	User *User ` + "`inject:\"\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})

	if got, want := p.ConstructorName, "NewContainer"; got != want {
		t.Errorf("ConstructorName = %q, want %q", got, want)
	}
	if len(p.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(p.Steps))
	}
	if p.Steps[0].Provider == nil || p.Steps[0].Provider.FuncName != "NewDB" {
		t.Errorf("step[0] = %+v, want NewDB", p.Steps[0])
	}
	if p.Steps[1].Provider == nil || p.Steps[1].Provider.FuncName != "NewUser" {
		t.Errorf("step[1] = %+v, want NewUser", p.Steps[1])
	}
	if len(p.Outputs) != 1 || p.Outputs[0].FieldName != "User" {
		t.Errorf("outputs = %+v", p.Outputs)
	}
	if p.ReturnsError {
		t.Errorf("ReturnsError = true, want false")
	}
}

func TestBuild_WithErrorPropagates(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
func NewDB() (*DB, error) { return nil, nil }
type Container struct {
	DB *DB ` + "`inject:\"\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})
	if !p.ReturnsError {
		t.Errorf("ReturnsError = false, want true")
	}
}

func TestBuild_ArgInput(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
type User struct{}
func NewUser(db *DB) *User { return nil }
type Container struct {
	_    *DB   ` + "`inject:\"arg\"`" + `
	User *User ` + "`inject:\"\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})

	if len(p.Inputs) != 1 || p.Inputs[0].Name != "db" {
		t.Errorf("inputs = %+v", p.Inputs)
	}

	if p.Steps[0].Kind != plan.StepKindInput {
		t.Errorf("step[0].Kind = %v, want StepKindInput", p.Steps[0].Kind)
	}
}

func TestBuild_ArgWithCustomName(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
func NewUserish(db *DB) *DB { return nil }
type Container struct {
	_  *DB ` + "`inject:\"arg=primary\"`" + `
	DB *DB ` + "`inject:\"with=NewUserish\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})

	if len(p.Inputs) != 1 || p.Inputs[0].Name != "primary" {
		t.Errorf("inputs = %+v, want name=primary", p.Inputs)
	}
}

func TestBuild_OverrideAmbiguous(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
type User struct{}
func NewWriter() *DB { return nil }
func NewReader() *DB { return nil }
func NewUser(db *DB) *User { return nil }
type Container struct {
	_    *DB   ` + "`inject:\"with=NewWriter\"`" + `
	User *User ` + "`inject:\"\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})

	var sawWriter bool
	for _, s := range p.Steps {
		if s.Provider == nil {
			continue
		}
		switch s.Provider.FuncName {
		case "NewWriter":
			sawWriter = true
		case "NewReader":
			t.Errorf("NewReader should not appear in the plan when override is NewWriter")
		case "NewUser":
			if !sawWriter {
				t.Errorf("NewWriter must precede NewUser, got steps: %+v", p.Steps)
			}
		}
	}
	if !sawWriter {
		t.Errorf("expected NewWriter step, got %+v", p.Steps)
	}
}

func TestBuild_VarNameForBareNewFactory(t *testing.T) {
	t.Parallel()

	// A package-level factory called bare-"New" used to produce a
	// variable literally named "new", which both reads like the Go
	// built-in and tells the reader nothing about the value. The variable
	// should now be named after the result type instead.
	src := `package test
type Transactor struct{}
func New() Transactor { return Transactor{} }
type Container struct {
	Tx Transactor ` + "`inject:\"\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})

	if len(p.Steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(p.Steps))
	}
	if got, want := p.Steps[0].VarName, "transactor"; got != want {
		t.Errorf("var name = %q, want %q", got, want)
	}
}

func TestBuild_VarNameForNewSuffixedFactoryUnchanged(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
func NewDB() *DB { return nil }
type Container struct {
	DB *DB ` + "`inject:\"\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})
	if got, want := p.Steps[0].VarName, "db"; got != want {
		t.Errorf("var name = %q, want %q (NewDB should still strip to DB)", got, want)
	}
}

func TestBuild_NonBlankWithActsAsOverride(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
type User struct{}
func NewWriter() *DB { return nil }
func NewReader() *DB { return nil }
func NewUser(db *DB) *User { return nil }
type Container struct {
	DB   *DB   ` + "`inject:\"with=NewWriter\"`" + `
	User *User ` + "`inject:\"\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})

	for _, s := range p.Steps {
		if s.Provider != nil && s.Provider.FuncName == "NewReader" {
			t.Errorf("NewReader must not appear when DB is pinned to NewWriter; steps=%+v", p.Steps)
		}
	}

	var hasDBOutput, hasUserOutput bool
	for _, o := range p.Outputs {
		switch o.FieldName {
		case "DB":
			hasDBOutput = true
		case "User":
			hasUserOutput = true
		}
	}
	if !hasDBOutput || !hasUserOutput {
		t.Errorf("expected both DB and User outputs, got %+v", p.Outputs)
	}
}

func TestBuild_NonBlankWithConflictingProviders(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
func NewWriter() *DB { return nil }
func NewReader() *DB { return nil }
type Container struct {
	A *DB ` + "`inject:\"with=NewWriter\"`" + `
	B *DB ` + "`inject:\"with=NewReader\"`" + `
}
`
	_, ds := build(t, src, "Container", plan.Options{})
	if !diag.HasErrors(ds) {
		t.Fatalf("expected conflicting-providers diag, got %v", ds)
	}
}

func TestBuild_AmbiguousProviderError(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
func NewWriter() *DB { return nil }
func NewReader() *DB { return nil }
type Container struct {
	DB *DB ` + "`inject:\"\"`" + `
}
`
	p, ds := build(t, src, "Container", plan.Options{})
	if !diag.HasErrors(ds) {
		t.Fatalf("expected ambiguity error, got plan=%+v diags=%v", p, ds)
	}
}

func TestBuild_NoProviderError(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
type Container struct {
	DB *DB ` + "`inject:\"\"`" + `
}
`
	p, ds := build(t, src, "Container", plan.Options{})
	if !diag.HasErrors(ds) {
		t.Fatalf("expected missing provider error, got plan=%+v diags=%v", p, ds)
	}
}

func TestBuild_DirectiveReturnTypeOverridesPointer(t *testing.T) {
	t.Parallel()

	src := `package test
type Greeter interface{ Greet() string }
type greeterImpl struct{}
func (greeterImpl) Greet() string { return "" }
func NewImpl() *greeterImpl { return nil }

//injector:container returns=Greeter
type app struct {
	G *greeterImpl ` + "`inject:\"\"`" + `
}
`
	p, _ := mustBuild(t, src, "app", plan.Options{})
	if p.ReturnType == nil {
		t.Fatal("ReturnType is nil")
	}
	if got := plan.TypeString(p.ReturnType); got != "test.Greeter" {
		t.Errorf("ReturnType = %s, want test.Greeter", got)
	}
}

func TestBuild_TagReturnsSetsReturnType(t *testing.T) {
	t.Parallel()

	src := `package test
type Greeter interface{ Greet() string }
type greeterImpl struct{}
func (greeterImpl) Greet() string { return "" }
func NewImpl() *greeterImpl { return nil }

type app struct {
	G *greeterImpl ` + "`inject:\"returns\"`" + `
}
`
	p, _ := mustBuild(t, src, "app", plan.Options{})
	if p.ReturnType == nil {
		t.Fatal("ReturnType is nil")
	}
	if got := plan.TypeString(p.ReturnType); got != "*test.greeterImpl" {
		t.Errorf("ReturnType = %s, want *test.greeterImpl", got)
	}
	if len(p.Outputs) != 1 || p.Outputs[0].FieldName != "G" {
		t.Errorf("outputs = %+v", p.Outputs)
	}
}

func TestBuild_DirectiveAndTagReturnsConflict(t *testing.T) {
	t.Parallel()

	src := `package test
type Greeter interface{ Greet() string }
type greeterImpl struct{}
func (greeterImpl) Greet() string { return "" }
func NewImpl() *greeterImpl { return nil }

//injector:container returns=Greeter
type app struct {
	_ Greeter        ` + "`inject:\"returns\"`" + `
	G *greeterImpl  ` + "`inject:\"\"`" + `
}
`
	_, ds := build(t, src, "app", plan.Options{})
	if !diag.HasErrors(ds) {
		t.Fatalf("expected conflict error, got %v", ds)
	}
}

func TestBuild_DuplicateArgName(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
type Cache struct{}
type Container struct {
	_  *DB    ` + "`inject:\"arg=primary\"`" + `
	_  *Cache ` + "`inject:\"arg=primary\"`" + `
	DB *DB    ` + "`inject:\"\"`" + `
}
`
	_, ds := build(t, src, "Container", plan.Options{})
	if !diag.HasErrors(ds) {
		t.Fatalf("expected duplicate input name diagnostic, got %v", ds)
	}

	var found bool
	for _, d := range ds {
		if d.Severity == diag.SeverityError && strings.Contains(d.Message, "duplicate input name") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected diagnostic mentioning %q, got %v", "duplicate input name", ds)
	}
}

func TestBuild_MustModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		src    string
		opts   plan.Options
		wantOn bool
	}{
		{
			name: "no directive, cli off",
			src: `package test
type Container struct { X X ` + "`inject:\"\"`" + ` }
type X struct{}
func NewX() X { return X{} }`,
			opts:   plan.Options{Must: false},
			wantOn: false,
		},
		{
			name: "no directive, cli on",
			src: `package test
type Container struct { X X ` + "`inject:\"\"`" + ` }
type X struct{}
func NewX() X { return X{} }`,
			opts:   plan.Options{Must: true},
			wantOn: true,
		},
		{
			name: "directive must=true overrides cli off",
			src: `package test
//injector:container must=true
type Container struct { X X ` + "`inject:\"\"`" + ` }
type X struct{}
func NewX() X { return X{} }`,
			opts:   plan.Options{Must: false},
			wantOn: true,
		},
		{
			name: "directive must=false overrides cli on",
			src: `package test
//injector:container must=false
type Container struct { X X ` + "`inject:\"\"`" + ` }
type X struct{}
func NewX() X { return X{} }`,
			opts:   plan.Options{Must: true},
			wantOn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, _ := mustBuild(t, tt.src, "Container", tt.opts)
			if p.EmitMust != tt.wantOn {
				t.Errorf("EmitMust = %v, want %v", p.EmitMust, tt.wantOn)
			}
		})
	}
}

func TestBuild_NonBlankArgIsStored(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
type Container struct {
	DB *DB ` + "`inject:\"arg\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})

	if len(p.Inputs) != 1 || p.Inputs[0].Name != "db" {
		t.Fatalf("inputs = %+v, want one named db", p.Inputs)
	}
	if len(p.Outputs) != 1 || p.Outputs[0].FieldName != "DB" {
		t.Fatalf("outputs = %+v, want one for DB", p.Outputs)
	}
	if p.Steps[p.Outputs[0].StepIndex].Kind != plan.StepKindInput {
		t.Errorf("expected stored arg output to point at an input step; got %+v",
			p.Steps[p.Outputs[0].StepIndex])
	}
}

func TestBuild_NonBlankArgWithCustomName(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
type Container struct {
	DB *DB ` + "`inject:\"arg=database\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})
	if p.Inputs[0].Name != "database" {
		t.Errorf("input name = %q, want database", p.Inputs[0].Name)
	}
	if p.Outputs[0].FieldName != "DB" {
		t.Errorf("output field = %q, want DB", p.Outputs[0].FieldName)
	}
}

func TestBuild_EmbedExposesFields(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
type Repo struct{}
type Infra struct {
	DB *DB
}
func NewRepo(db *DB) *Repo { return nil }
type Container struct {
	_    *Infra ` + "`inject:\"embed\"`" + `
	Repo *Repo  ` + "`inject:\"\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})

	if len(p.Inputs) != 1 || p.Inputs[0].Name != "infra" {
		t.Fatalf("inputs = %+v, want one named infra", p.Inputs)
	}

	var foundEmbed, foundProvider bool
	for _, s := range p.Steps {
		switch s.Kind {
		case plan.StepKindEmbedField:
			foundEmbed = true
			if s.EmbedFieldName != "DB" {
				t.Errorf("embed field = %q, want DB", s.EmbedFieldName)
			}
			if s.InputIndex != 0 {
				t.Errorf("embed input index = %d, want 0", s.InputIndex)
			}
		case plan.StepKindProvider:
			foundProvider = true
		case plan.StepKindInput:
			// not relevant for this assertion
		}
	}
	if !foundEmbed {
		t.Errorf("no StepKindEmbedField step emitted; steps=%+v", p.Steps)
	}
	if !foundProvider {
		t.Errorf("no provider step emitted")
	}
}

func TestBuild_EmbedPromotedField(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
type Repo struct{}
type Common struct{ DB *DB }
type Infra struct{ Common }
func NewRepo(db *DB) *Repo { return nil }
type Container struct {
	_    *Infra ` + "`inject:\"embed\"`" + `
	Repo *Repo  ` + "`inject:\"\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})

	var embed plan.Step
	for _, s := range p.Steps {
		if s.Kind == plan.StepKindEmbedField {
			embed = s
			break
		}
	}
	if embed.EmbedFieldName == "" {
		t.Fatalf("no embed step found; steps=%+v", p.Steps)
	}
	if got, want := embed.EmbedFieldName, "Common.DB"; got != want {
		t.Errorf("embed field name = %q, want %q", got, want)
	}
	if embed.VarName != "db" {
		t.Errorf("embed var name = %q, want %q", embed.VarName, "db")
	}
}

func TestBuild_EmbedShallowerShadowsDeeper(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
type Repo struct{}
type Common struct{ DB *DB }
type Infra struct {
	Common
	DB *DB
}
func NewRepo(db *DB) *Repo { return nil }
type Container struct {
	_    *Infra ` + "`inject:\"embed\"`" + `
	Repo *Repo  ` + "`inject:\"\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})

	for _, s := range p.Steps {
		if s.Kind != plan.StepKindEmbedField {
			continue
		}
		if s.EmbedFieldName != "DB" {
			t.Errorf("embed field name = %q, want direct DB to shadow Common.DB", s.EmbedFieldName)
		}
	}
}

func TestBuild_EmbedAmbiguousAtSameDepth(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
type Infra struct {
	DB1 *DB
	DB2 *DB
}
func NewRepo(db *DB) *DB { return nil }
type Container struct {
	_  *Infra ` + "`inject:\"embed\"`" + `
	DB *DB   ` + "`inject:\"with=NewRepo\"`" + `
}
`
	_, ds := build(t, src, "Container", plan.Options{})
	if !diag.HasErrors(ds) {
		t.Fatalf("expected ambiguity error for two *DB fields at same depth, got %v", ds)
	}
}

func TestBuild_EmbedRejectsNonStruct(t *testing.T) {
	t.Parallel()

	src := `package test
type Greeter interface{ Greet() string }
type Container struct {
	_ Greeter ` + "`inject:\"embed\"`" + `
}
`
	_, ds := build(t, src, "Container", plan.Options{})
	if !diag.HasErrors(ds) {
		t.Fatalf("expected error diag, got %v", ds)
	}
}

func TestBuild_EmbedAmbiguousAcrossEmbeds(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
type A struct { DB *DB }
type B struct { DB *DB }
type Container struct {
	_ *A ` + "`inject:\"embed\"`" + `
	_ *B ` + "`inject:\"embed\"`" + `
}
`
	_, ds := build(t, src, "Container", plan.Options{})
	if !diag.HasErrors(ds) {
		t.Fatalf("expected error diag for duplicate embed field type, got %v", ds)
	}
}

func TestBuild_DirectArgWinsOverEmbed(t *testing.T) {
	t.Parallel()

	src := `package test
type DB struct{}
type Repo struct{}
type Infra struct { DB *DB }
func NewRepo(db *DB) *Repo { return nil }
type Container struct {
	_    *Infra ` + "`inject:\"embed\"`" + `
	_    *DB    ` + "`inject:\"arg\"`" + `
	Repo *Repo  ` + "`inject:\"\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})

	for _, s := range p.Steps {
		if s.Kind == plan.StepKindEmbedField {
			t.Errorf("did not expect any embed step when direct arg matches; got %+v", s)
		}
	}
}

func TestBuild_TypeAliasResolves(t *testing.T) {
	t.Parallel()

	src := `package test
type Real struct{}
type Alias = Real
func NewAlias() Alias { return Alias{} }
type Container struct {
	Field Alias ` + "`inject:\"\"`" + `
}
`
	p, _ := mustBuild(t, src, "Container", plan.Options{})
	if len(p.Steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(p.Steps))
	}
	if p.Steps[0].Provider == nil || p.Steps[0].Provider.FuncName != "NewAlias" {
		t.Errorf("step[0] = %+v, want NewAlias", p.Steps[0])
	}
}

// build runs scan + plan on src and returns the plan plus all diagnostics.
func build(t *testing.T, src, containerName string, opts plan.Options) (plan.Plan, []diag.Diag) {
	t.Helper()
	pkg := loadTestPackage(t, src)

	cs, dsC := scan.Containers([]*packages.Package{pkg})
	if diag.HasErrors(dsC) {
		t.Fatalf("scan.Containers diags: %v", dsC)
	}
	ps, dsP := scan.Providers([]*packages.Package{pkg})
	if diag.HasErrors(dsP) {
		t.Fatalf("scan.Providers diags: %v", dsP)
	}

	var target ir.Container
	for _, c := range cs {
		if c.StructName == containerName {
			target = c
			break
		}
	}
	if target.StructName == "" {
		t.Fatalf("container %q not found in scan results", containerName)
	}

	idx := plan.NewIndex(ps)
	pl, dsB := plan.Build(target, idx, opts)
	return pl, dsB
}

// mustBuild is like build but fails the test if any error diagnostic is
// produced.
func mustBuild(t *testing.T, src, containerName string, opts plan.Options) (plan.Plan, []diag.Diag) {
	t.Helper()
	pl, ds := build(t, src, containerName, opts)
	if diag.HasErrors(ds) {
		t.Fatalf("Build returned errors: %v", ds)
	}
	return pl, ds
}

// loadTestPackage parses and type-checks a self-contained Go source string.
func loadTestPackage(t *testing.T, src string) *packages.Package {
	t.Helper()

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	conf := &types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("test", fset, []*ast.File{f}, info)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	return &packages.Package{
		PkgPath:   "test",
		Name:      "test",
		Syntax:    []*ast.File{f},
		Types:     pkg,
		TypesInfo: info,
		Fset:      fset,
	}
}
