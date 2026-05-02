// Package cli implements injector's command-line entry point. It parses
// flags, drives the scan/plan/emit pipeline for the requested package
// patterns, and writes the generated file into each container package.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickamy/injector/internal/diag"
	"github.com/mickamy/injector/internal/emit"
	"github.com/mickamy/injector/internal/ir"
	"github.com/mickamy/injector/internal/packages"
	"github.com/mickamy/injector/internal/plan"
	"github.com/mickamy/injector/internal/scan"
)

// App is the CLI entry point. Out and Err default to os.Stdout/os.Stderr
// when constructed via New.
type App struct {
	Out     io.Writer
	Err     io.Writer
	Version string
}

// New constructs an App with default writers and the given version string.
func New(version string) *App {
	return &App{
		Out:     os.Stdout,
		Err:     os.Stderr,
		Version: version,
	}
}

// Run parses args (excluding the program name) and returns an exit code:
//
//	0  success
//	1  generation failed (missing providers, ambiguity, write errors, ...)
//	2  invocation problem (bad flags, no patterns)
func (a *App) Run(args []string) int {
	for _, raw := range args {
		switch raw {
		case "--version":
			fmt.Fprintln(a.Out, a.Version)
			return 0
		case "-h", "--help", "help":
			a.printUsage(a.Out)
			return 0
		}
	}

	fs := flag.NewFlagSet("injector", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	var (
		verbose    bool
		tagsRaw    string
		mustFlag   bool
		outputFile string
	)
	fs.BoolVar(&verbose, "v", false, "verbose output")
	fs.BoolVar(&verbose, "verbose", false, "verbose output")
	fs.StringVar(&tagsRaw, "tags", "", "comma-separated build tags")
	fs.BoolVar(&mustFlag, "must", false, "generate MustNew* constructors that panic on error")
	fs.StringVar(&outputFile, "o", "injector_gen.go", "output file name (per package)")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	patterns := fs.Args()
	if len(patterns) == 0 {
		a.printUsage(a.Err)
		return 2
	}

	if verbose {
		fmt.Fprintln(a.Out, "output:", outputFile)
		if tagsRaw != "" {
			fmt.Fprintln(a.Out, "tags:", tagsRaw)
		}
		if mustFlag {
			fmt.Fprintln(a.Out, "must: true")
		}
	}

	res, err := packages.Load(patterns, packages.Config{
		BuildTags: splitTags(tagsRaw),
	})
	if err != nil {
		fmt.Fprintln(a.Err, err)
		return 1
	}
	if verbose {
		fmt.Fprintln(a.Out, "packages:", len(res.Packages))
	}

	containers, dsC := scan.Containers(res.Packages)
	a.printDiags(dsC)
	if diag.HasErrors(dsC) {
		return 1
	}
	if len(containers) == 0 {
		fmt.Fprintln(a.Err, "no container found")
		return 1
	}

	providers, dsP := scan.Providers(res.Packages)
	a.printDiags(dsP)
	if diag.HasErrors(dsP) {
		return 1
	}

	if verbose {
		fmt.Fprintln(a.Out, "containers:", len(containers))
		fmt.Fprintln(a.Out, "providers:", len(providers))
	}

	idx := plan.NewIndex(providers)
	opts := plan.Options{Must: mustFlag}

	failed := false
	for _, group := range groupByPackage(containers) {
		var plans []plan.Plan
		for _, c := range group.containers {
			pl, ds := plan.Build(c, idx, opts)
			a.printDiags(ds)
			if diag.HasErrors(ds) {
				failed = true
				continue
			}
			plans = append(plans, pl)
		}
		if len(plans) == 0 {
			continue
		}

		out, err := emit.Emit(group.pkgName, plans)
		if err != nil {
			fmt.Fprintln(a.Err, err)
			failed = true
			continue
		}

		outDir := filepath.Dir(group.containers[0].Pos.Filename)
		outPath := filepath.Join(outDir, outputFile)
		if err := os.WriteFile(outPath, out, 0644); err != nil {
			fmt.Fprintln(a.Err, err)
			failed = true
			continue
		}

		if verbose {
			fmt.Fprintln(a.Out, "generate:", outPath)
		} else {
			fmt.Fprintln(a.Out, outPath)
		}
	}

	if failed {
		return 1
	}
	return 0
}

// containerGroup is the set of containers that live in a single package and
// will be emitted into one .go file together.
type containerGroup struct {
	pkgPath    string
	pkgName    string
	containers []ir.Container
}

// groupByPackage groups containers by their declaring package, preserving the
// order in which packages are first seen.
func groupByPackage(cs []ir.Container) []containerGroup {
	idxOf := map[string]int{}
	var out []containerGroup
	for _, c := range cs {
		i, ok := idxOf[c.PkgPath]
		if !ok {
			idxOf[c.PkgPath] = len(out)
			out = append(out, containerGroup{
				pkgPath:    c.PkgPath,
				pkgName:    c.PkgName,
				containers: []ir.Container{c},
			})
			continue
		}
		out[i].containers = append(out[i].containers, c)
	}
	return out
}

func (a *App) printDiags(ds []diag.Diag) {
	if len(ds) == 0 {
		return
	}
	fmt.Fprintln(a.Err, diag.Format(ds))
}

func (a *App) printUsage(w io.Writer) {
	fmt.Fprintln(w, "injector — type-safe dependency-injection code generator for Go")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  injector [flags] <packages>...")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -o <file>          output file name per package (default: injector_gen.go)")
	fmt.Fprintln(w, "  --tags <list>      comma-separated build tags")
	fmt.Fprintln(w, "  --must             generate MustNew* constructors that panic on error")
	fmt.Fprintln(w, "  -v, --verbose      verbose output")
	fmt.Fprintln(w, "  --version          print version")
	fmt.Fprintln(w, "  -h, --help         show this help")
}

func splitTags(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
