package feeders

import "github.com/golobby/config/v3/pkg/feeder"

// DotEnvFeeder is a feeder that reads .env files
type DotEnvFeeder = feeder.DotEnv

// NewDotEnvFeeder creates a new DotEnvFeeder that reads from the specified .env file
func NewDotEnvFeeder(filePath string) DotEnvFeeder {
	return DotEnvFeeder{Path: filePath}
}
