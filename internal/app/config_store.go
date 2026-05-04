package app

import "time"

type RuntimeConfig struct {
	Settings      Settings
	WatchDebounce time.Duration
}

type ConfigStore interface {
	Save(new RuntimeConfig) error
}
