package di

import (
	"github.com/mickamy/injector/example/embedded/config"
	"github.com/mickamy/injector/example/embedded/infra"
)

type Infra struct {
	_        config.DatabaseConfig `inject:"provider:config.NewReaderDatabaseConfig"`
	Database *infra.Database       `inject:""`
}
