package modular

import (
	"github.com/GoCodeAlone/modular/feeders"
	"github.com/golobby/config/v3"
)

var ConfigFeeders = []Feeder{
	feeders.EnvFeeder{},
}

// Feeder aliases
type Feeder = config.Feeder

type ComplexFeeder interface {
	Feeder
	FeedKey(string, interface{}) error
}
