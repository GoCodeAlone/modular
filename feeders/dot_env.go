package feeders

import "github.com/golobby/config/v3/pkg/feeder"

// DotEnvFeeder is a feeder that reads .env files
type DotEnvFeeder = feeder.DotEnv

func NewDotEnvFeeder(filePath string) DotEnvFeeder {
	return DotEnvFeeder{Path: filePath}
}
