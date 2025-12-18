package service

import (
	"errors"
	"log"

	"github.com/mickamy/injector/example/with-error/infra"
)

type User interface {
	Register(name string, password string) error
}

type user struct {
	database *infra.Database
}

func (u *user) Register(name string, password string) error {
	if u.database == nil {
		return errors.New("database is not initialized")
	}
	log.Printf("insert user %s with password %s\n", name, password)
	return nil
}

func NewUser(database *infra.Database) User {
	return &user{database: database}
}
