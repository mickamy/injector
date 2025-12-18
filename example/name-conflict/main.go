package main

import (
	"github.com/mickamy/injector/example/name-conflict/config"
	"github.com/mickamy/injector/example/name-conflict/domain/task"
	"github.com/mickamy/injector/example/name-conflict/domain/user"
	"github.com/mickamy/injector/example/name-conflict/infra"
)

type TaskContainer struct {
	_       config.DatabaseConfig `inject:"provider:config.NewWriterDatabaseConfig"`
	_       infra.Database        `inject:""`
	Service task.Service          `inject:""`
}

type UserContainer struct {
	_       config.DatabaseConfig `inject:"provider:config.NewWriterDatabaseConfig"`
	_       infra.Database        `inject:""`
	Service user.Service          `inject:""`
}

func main() {
}
