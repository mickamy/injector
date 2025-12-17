package cli

import (
	"io"
	"os"
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
	fprintln(a.err, "injector is a Go DI code generator.")
	fprintln(a.err, "")
	fprintln(a.err, "Usage:")
	fprintln(a.err, "  injector <command> [flags]")
	fprintln(a.err, "")
	fprintln(a.err, "Commands:")
	fprintln(a.err, "  generate   Generate injector code for packages")
	fprintln(a.err, "  version    Print version information")
	fprintln(a.err, "  help       Show help")
}

func (a *App) printUnknownCommand(cmd string) {
	fprintln(a.err, "unknown command:", cmd)
	fprintln(a.err, "")
	a.printUsage()
}

func (a *App) printVersion() {
	fprintln(a.out, a.version)
}
