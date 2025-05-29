package feeders

import "github.com/golobby/config/v3/pkg/feeder"

// EnvFeeder is a feeder that reads environment variables
type EnvFeeder = feeder.Env

// NewEnvFeeder creates a new EnvFeeder that reads from environment variables
func NewEnvFeeder() EnvFeeder {
	return EnvFeeder{}
}
