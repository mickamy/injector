package di

import (
	"github.com/mickamy/injector/example/embedded/infra"
)

type Infra struct {
	Database *infra.Database `inject:""`
}
