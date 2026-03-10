package modular

import (
	"errors"
	"fmt"
	"strings"
)

// Static errors for logger decorator BDD tests
var (
	errLoggerNotSet           = errors.New("logger not set")
	errBaseLoggerNotSet       = errors.New("base logger not set")
	errPrimaryLoggerNotSet    = errors.New("primary logger not set")
	errSecondaryLoggerNotSet  = errors.New("secondary logger not set")
	errDecoratedLoggerNotSet  = errors.New("decorated logger not set")
	errNoMessagesLogged       = errors.New("no messages logged")
	_ = errors.New("unexpected message count")
	_        = errors.New("message not found")
	_            = errors.New("argument not found")
	_     = errors.New("unexpected log level")
	errServiceLoggerMismatch  = errors.New("service logger mismatch")
)

// LoggerDecoratorBDDTestContext holds the test context for logger decorator BDD scenarios
type LoggerDecoratorBDDTestContext struct {
	app              Application
	baseLogger       *TestLogger
	primaryLogger    *TestLogger
	secondaryLogger  *TestLogger
	auditLogger      *TestLogger
	decoratedLogger  Logger
	initialLogger    *TestLogger
	currentLogger    Logger
	expectedMessages []string
	expectedArgs     map[string]string
	filterCriteria   map[string]any
	levelMappings    map[string]string
	messageCount     int
	expectedLevels   []string
}

// Step definitions for logger decorator BDD tests

func (ctx *LoggerDecoratorBDDTestContext) iHaveANewModularApplication() error {
	ctx.baseLogger = NewTestLogger()
	ctx.app = NewStdApplication(NewStdConfigProvider(&struct{}{}), ctx.baseLogger)
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iHaveATestLoggerConfigured() error {
	if ctx.baseLogger == nil {
		ctx.baseLogger = NewTestLogger()
	}
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iHaveABaseLogger() error {
	// Don't overwrite existing baseLogger if we already have one for the application
	if ctx.baseLogger == nil {
		ctx.baseLogger = NewTestLogger()
	}
	ctx.currentLogger = ctx.baseLogger
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iHaveAPrimaryTestLogger() error {
	ctx.primaryLogger = NewTestLogger()
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iHaveASecondaryTestLogger() error {
	ctx.secondaryLogger = NewTestLogger()
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iHaveAnAuditTestLogger() error {
	ctx.auditLogger = NewTestLogger()
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iHaveAnInitialTestLoggerInTheApplication() error {
	ctx.initialLogger = NewTestLogger()
	ctx.app = NewStdApplication(NewStdConfigProvider(&struct{}{}), ctx.initialLogger)
	return nil
}

// findActiveLogger returns the first logger that has entries, or nil if none found
func (ctx *LoggerDecoratorBDDTestContext) findActiveLogger() *TestLogger {
	if ctx.baseLogger != nil && len(ctx.baseLogger.GetEntries()) > 0 {
		return ctx.baseLogger
	}
	if ctx.initialLogger != nil && len(ctx.initialLogger.GetEntries()) > 0 {
		return ctx.initialLogger
	}
	if ctx.primaryLogger != nil && len(ctx.primaryLogger.GetEntries()) > 0 {
		return ctx.primaryLogger
	}
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) theLoggedMessageShouldContain(expectedContent string) error {
	targetLogger := ctx.findActiveLogger()
	if targetLogger == nil {
		return errNoMessagesLogged
	}

	entries := targetLogger.GetEntries()
	if len(entries) == 0 {
		return errNoMessagesLogged
	}

	lastEntry := entries[len(entries)-1]
	if !strings.Contains(lastEntry.Message, expectedContent) {
		return fmt.Errorf("expected message to contain '%s', but got '%s'", expectedContent, lastEntry.Message)
	}
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) theLoggedArgsShouldContain(key, expectedValue string) error {
	targetLogger := ctx.findActiveLogger()
	if targetLogger == nil {
		return errNoMessagesLogged
	}

	entries := targetLogger.GetEntries()
	if len(entries) == 0 {
		return errNoMessagesLogged
	}

	lastEntry := entries[len(entries)-1]
	args := argsToMap(lastEntry.Args)

	actualValue, exists := args[key]
	if !exists {
		return fmt.Errorf("expected arg '%s' not found in logged args: %v", key, args)
	}

	if fmt.Sprintf("%v", actualValue) != expectedValue {
		return fmt.Errorf("expected arg '%s' to be '%s', but got '%v'", key, expectedValue, actualValue)
	}
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) theLoggedMessageShouldBe(expectedMessage string) error {
	// Find the appropriate logger to check - could be base, initial, or primary
	var targetLogger *TestLogger

	if ctx.baseLogger != nil && len(ctx.baseLogger.GetEntries()) > 0 {
		targetLogger = ctx.baseLogger
	} else if ctx.initialLogger != nil && len(ctx.initialLogger.GetEntries()) > 0 {
		targetLogger = ctx.initialLogger
	} else if ctx.primaryLogger != nil && len(ctx.primaryLogger.GetEntries()) > 0 {
		targetLogger = ctx.primaryLogger
	} else {
		return errNoMessagesLogged
	}

	entries := targetLogger.GetEntries()
	if len(entries) == 0 {
		return errNoMessagesLogged
	}

	lastEntry := entries[len(entries)-1]
	if lastEntry.Message != expectedMessage {
		return fmt.Errorf("expected message to be '%s', but got '%s'", expectedMessage, lastEntry.Message)
	}
	return nil
}
