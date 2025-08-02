package eventbus

import "errors"

var (
	// Event bus state errors
	ErrEventBusNotStarted       = errors.New("event bus not started")
	ErrEventBusShutdownTimedOut = errors.New("event bus shutdown timed out")

	// Subscription errors
	ErrEventHandlerNil         = errors.New("event handler cannot be nil")
	ErrInvalidSubscriptionType = errors.New("invalid subscription type")
)
