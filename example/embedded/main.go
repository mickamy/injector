package main

import (
	"github.com/mickamy/injector/example/embedded/di"
	"github.com/mickamy/injector/example/embedded/service"
)

type App struct {
	_           di.Infra     // embedded container
	UserService service.User `inject:""`
}

func main() {
	app := NewApp()
	if err := app.UserService.Register("Alice", "P@ssw0rd"); err != nil {
		panic(err)
	}
}
