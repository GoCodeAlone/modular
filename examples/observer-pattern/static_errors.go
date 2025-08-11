package main

import "errors"

var (
	errAuditModuleDoesNotEmitEvents         = errors.New("audit module does not emit events")
	errApplicationDoesNotSupportCloudEvents = errors.New("application does not support CloudEvents")
	errInvalidEnvironment                   = errors.New("environment must be one of [dev, test, prod, demo]")
	errNotificationModuleDoesNotEmitEvents  = errors.New("notification module does not emit events")
	errNoSubjectAvailableForEventEmission   = errors.New("no subject available for event emission")
)
