package main

import (
	"github.com/mickamy/injector/example/embedded-error/di"
	"github.com/mickamy/injector/example/embedded-error/infra"
	"github.com/mickamy/injector/example/embedded-error/service"
)

type App struct {
	_           *infra.Database `inject:"arg"`
	UserService service.User    `inject:""`
}

func main() {
	inf := di.MustNewInfra()
	app := MustNewApp(inf.Database)
	if err := app.UserService.Register("Alice", "P@ssw0rd"); err != nil {
		panic(err)
	}
}
