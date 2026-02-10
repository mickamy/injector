package main

import (
	"github.com/mickamy/injector/example/embedded-error/di"
	"github.com/mickamy/injector/example/embedded-error/service"
)

type App struct {
	_           di.Infra `inject:"param"`
	UserService service.User `inject:""`
}

func main() {
	infra, err := di.NewInfra()
	if err != nil {
		panic(err)
	}
	app := NewApp(infra)
	if err := app.UserService.Register("Alice", "P@ssw0rd"); err != nil {
		panic(err)
	}
}
