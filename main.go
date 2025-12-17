package main

import (
	"os"

	"github.com/mickamy/injector/internal/cli"
)

// version is set via -ldflags at build time.
// Default is "dev".
var version = "dev"

func main() {
	app := cli.NewApp(version)
	code := app.Run(os.Args)
	os.Exit(code)
}
