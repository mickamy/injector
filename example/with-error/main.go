package main

import (
	"github.com/mickamy/injector/example/with-error/config"
	"github.com/mickamy/injector/example/with-error/service"
)

type Container struct {
	_           config.DatabaseConfig `inject:"with=config.NewReaderDatabaseConfig"`
	UserService service.User          `inject:""`
}

func main() {
	c := MustNewContainer()
	if err := c.UserService.Register("Alice", "P@ssw0rd"); err != nil {
		panic(err)
	}
}
