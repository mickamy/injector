package main

import (
	"github.com/mickamy/injector/example/embedded/di"
	"github.com/mickamy/injector/example/embedded/service"
)

type App struct {
	_           di.Infra `inject:"param"`
	UserService service.User `inject:""`
}

func main() {
	infra := di.NewInfra()
	app := NewApp(infra)
	if err := app.UserService.Register("Alice", "P@ssw0rd"); err != nil {
		panic(err)
	}
}
