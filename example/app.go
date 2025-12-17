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
