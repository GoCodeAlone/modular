package feeders

import "github.com/golobby/config/v3/pkg/feeder"

// EnvFeeder is a feeder that reads environment variables
type EnvFeeder = feeder.Env

func NewEnvFeeder() EnvFeeder {
	return EnvFeeder{}
}
