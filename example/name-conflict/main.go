package main

import (
	"github.com/mickamy/injector/example/name-conflict/config"
	"github.com/mickamy/injector/example/name-conflict/domain/task"
	"github.com/mickamy/injector/example/name-conflict/domain/user"
)

type TaskContainer struct {
	_       config.DatabaseConfig `inject:"provider:config.NewWriterDatabaseConfig"`
	Service task.Service          `inject:""`
}

type UserContainer struct {
	_       config.DatabaseConfig `inject:"provider:config.NewWriterDatabaseConfig"`
	Service user.Service          `inject:""`
}

func main() {
}
