package main

import (
	"github.com/mickamy/injector/example/config"
	"github.com/mickamy/injector/example/infra"
	"github.com/mickamy/injector/example/service"
)

type Container struct {
	_           config.DatabaseConfig `inject:"provider:config.NewReaderDatabaseConfig"`
	Database    *infra.Database       `inject:""`
	UserService service.User          `inject:""`
}

func init() {
	container := NewContainer()
	if err := container.UserService.Register("alice", "P@ssw0rd"); err != nil {
		panic(err)
	}
}
