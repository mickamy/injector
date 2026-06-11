package di

import (
	"github.com/mickamy/injector/example/embed/infra"
)

type Infra struct {
	Database *infra.Database `inject:""`
}
