package di

import (
	"github.com/mickamy/injector/example/embedded-error/config"
	"github.com/mickamy/injector/example/embedded-error/infra"
)

type Infra struct {
	_        config.DatabaseConfig `inject:"provider:config.NewReaderDatabaseConfig"`
	Database *infra.Database       `inject:""`
}
