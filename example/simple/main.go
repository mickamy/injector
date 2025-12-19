package main

import (
	"github.com/mickamy/injector/example/simple/config"
	"github.com/mickamy/injector/example/simple/service"
)

type Container struct {
	_           config.DatabaseConfig `inject:"provider:config.NewReaderDatabaseConfig"`
	UserService service.User          `inject:""`
}

func init() {
	container := NewContainer()
	if err := container.UserService.Register("Alice", "P@ssw0rd"); err != nil {
		panic(err)
	}
}
