package modular

import (
	"context"
	"time"
)

// Drainable is an optional interface for modules that need a pre-stop drain phase.
// During shutdown, PreStop is called on all Drainable modules (reverse dependency order)
// before Stop is called on Stoppable modules.
type Drainable interface {
	PreStop(ctx context.Context) error
}

const defaultDrainTimeout = 15 * time.Second
