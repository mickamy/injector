package main

import (
	"github.com/mickamy/injector/example/embedded-error/di"
	"github.com/mickamy/injector/example/embedded-error/service"
)

type App struct {
	_           di.Infra     // embedded container (with error propagation)
	UserService service.User `inject:""`
}

func main() {
	app, err := NewApp()
	if err != nil {
		panic(err)
	}
	if err := app.UserService.Register("Alice", "P@ssw0rd"); err != nil {
		panic(err)
	}
}
