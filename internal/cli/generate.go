package cli

import (
	"flag"
	"fmt"
	"strings"
)

// runGenerate handles the `generate` subcommand.
func (a *App) runGenerate(args []string) int {
	if len(args) == 0 {
		fprintln(a.err, generateUsage())
		return 2
	}

	flags, rest, err := parseGenerateFlags(args)
	if err != nil {
		fprintln(a.err, wrapFlagError(err))
		return 2
	}

	patterns := rest

	outFile := flags.Output
	if outFile == "" {
		outFile = "injector_gen.go"
	}

	if flags.Verbose {
		fprintln(a.out, "output:", outFile)
	}

	fprintln(a.out, "patterns:", patterns)

	return 0
}

// generateFlags holds flags for the `generate` subcommand.
type generateFlags struct {
	Output  string
	Verbose bool
}

// parseGenerateFlags parses flags for `injector generate`.
func parseGenerateFlags(args []string) (generateFlags, []string, error) {
	var gf generateFlags

	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(nil) // prevent flag package from writing to stdout/stderr automatically

	fs.StringVar(&gf.Output, "o", "", "output file name (default: injector_gen.go)")
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
