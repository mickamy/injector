package plan

import (
	"fmt"
	"go/token"
	"go/types"
	"strings"
	"unicode"

	"github.com/mickamy/injector/internal/diag"
	"github.com/mickamy/injector/internal/ir"
)

// Options carries CLI-level defaults that may be overridden per container by
// a //injector:container directive.
type Options struct {
	// Must is the default value applied to containers whose directive does
	// not explicitly specify must.
	Must bool
}

// Plan is the resolved sequence of operations needed to construct a single
// container, plus metadata used by the emit layer.
//
// ReturnType is the *declared* return type of the constructor. The emitter
// always produces &<StructName>{...} as the return expression and relies on
// Go's assignability rules to fit ReturnType — so when ReturnType differs
// from *<StructName> (e.g. via inject:"returns" or //injector:container
// returns=...), *<StructName> must implement (or be identical to)
// ReturnType. This is also why RoleReturnsOnly fields contribute no Step:
// their type is recorded for the signature but the value is supplied by
// the container struct literal itself.
type Plan struct {
	Container       ir.Container
	ConstructorName string
	ReturnType      types.Type
	EmitMust        bool
	ReturnsError    bool

	// Inputs are constructor parameters in container-field order.
	Inputs []Input
	// Steps are resolution operations in execution order (deps first).
	Steps []Step
	// Outputs map RoleOut field names to the step that produces their value.
	Outputs []Output
}

// StepKind classifies a step in a Plan.
type StepKind int

const (
	// StepKindProvider invokes a provider function with the given args.
	StepKindProvider StepKind = iota
	// StepKindInput refers to a constructor input parameter.
	StepKindInput
	// StepKindEmbedField refers to an exported field of an inject:"embed"
	// input, accessed as <input>.<FieldName>.
	StepKindEmbedField
)

// Step is a single resolution operation.
type Step struct {
	Kind    StepKind
	VarName string
	OutType types.Type

	// For StepKindProvider:
	Provider *ir.Provider
	ArgSteps []int

	// For StepKindInput and StepKindEmbedField:
	InputIndex int

	// For StepKindEmbedField:
	EmbedFieldName string
}

// Input is a constructor parameter (declared via inject:"arg" or
// inject:"embed").
type Input struct {
	Name string
	Type types.Type
}

// Output is a RoleOut field assignment: which step's value goes into which
// container field.
type Output struct {
	FieldName string
	StepIndex int
}

// Build resolves dependencies for one container against the given provider
// index and CLI defaults, returning a Plan that the emit layer can consume.
func Build(c ir.Container, idx *Index, opts Options) (Plan, []diag.Diag) {
	var diags []diag.Diag

	constructorName := constructorNameFor(c)
	returnType, retDiag := resolveReturnType(c)
	if retDiag != nil {
		diags = append(diags, *retDiag)
	}
	emitMust := mergeMust(c.Directive.Must, opts.Must)

	inputs, inDiags := buildInputs(c)
	diags = append(diags, inDiags...)

	overrides, ovDiags := buildOverrides(c, idx)
	diags = append(diags, ovDiags...)

	embeds, emDiags := buildEmbeds(c, inputs)
	diags = append(diags, emDiags...)

	r := &resolver{
		idx:       idx,
		inputs:    inputs,
		overrides: overrides,
		embeds:    embeds,
		stepByKey: map[string]int{},
		active:    map[string]bool{},
	}

	var outputs []Output
	for _, f := range c.Fields {
		switch f.Role {
		case ir.RoleOut:
			stepIdx, ds := r.resolveField(f)
			diags = append(diags, ds...)
			if stepIdx < 0 {
				continue
			}
			outputs = append(outputs, Output{FieldName: f.Name, StepIndex: stepIdx})
		case ir.RoleArg:
			if f.Name == "_" {
				continue
			}
			stepIdx, ds := r.resolveByType(f.Type, f.Pos, "field "+f.Name)
			diags = append(diags, ds...)
			if stepIdx < 0 {
				continue
			}
			outputs = append(outputs, Output{FieldName: f.Name, StepIndex: stepIdx})
		case ir.RoleOverride, ir.RoleReturnsOnly, ir.RoleEmbed:
			// Handled in buildOverrides / resolveReturnType / buildEmbeds.
		}
	}

	returnsErr := false
	for _, s := range r.steps {
		if s.Kind == StepKindProvider && s.Provider != nil && s.Provider.ReturnsError {
			returnsErr = true
			break
		}
	}

	return Plan{
		Container:       c,
		ConstructorName: constructorName,
		ReturnType:      returnType,
		EmitMust:        emitMust,
		ReturnsError:    returnsErr,
		Inputs:          inputs,
		Steps:           r.steps,
		Outputs:         outputs,
	}, diags
}

// resolver holds mutable state during resolution.
type resolver struct {
	idx       *Index
	inputs    []Input
	overrides map[string]*ir.Provider // typeKey → provider
	embeds    map[string]embedSource  // typeKey → embed source
	steps     []Step
	stepByKey map[string]int
	active    map[string]bool
}

// embedSource describes one exported field of an inject:"embed" input that
// is exposed as a resolution source.
type embedSource struct {
	InputIndex int
	FieldName  string
	FieldType  types.Type
}

func (r *resolver) resolveField(f ir.Field) (int, []diag.Diag) {
	if f.ProviderRef.HasRef() {
		return r.resolveByRef(f.Type, f.ProviderRef.Raw, f.Pos)
	}
	return r.resolveByType(f.Type, f.Pos, "field "+f.Name)
}

func (r *resolver) resolveByType(want types.Type, pos token.Position, parent string) (int, []diag.Diag) {
	tk := TypeKey(want)

	for i, in := range r.inputs {
		if TypeKey(in.Type) == tk {
			return r.useInput(i), nil
		}
	}

	if p, ok := r.overrides[tk]; ok {
		return r.resolveProvider(p, pos)
	}

	if es, ok := r.embeds[tk]; ok {
		return r.useEmbed(es), nil
	}

	candidates := r.idx.LookupByType(want)
	if len(candidates) == 0 {
		return -1, []diag.Diag{
			diag.Errorf(pos, "no provider for %s (required by %s)", TypeString(want), parent),
		}
	}
	if len(candidates) > 1 {
		return -1, []diag.Diag{
			diag.Errorf(pos, "multiple providers for %s (required by %s)", TypeString(want), parent).
				WithHints(FormatCandidates(candidates)...),
		}
	}
	return r.resolveProvider(candidates[0], pos)
}

func (r *resolver) resolveByRef(want types.Type, ref string, pos token.Position) (int, []diag.Diag) {
	candidates := r.idx.LookupByRef(ref)
	if len(candidates) == 0 {
		return -1, []diag.Diag{
			diag.Errorf(pos, "no provider matches %q", ref),
		}
	}
	var matched []*ir.Provider
	for _, p := range candidates {
		if p.Result != nil && types.Identical(p.Result, want) {
			matched = append(matched, p)
		}
	}
	if len(matched) == 0 {
		return -1, []diag.Diag{
			diag.Errorf(pos, "provider %q does not produce %s", ref, TypeString(want)).
				WithHints(FormatCandidates(candidates)...),
		}
	}
	if len(matched) > 1 {
		return -1, []diag.Diag{
			diag.Errorf(pos, "reference %q is ambiguous", ref).
				WithHints(FormatCandidates(matched)...),
		}
	}
	return r.resolveProvider(matched[0], pos)
}

func (r *resolver) useInput(idx int) int {
	key := fmt.Sprintf("input:%d", idx)
	if id, ok := r.stepByKey[key]; ok {
		return id
	}
	in := r.inputs[idx]
	r.steps = append(r.steps, Step{
		Kind:       StepKindInput,
		VarName:    in.Name,
		OutType:    in.Type,
		InputIndex: idx,
	})
	id := len(r.steps) - 1
	r.stepByKey[key] = id
	return id
}

func (r *resolver) useEmbed(es embedSource) int {
	key := fmt.Sprintf("embed:%d:%s", es.InputIndex, es.FieldName)
	if id, ok := r.stepByKey[key]; ok {
		return id
	}
	r.steps = append(r.steps, Step{
		Kind:           StepKindEmbedField,
		VarName:        varNameForEmbed(es, r.steps),
		OutType:        es.FieldType,
		InputIndex:     es.InputIndex,
		EmbedFieldName: es.FieldName,
	})
	id := len(r.steps) - 1
	r.stepByKey[key] = id
	return id
}

func (r *resolver) resolveProvider(p *ir.Provider, pos token.Position) (int, []diag.Diag) {
	key := "provider:" + ProviderName(p)
	if id, ok := r.stepByKey[key]; ok {
		return id, nil
	}
	if r.active[key] {
		return -1, []diag.Diag{
			diag.Errorf(pos, "circular dependency at %s", ProviderName(p)),
		}
	}
	r.active[key] = true
	defer delete(r.active, key)

	var argIDs []int
	var diags []diag.Diag
	for _, pt := range p.Params {
		argID, ds := r.resolveByType(pt, pos, ProviderName(p))
		diags = append(diags, ds...)
		if argID < 0 {
			return -1, diags
		}
		argIDs = append(argIDs, argID)
	}

	r.steps = append(r.steps, Step{
		Kind:     StepKindProvider,
		VarName:  varNameForProvider(p, r.steps),
		OutType:  p.Result,
		Provider: p,
		ArgSteps: argIDs,
	})
	id := len(r.steps) - 1
	r.stepByKey[key] = id
	return id, diags
}

func constructorNameFor(c ir.Container) string {
	if c.Directive.Name != "" {
		return c.Directive.Name
	}
	return "New" + upperFirst(c.StructName)
}

func resolveReturnType(c ir.Container) (types.Type, *diag.Diag) {
	var taggedReturns *ir.Field
	for i := range c.Fields {
		f := &c.Fields[i]
		if !f.IsReturns {
			continue
		}
		if taggedReturns != nil {
			d := diag.Errorf(f.Pos,
				`multiple inject:"returns" fields (also at %s)`, taggedReturns.Pos)
			return nil, &d
		}
		taggedReturns = f
	}

	if c.Directive.ReturnType != nil {
		if taggedReturns != nil {
			d := diag.Errorf(c.Pos,
				`directive returns= conflicts with inject:"returns" on field %s`,
				taggedReturns.Name)
			return nil, &d
		}
		return c.Directive.ReturnType, nil
	}

	if taggedReturns != nil {
		return taggedReturns.Type, nil
	}
	if c.StructType != nil {
		return types.NewPointer(c.StructType), nil
	}
	return nil, nil
}

func mergeMust(d ir.MustMode, cliMust bool) bool {
	switch d {
	case ir.MustOn:
		return true
	case ir.MustOff:
		return false
	case ir.MustUnset:
		fallthrough
	default:
		return cliMust
	}
}

func buildInputs(c ir.Container) ([]Input, []diag.Diag) {
	var inputs []Input
	var diags []diag.Diag
	seenTypes := map[string]token.Position{}
	seenNames := map[string]token.Position{}
	for _, f := range c.Fields {
		if f.Role != ir.RoleArg && f.Role != ir.RoleEmbed {
			continue
		}
		name := f.ArgName
		if name == "" {
			name = deriveInputName(f.Type)
		}
		tk := TypeKey(f.Type)
		if prev, ok := seenTypes[tk]; ok {
			diags = append(diags, diag.Errorf(f.Pos,
				"duplicate input type %s (first declared at %s)", TypeString(f.Type), prev))
			continue
		}
		if prev, ok := seenNames[name]; ok {
			diags = append(diags, diag.Errorf(f.Pos,
				`duplicate input name %q (first declared at %s); use inject:"arg=..." to disambiguate`,
				name, prev))
			continue
		}
		seenTypes[tk] = f.Pos
		seenNames[name] = f.Pos
		inputs = append(inputs, Input{Name: name, Type: f.Type})
	}
	return inputs, diags
}

// buildEmbeds walks the container's RoleEmbed fields and returns a TypeKey
// → embedSource map of exported sub-fields available as resolution sources.
// Each embed input must be a struct (or pointer to a struct); other shapes
// produce diagnostics. Promoted fields reached through anonymous embeds
// are also exposed; shallower fields shadow deeper ones inside a single
// embed (matching Go's selector semantics), while equal-depth duplicates
// within one embed and same-type sources across two embeds are both
// reported as errors.
func buildEmbeds(c ir.Container, inputs []Input) (map[string]embedSource, []diag.Diag) {
	out := map[string]embedSource{}
	var diags []diag.Diag

	indexByType := make(map[string]int, len(inputs))
	for i, in := range inputs {
		indexByType[TypeKey(in.Type)] = i
	}

	for _, f := range c.Fields {
		if f.Role != ir.RoleEmbed {
			continue
		}
		idx, ok := indexByType[TypeKey(f.Type)]
		if !ok {
			// The corresponding input was rejected (duplicate type/name).
			continue
		}
		st, ok := structOf(f.Type)
		if !ok {
			diags = append(diags, diag.Errorf(f.Pos,
				`inject:"embed" requires a struct or pointer to struct, got %s`,
				TypeString(f.Type)))
			continue
		}
		sources, srcDiags := embedSourcesOf(f.Type, st, idx, f.Pos, inputs)
		diags = append(diags, srcDiags...)
		for tk, src := range sources {
			if existing, dup := out[tk]; dup {
				diags = append(diags, diag.Errorf(f.Pos,
					"embed: multiple sources for %s (also %s.%s)",
					TypeString(src.FieldType),
					inputs[existing.InputIndex].Name, existing.FieldName))
				continue
			}
			out[tk] = src
		}
	}
	return out, diags
}

// embedSourcesOf walks a single embed input breadth-first, recording each
// exported field (direct or promoted through anonymous embeds) keyed by
// TypeKey. The traversal mirrors Go's selector promotion: a shallower
// field wins over deeper ones of the same type, while same-depth
// duplicates are reported as ambiguity diagnostics and skipped.
func embedSourcesOf(
	rootType types.Type,
	rootSt *types.Struct,
	inputIdx int,
	fPos token.Position,
	inputs []Input,
) (map[string]embedSource, []diag.Diag) {
	out := map[string]embedSource{}
	claimed := map[string]bool{}
	var diags []diag.Diag

	type frame struct {
		st     *types.Struct
		prefix string
	}
	visited := map[string]bool{TypeKey(rootType): true}
	level := []frame{{rootSt, ""}}

	for len(level) > 0 {
		var next []frame
		levelCands := map[string][]embedSource{}

		for _, fr := range level {
			for sf := range fr.st.Fields() {
				if !sf.Exported() {
					continue
				}
				name := sf.Name()
				if fr.prefix != "" {
					name = fr.prefix + "." + name
				}
				tk := TypeKey(sf.Type())
				if !claimed[tk] {
					levelCands[tk] = append(levelCands[tk], embedSource{
						InputIndex: inputIdx,
						FieldName:  name,
						FieldType:  sf.Type(),
					})
				}
				if sf.Anonymous() && !visited[tk] {
					visited[tk] = true
					if subst, ok := structOf(sf.Type()); ok {
						next = append(next, frame{subst, name})
					}
				}
			}
		}

		for tk, cands := range levelCands {
			if len(cands) > 1 {
				names := make([]string, 0, len(cands))
				for _, c := range cands {
					names = append(names, inputs[c.InputIndex].Name+"."+c.FieldName)
				}
				diags = append(diags, diag.Errorf(fPos,
					"embed: ambiguous source for %s at the same depth (%s)",
					TypeString(cands[0].FieldType), strings.Join(names, ", ")))
				claimed[tk] = true
				continue
			}
			out[tk] = cands[0]
			claimed[tk] = true
		}

		level = next
	}
	return out, diags
}

// structOf returns the underlying *types.Struct of t (unwrapping a leading
// pointer and resolving type aliases) and reports whether t had a struct
// shape at all.
func structOf(t types.Type) (*types.Struct, bool) {
	t = types.Unalias(t)
	if ptr, ok := t.(*types.Pointer); ok {
		t = types.Unalias(ptr.Elem())
	}
	if named, ok := t.(*types.Named); ok {
		if st, ok := named.Underlying().(*types.Struct); ok {
			return st, true
		}
		return nil, false
	}
	if st, ok := t.(*types.Struct); ok {
		return st, true
	}
	return nil, false
}

// buildOverrides walks fields whose inject tag names a specific provider
// (`inject:"with=..."`) and indexes them by their declared type. The
// resolver consults this map ahead of provider-by-type lookup, so that any
// transitive dependency inside the same container resolves to the same
// provider the user picked for the field.
//
// Both blank (RoleOverride) and non-blank (RoleOut with a ref) fields
// contribute. The non-blank case lets a stored field double as a
// container-wide override; users no longer need a redundant blank twin to
// disambiguate sibling resolutions.
func buildOverrides(c ir.Container, idx *Index) (map[string]*ir.Provider, []diag.Diag) {
	out := map[string]*ir.Provider{}
	posByType := map[string]token.Position{}
	var diags []diag.Diag
	for _, f := range c.Fields {
		if f.Role != ir.RoleOverride && f.Role != ir.RoleOut {
			continue
		}
		if !f.ProviderRef.HasRef() {
			continue
		}
		candidates := idx.LookupByRef(f.ProviderRef.Raw)
		if len(candidates) == 0 {
			diags = append(diags, diag.Errorf(f.Pos,
				"no provider matches %q", f.ProviderRef.Raw))
			continue
		}
		var matched []*ir.Provider
		for _, p := range candidates {
			if p.Result != nil && types.Identical(p.Result, f.Type) {
				matched = append(matched, p)
			}
		}
		switch len(matched) {
		case 0:
			diags = append(diags, diag.Errorf(f.Pos,
				"provider %q does not produce %s", f.ProviderRef.Raw, TypeString(f.Type)).
				WithHints(FormatCandidates(candidates)...))
		case 1:
			tk := TypeKey(f.Type)
			if existing, dup := out[tk]; dup && existing != matched[0] {
				diags = append(diags, diag.Errorf(f.Pos,
					"conflicting providers selected for %s: %s vs %s (also at %s)",
					TypeString(f.Type),
					ProviderName(existing), ProviderName(matched[0]),
					posByType[tk]))
				continue
			}
			out[tk] = matched[0]
			posByType[tk] = f.Pos
		default:
			diags = append(diags, diag.Errorf(f.Pos,
				"reference %q is ambiguous", f.ProviderRef.Raw).
				WithHints(FormatCandidates(matched)...))
		}
	}
	return out, diags
}

func deriveInputName(t types.Type) string {
	if t == nil {
		return "arg"
	}
	if ptr, ok := t.(*types.Pointer); ok {
		return deriveInputName(ptr.Elem())
	}
	if named, ok := t.(*types.Named); ok {
		if obj := named.Obj(); obj != nil && obj.Name() != "" {
			return lowerFirst(obj.Name())
		}
	}
	return "arg"
}

func varNameForEmbed(es embedSource, existing []Step) string {
	// FieldName may be a dotted selector (e.g. "CommonInfra.DB") when the
	// source comes from a promoted field; only the leaf segment is a valid
	// identifier base.
	leaf := es.FieldName
	if i := strings.LastIndex(leaf, "."); i >= 0 {
		leaf = leaf[i+1:]
	}
	base := lowerFirst(leaf)
	if base == "" {
		base = "v"
	}
	used := map[string]struct{}{}
	for _, s := range existing {
		used[s.VarName] = struct{}{}
	}
	if _, ok := used[base]; !ok {
		return base
	}
	for i := 2; ; i++ {
		try := fmt.Sprintf("%s%d", base, i)
		if _, ok := used[try]; !ok {
			return try
		}
	}
}

func varNameForProvider(p *ir.Provider, existing []Step) string {
	base := p.FuncName
	if strings.HasPrefix(base, "New") && len(base) > 3 {
		base = base[3:]
	}
	base = lowerFirst(base)
	if base == "" {
		base = "v"
	}

	used := map[string]struct{}{}
	for _, s := range existing {
		used[s.VarName] = struct{}{}
	}
	if _, ok := used[base]; !ok {
		return base
	}
	for i := 2; ; i++ {
		try := fmt.Sprintf("%s%d", base, i)
		if _, ok := used[try]; !ok {
			return try
		}
	}
}

func upperFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// lowerFirst lowercases the leading run of uppercase letters of s, following
// the Go convention that "URL" → "url" and "URLPath" → "urlPath" (the last
// cap of a leading run is preserved when followed by a lowercase letter).
func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	n := len(runes)
	i := 0
	for i < n && unicode.IsUpper(runes[i]) {
		i++
	}
	if i == 0 {
		return s
	}
	if i < n && i > 1 {
		i--
	}
	for j := range i {
		runes[j] = unicode.ToLower(runes[j])
	}
	return string(runes)
}
