package main

import (
	"github.com/mickamy/injector/example/embedded/di"
	"github.com/mickamy/injector/example/embedded/infra"
	"github.com/mickamy/injector/example/embedded/service"
)

type App struct {
	_           *infra.Database `inject:"arg"`
	UserService service.User    `inject:""`
}

func main() {
	inf := di.NewInfra()
	app := NewApp(inf.Database)
	if err := app.UserService.Register("Alice", "P@ssw0rd"); err != nil {
		panic(err)
	}
}
