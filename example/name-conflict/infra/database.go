package infra

import (
	"github.com/mickamy/injector/example/name-conflict/config"
)

type Database struct {
	cfg config.DatabaseConfig
}

func NewDatabase(cfg config.DatabaseConfig) (*Database, error) {
	return &Database{cfg: cfg}, nil
}
