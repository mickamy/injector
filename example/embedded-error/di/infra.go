package di

import (
	"github.com/mickamy/injector/example/embedded-error/infra"
)

type Infra struct {
	Database *infra.Database `inject:""`
}
