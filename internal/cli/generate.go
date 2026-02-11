package cli

import (
	"flag"
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickamy/injector/internal/config"
	"github.com/mickamy/injector/internal/gen"
	"github.com/mickamy/injector/internal/prints"
	"github.com/mickamy/injector/internal/resolve"
	"github.com/mickamy/injector/internal/scan"
	"github.com/mickamy/injector/internal/workspace"
)

// runGenerate handles the `generate` subcommand.
func (a *App) runGenerate(args []string) int {
	if len(args) == 0 {
		prints.Fprintln(a.err, generateUsage())
		return 2
	}

	flags, rest, err := parseGenerateFlags(args)
	if err != nil {
		prints.Fprintln(a.err, wrapFlagError(err))
		return 2
	}

	patterns := rest

	outFile := flags.Output
	if outFile == "" {
		outFile = "injector_gen.go"
	}

	if flags.Verbose {
		prints.Fprintln(a.out, "output:", outFile)

		if flags.Tags != "" {
			prints.Fprintln(a.out, "tags:", flags.Tags)
		}
		if flags.Must {
			prints.Fprintln(a.out, "must:", flags.Must)
			prints.Fprintln(a.out, "on-error:", flags.OnError)
		}
	}

	loaded, err := workspace.Load(patterns, workspace.LoadConfig{
		BuildTags: splitTags(flags.Tags),
		Tests:     false,
	})
	if err != nil {
		prints.Fprintln(a.err, err.Error())
		return 1
	}

	if flags.Verbose {
		prints.Fprintln(a.out, "number of packages:", len(loaded.Packages))
	}

	containers, err := scan.CollectContainers(loaded.Packages)
	if err != nil {
		prints.Fprintln(a.err, err.Error())
		return 1
	}
	if len(containers) == 0 {
		prints.Fprintln(a.err, "no container found")
		return 1
	}

	providers, err := scan.CollectProviders(loaded.Packages)
	if err != nil {
		prints.Fprintln(a.err, err.Error())
		return 1
	}

	if flags.Verbose {
		prints.Fprintln(a.out, "containers:", len(containers))
		for _, c := range containers {
			prints.Fprintf(a.out, "container: %s.%s (%s)\n", c.PkgPath, c.Name, c.Position)
			for _, f := range c.Fields {
				if f.InjectRaw != "" {
					prints.Fprintf(a.out, "  field: %s %s inject=%q (%s)\n", f.Name, f.TypeExpr, f.InjectRaw, f.Position)
				} else {
					prints.Fprintf(a.out, "  field: %s %s (%s)\n", f.Name, f.TypeExpr, f.Position)
				}
			}
		}

		prints.Fprintln(a.out, "providers:", len(providers))
		for _, p := range providers {
			prints.Fprintf(a.out, "provider: %s.%s -> %s (%s)\n", p.PkgPath, p.Name, p.ResultString, p.Position)
		}
	}

	rproviders, err := resolve.ConvertProviders(providers)
	if err != nil {
		prints.Fprintln(a.err, err.Error())
		return 1
	}

	// Build container registry for embedded container detection.
	containerRegistry := make(map[string]scan.ContainerSpec)
	for _, c := range containers {
		key := c.PkgPath + "." + c.Name
		containerRegistry[key] = c
	}

	// Topologically sort containers so embedded containers are processed first.
	sortedContainers := topoSortContainers(containers, containerRegistry)

	// Filter out MustNew* constructors for all containers — they are convenience
	// wrappers and should never be used as dependency providers.
	var mustNames []string
	for _, c := range containers {
		fn := "MustNew" + strings.ToUpper(c.Name[:1]) + c.Name[1:]
		mustNames = append(mustNames, c.PkgPath+"."+fn)
	}
	rproviders = filterOutByNameWithPkg(rproviders, mustNames)

	// Build container-derived providers for inject:"returns" containers.
	// This ensures providers are available even on the first run (before
	// any generated files exist) or when inject:"returns" is newly added.
	containerProviders := buildContainerAsProviders(containers)
	rproviders = replaceWithContainerProviders(rproviders, containerProviders)

	// Track which containers return error from their generated constructor.
	processedResults := make(map[string]bool)

	var failed bool
	emitInputs := make(map[string]gen.EmitInput)
	for _, c := range sortedContainers {
		fields, err := resolve.ConvertContainerFields(c, containerRegistry)
		if err != nil {
			prints.Fprintln(a.err, err.Error())
			failed = true
			continue
		}
		if len(fields) == 0 {
			prints.Fprintf(a.err, "no injectable fields found in container: %s.%s (%s)\n", c.PkgPath, c.Name, c.Position)
			failed = true
			continue
		}

		// Filter out this container's own generated constructors to avoid
		// conflicts during re-generation while keeping other containers'
		// constructors available as providers.
		funcName := "New" + strings.ToUpper(c.Name[:1]) + c.Name[1:]
		selfProviders := resolve.FilterOutSelf(rproviders, c.PkgPath, []string{
			funcName,
			"Must" + funcName,
		})

		// Detect embedded containers and create synthetic providers.
		embeddedRefs := resolve.DetectEmbeddedContainers(c.Fields, containerRegistry, processedResults)
		syntheticProviders := resolve.CreateSyntheticProviders(embeddedRefs)

		// Detect non-container inject:"param" fields.
		paramProviders := resolve.DetectParamFields(c.Fields, containerRegistry)
		syntheticProviders = append(syntheticProviders, paramProviders...)

		// Field-access providers override regular providers for the same type.
		allProviders := resolve.MergeProviders(selfProviders, syntheticProviders)

		g, err := resolve.BuildGraph(fields, allProviders)
		if err != nil {
			prints.Fprintln(a.err, fmt.Sprintf("failed to build graph for container %s.%s: %v", c.PkgPath, c.Name, err))
			failed = true
			continue
		}

		ordered, err := resolve.OrderProviders(g)
		if err != nil {
			prints.Fprintln(a.err, err.Error())
			failed = true
			continue
		}

		if len(ordered) == 0 {
			prints.Fprintf(
				a.err,
				"resolve: no providers selected for container %s.%s\n",
				c.PkgPath,
				c.Name,
			)
			failed = true
			continue
		}

		// Track whether this container's constructor returns error.
		key := c.PkgPath + "." + c.Name
		returnsErr := false
		for _, p := range ordered {
			if p.ReturnError {
				returnsErr = true
				break
			}
		}
		processedResults[key] = returnsErr

		// Update the container-derived provider's ReturnError now that we
		// know whether any dependency returns an error.  Because topo sort
		// guarantees dependencies are processed first, subsequent containers
		// will see the correct value.
		if cp, ok := containerProviders[c.PkgPath+"."+funcName]; ok {
			cp.ReturnError = returnsErr
		}

		// Detect return type override from `inject:"returns"` field.
		var returnType types.Type
		for _, f := range c.Fields {
			if f.IsReturns {
				returnType = f.Type
				break
			}
		}

		outDir := filepath.Dir(positionToFile(c.Position))
		outPath := filepath.Join(outDir, outFile)
		if _, ok := emitInputs[outPath]; ok {
			emitInputs[outPath] = emitInputs[outPath].Append(gen.Container{
				Name:       c.Name,
				Fields:     fields,
				Providers:  ordered,
				PkgPath:    c.PkgPath,
				FuncName:   "",
				ReturnType: returnType,
			})
		} else {
			emitInputs[outPath] = gen.EmitInput{
				PackageName: c.PkgName,
				OnError:     flags.OnError,
				Containers: []gen.Container{{
					Name:       c.Name,
					Fields:     fields,
					Providers:  ordered,
					PkgPath:    c.PkgPath,
					FuncName:   "",
					ReturnType: returnType,
				}},
			}
		}
	}

	generatedFiles := make(map[string]struct{})
	for outPath, inputs := range emitInputs {
		if _, ok := generatedFiles[outPath]; !ok {
			if err := os.Remove(outPath); err != nil && !os.IsNotExist(err) {
				prints.Fprintln(a.err, err.Error())
				failed = true
				continue
			}
			generatedFiles[outPath] = struct{}{}
		}

		bytes, err := gen.EmitContainers(inputs)
		if err != nil {
			prints.Fprintln(a.err, err.Error())
			failed = true
			continue
		}

		if err := a.write(bytes, outPath); err != nil {
			prints.Fprintln(a.err, err.Error())
			failed = true
			continue
		}

		prints.Fprintln(a.out, "generate:", outPath)
	}

	if failed {
		prints.Fprintln(a.err, "generation failed")
		return 1
	}
	return 0
}

func (a *App) write(bytes []byte, outPath string) error {
	f, err := os.OpenFile(outPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	_, err = f.Write(bytes)
	if err != nil {
		return err
	}

	return nil
}

// generateFlags holds flags for the `generate` subcommand.
type generateFlags struct {
	Output  string
	Must    bool
	OnError *config.OnError
	Tags    string
	Verbose bool
}

// parseGenerateFlags parses flags for `injector generate`.
func parseGenerateFlags(args []string) (generateFlags, []string, error) {
	var gf generateFlags

	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(nil) // prevent flag package from writing to stdout/stderr automatically

	var onErrorRaw string
	fs.StringVar(&gf.Output, "o", "", "output file name (default: injector_gen.go)")
	fs.StringVar(&gf.Tags, "tags", "", "comma-separated build tags (optional)")
	fs.BoolVar(&gf.Must, "must", false, "generate MustNew* constructors that crash on failure (optional)")
	fs.StringVar(&onErrorRaw, "on-error", "", "error handling for MustNew* (panic|fatal). Requires --must (default: panic)")
	fs.BoolVar(&gf.Verbose, "v", false, "enable verbose output")
	fs.BoolVar(&gf.Verbose, "verbose", false, "enable verbose output")

	if err := fs.Parse(args); err != nil {
		return generateFlags{}, nil, err
	}

	if onErrorRaw != "" {
		onError, err := config.NewOnError(onErrorRaw)
		if err != nil {
			return generateFlags{}, nil, fmt.Errorf("invalid on-error value: %w", err)
		}
		gf.OnError = &onError
	}

	if gf.Must && gf.OnError == nil {
		gf.OnError = &config.OnErrorPanic
	}

	return gf, fs.Args(), nil
}

// generateUsage returns the usage text for `generate`.
func generateUsage() string {
	return strings.Join([]string{
		"Usage:",
		"  injector generate [flags] <packages>",
		"",
		"Examples:",
		"  injector generate ./...",
		"  injector generate -o injector_gen.go ./...",
		"",
		"Flags:",
		"  -o, --output      output file name (default: injector_gen.go)",
		"  -v, --verbose     enable verbose output",
	}, "\n")
}

// wrapFlagError turns a flag parsing error into a human-friendly message.
func wrapFlagError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%v\n\n%s", err, generateUsage())
}

// splitTags splits a comma-separated build tag string into a slice.
func splitTags(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// topoSortContainers sorts containers so that embedded (referenced) containers
// appear before containers that embed them.
func topoSortContainers(containers []scan.ContainerSpec, registry map[string]scan.ContainerSpec) []scan.ContainerSpec {
	keyOf := func(c scan.ContainerSpec) string {
		return c.PkgPath + "." + c.Name
	}

	// Build adjacency: container → set of containers it embeds.
	deps := make(map[string][]string)
	for _, c := range containers {
		k := keyOf(c)
		for _, f := range c.Fields {
			if !f.IsEmbeddedCandidate || f.Type == nil {
				continue
			}
			t := f.Type
			if ptr, ok := t.(*types.Pointer); ok {
				t = ptr.Elem()
			}
			named, ok := t.(*types.Named)
			if !ok {
				continue
			}
			obj := named.Obj()
			if obj == nil || obj.Pkg() == nil {
				continue
			}
			refKey := obj.Pkg().Path() + "." + obj.Name()
			if _, ok := registry[refKey]; ok {
				deps[k] = append(deps[k], refKey)
			}
		}
	}

	// Build returns-type → container key mapping for inject:"returns" deps.
	returnsByType := make(map[string]string)
	for _, c := range containers {
		k := keyOf(c)
		for _, f := range c.Fields {
			if f.IsReturns && f.Type != nil {
				returnsByType[containerTypeKey(f.Type)] = k
			}
		}
	}

	// Add dependency edges for fields whose type matches another container's returns type.
	for _, c := range containers {
		k := keyOf(c)
		for _, f := range c.Fields {
			if f.IsEmbeddedCandidate || f.IsReturns || f.Type == nil {
				continue
			}
			typeKey := containerTypeKey(f.Type)
			if depKey, ok := returnsByType[typeKey]; ok && depKey != k {
				deps[k] = append(deps[k], depKey)
			}
		}
	}

	visited := make(map[string]bool)
	var order []string
	var visit func(key string)
	visit = func(key string) {
		if visited[key] {
			return
		}
		visited[key] = true
		for _, d := range deps[key] {
			visit(d)
		}
		order = append(order, key)
	}
	for _, c := range containers {
		visit(keyOf(c))
	}

	byKey := make(map[string]scan.ContainerSpec)
	for _, c := range containers {
		byKey[keyOf(c)] = c
	}
	out := make([]scan.ContainerSpec, 0, len(order))
	for _, k := range order {
		if c, ok := byKey[k]; ok {
			out = append(out, c)
		}
	}
	return out
}

func filterOutByNameWithPkg(providers []*resolve.Provider, names []string) []*resolve.Provider {
	exclude := make(map[string]struct{}, len(names))
	for _, n := range names {
		exclude[n] = struct{}{}
	}
	out := make([]*resolve.Provider, 0, len(providers))
	for _, p := range providers {
		if _, ok := exclude[p.NameWithPkg]; ok {
			continue
		}
		out = append(out, p)
	}
	return out
}

func positionToFile(pos string) string {
	// pos format: "/path/to/file.go:line:col"
	// We split from the right to keep Windows drive letters safe-ish.
	i := strings.LastIndexByte(pos, ':')
	if i < 0 {
		return pos
	}
	j := strings.LastIndexByte(pos[:i], ':')
	if j < 0 {
		return pos[:i]
	}
	return pos[:j]
}

// containerTypeKey returns a fully-qualified type string for use as a map key.
func containerTypeKey(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p == nil {
			return ""
		}
		return p.Path()
	})
}

// buildContainerAsProviders creates synthetic providers from containers that
// have an inject:"returns" field. This allows other containers to depend on
// these return types without relying on previously generated files.
func buildContainerAsProviders(containers []scan.ContainerSpec) map[string]*resolve.Provider {
	result := make(map[string]*resolve.Provider)
	for _, c := range containers {
		var returnsType types.Type
		for _, f := range c.Fields {
			if f.IsReturns {
				returnsType = f.Type
				break
			}
		}
		if returnsType == nil {
			continue
		}

		var params []types.Type
		for _, f := range c.Fields {
			if f.IsEmbeddedCandidate {
				params = append(params, f.Type)
			}
		}

		name := "New" + strings.ToUpper(c.Name[:1]) + c.Name[1:]
		nameWithPkg := c.PkgPath + "." + name
		result[nameWithPkg] = &resolve.Provider{
			PkgPath:     c.PkgPath,
			Name:        name,
			NameWithPkg: nameWithPkg,
			ResultType:  returnsType,
			Params:      params,
			ReturnError: false,
		}
	}
	return result
}

// replaceWithContainerProviders removes scanned providers that conflict with
// container-derived providers, then appends the container-derived ones.
func replaceWithContainerProviders(providers []*resolve.Provider, containerProviders map[string]*resolve.Provider) []*resolve.Provider {
	out := make([]*resolve.Provider, 0, len(providers))
	for _, p := range providers {
		if _, ok := containerProviders[p.NameWithPkg]; ok {
			continue
		}
		out = append(out, p)
	}
	for _, cp := range containerProviders {
		out = append(out, cp)
	}
	return out
}
