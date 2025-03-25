package modular

import (
	"github.com/golobby/config/v3"
	"github.com/golobby/config/v3/pkg/feeder"
)

var ConfigFeeders = []Feeder{
	EnvFeeder{},
}

// Feeder aliases
type Feeder = config.Feeder
type EnvFeeder = feeder.Env
type DotEnvFeeder = feeder.DotEnv
type YamlFeeder = feeder.Yaml
type JsonFeeder = feeder.Json
type TomlFeeder = feeder.Toml
