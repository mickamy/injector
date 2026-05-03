package main

import (
	"os"

	"github.com/mickamy/injector/internal/cli"
)

// version is set via -ldflags at build time. The default is "dev".
var version = "dev"

func main() {
	app := cli.New(version)
	os.Exit(app.Run(os.Args[1:]))
}
