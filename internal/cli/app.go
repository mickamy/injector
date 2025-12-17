package cli

import (
	"io"
	"os"

	"github.com/mickamy/injector/internal/prints"
)

// App is the entry point for the CLI application.
type App struct {
	out     io.Writer
	err     io.Writer
	version string
}

// NewApp creates a new CLI application with default writers.
func NewApp(version string) *App {
	return &App{
		out:     os.Stdout,
		err:     os.Stderr,
		version: version,
	}
}

// Run executes the CLI with the given args and returns an exit code.
func (a *App) Run(args []string) int {
	if len(args) < 2 {
		a.printUsage()
		return 2
	}

	switch args[1] {
	case "generate":
		return a.runGenerate(args[2:])
	case "help", "-h", "--help":
		a.printUsage()
		return 0
	case "version", "-v", "--version":
		a.printVersion()
		return 0
	default:
		a.printUnknownCommand(args[1])
		return 2
	}
}

func (a *App) printUsage() {
	prints.Fprintln(a.err, "injector is a Go DI code generator.")
	prints.Fprintln(a.err, "")
	prints.Fprintln(a.err, "Usage:")
	prints.Fprintln(a.err, "  injector <command> [flags]")
	prints.Fprintln(a.err, "")
	prints.Fprintln(a.err, "Commands:")
	prints.Fprintln(a.err, "  generate   Generate injector code for packages")
	prints.Fprintln(a.err, "  version    Print version information")
	prints.Fprintln(a.err, "  help       Show help")
}

func (a *App) printUnknownCommand(cmd string) {
	prints.Fprintln(a.err, "unknown command:", cmd)
	prints.Fprintln(a.err, "")
	a.printUsage()
}

func (a *App) printVersion() {
	prints.Fprintln(a.out, a.version)
}
