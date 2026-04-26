package app

import (
	"github.com/controlado/lol-autobuild/internal/config"
)

type ConfigStore interface {
	Load() (config.Config, error)
	Save(new config.Config) error
}
