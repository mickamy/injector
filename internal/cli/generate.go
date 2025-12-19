package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	var failed bool
	emitInputs := make(map[string]gen.EmitInput)
	for _, c := range containers {
		fields, err := resolve.ConvertContainerFields(c)
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

		g, err := resolve.BuildGraph(fields, rproviders)
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

		outDir := filepath.Dir(positionToFile(c.Position))
		outPath := filepath.Join(outDir, outFile)
		if _, ok := emitInputs[outPath]; ok {
			emitInputs[outPath] = emitInputs[outPath].Append(gen.Container{
				Name:      c.Name,
				Fields:    fields,
				Providers: ordered,
				PkgPath:   c.PkgPath,
				FuncName:  "New" + c.Name,
			})
		} else {
			emitInputs[outPath] = gen.EmitInput{
				PackageName: c.PkgName,
				Containers: []gen.Container{{
					Name:      c.Name,
					Fields:    fields,
					Providers: ordered,
					PkgPath:   c.PkgPath,
					FuncName:  "New" + c.Name,
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
	Tags    string
	Verbose bool
}

// parseGenerateFlags parses flags for `injector generate`.
func parseGenerateFlags(args []string) (generateFlags, []string, error) {
	var gf generateFlags

	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(nil) // prevent flag package from writing to stdout/stderr automatically

	fs.StringVar(&gf.Output, "o", "", "output file name (default: injector_gen.go)")
	fs.StringVar(&gf.Tags, "tags", "", "comma-separated build tags (optional)")
	fs.BoolVar(&gf.Verbose, "v", false, "enable verbose output")
	fs.BoolVar(&gf.Verbose, "verbose", false, "enable verbose output")

	if err := fs.Parse(args); err != nil {
		return generateFlags{}, nil, err
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
