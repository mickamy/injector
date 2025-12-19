package main

import (
	"github.com/mickamy/injector/example/with-error/config"
	"github.com/mickamy/injector/example/with-error/service"
)

type Container struct {
	_           config.DatabaseConfig `inject:"provider:config.NewReaderDatabaseConfig"`
	UserService service.User          `inject:""`
}

func init() {
	container, err := NewContainer()
	if err != nil {
		panic(err)
	}
	if err := container.UserService.Register("Alice", "P@ssw0rd"); err != nil {
		panic(err)
	}
}
