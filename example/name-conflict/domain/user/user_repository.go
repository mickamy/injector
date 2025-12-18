package user

import (
	"errors"
	"log"

	"github.com/mickamy/injector/example/name-conflict/infra"
)

type Repository interface {
	Create(user User) error
	Get(userID string) (User, error)
}

type repository struct {
	database *infra.Database
}

func (r *repository) Create(user User) error {
	if r.database == nil {
		return errors.New("database is not initialized")
	}
	log.Printf("insert user %s with password %s\n", user.Name, user.Password)
	return nil
}

func (r *repository) Get(userID string) (User, error) {
	if r.database == nil {
		errors.New("database is not initialized")
	}
	return User{
		ID:       userID,
		Name:     "Alice",
		Password: "P@ssw0rd",
	}, nil
}

func NewRepository(database *infra.Database) Repository {
	return &repository{database: database}
}
