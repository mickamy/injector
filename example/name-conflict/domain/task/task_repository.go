package task

import (
	"errors"
	"log"

	"github.com/mickamy/injector/example/name-conflict/infra"
)

type Repository interface {
	Create(task Task) error
}

type repository struct {
	database *infra.Database
}

func (r *repository) Create(task Task) error {
	if r.database == nil {
		return errors.New("database is not initialized")
	}
	log.Printf("insert task %s with user_id %s\n", task.Title, task.UserID)
	return nil
}

func NewRepository(database *infra.Database) Repository {
	return &repository{database: database}
}
