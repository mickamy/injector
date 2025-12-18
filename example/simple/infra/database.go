package infra

import (
	"github.com/mickamy/injector/example/simple/config"
)

type Database struct {
	cfg config.DatabaseConfig
}

func NewDatabase(cfg config.DatabaseConfig) *Database {
	return &Database{cfg: cfg}
}
