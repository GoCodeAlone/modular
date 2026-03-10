package modular

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cucumber/godog"
)

// Assertion steps for logger decorator BDD tests

func (ctx *LoggerDecoratorBDDTestContext) bothThePrimaryAndSecondaryLoggersShouldReceiveTheMessage() error {
	if ctx.primaryLogger == nil {
		return errPrimaryLoggerNotSet
	}
	if ctx.secondaryLogger == nil {
		return errSecondaryLoggerNotSet
	}

	primaryEntries := ctx.primaryLogger.GetEntries()
	secondaryEntries := ctx.secondaryLogger.GetEntries()

	if len(primaryEntries) == 0 || len(secondaryEntries) == 0 {
		return fmt.Errorf("both loggers should have received messages, primary: %d, secondary: %d",
			len(primaryEntries), len(secondaryEntries))
	}
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) theBaseLoggerShouldHaveReceivedMessages(expectedCount int) error {
	// Find the appropriate logger to check - could be base, initial, or primary
	var targetLogger *TestLogger

	if ctx.baseLogger != nil {
		targetLogger = ctx.baseLogger
	} else if ctx.initialLogger != nil {
		targetLogger = ctx.initialLogger
	} else if ctx.primaryLogger != nil {
		targetLogger = ctx.primaryLogger
	} else {
		return errBaseLoggerNotSet
	}

	entries := targetLogger.GetEntries()
	if len(entries) != expectedCount {
		return fmt.Errorf("expected %d messages, but got %d", expectedCount, len(entries))
	}
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) bothThePrimaryAndAuditLoggersShouldHaveReceivedMessages(expectedCount int) error {
	// Check which loggers we actually have
	var logger1, logger2 *TestLogger

	if ctx.primaryLogger != nil && ctx.auditLogger != nil {
		logger1, logger2 = ctx.primaryLogger, ctx.auditLogger
	} else if ctx.primaryLogger != nil && ctx.secondaryLogger != nil {
		logger1, logger2 = ctx.primaryLogger, ctx.secondaryLogger
	} else {
		return errPrimaryLoggerNotSet
	}

	entries1 := logger1.GetEntries()
	entries2 := logger2.GetEntries()

	if len(entries1) != expectedCount {
		return fmt.Errorf("expected first logger to receive %d messages, but got %d", expectedCount, len(entries1))
	}
	if len(entries2) != expectedCount {
		return fmt.Errorf("expected second logger to receive %d messages, but got %d", expectedCount, len(entries2))
	}
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) theLoggerServiceShouldBeTheDecoratedLogger() error {
	if ctx.currentLogger == nil {
		return errLoggerNotSet
	}
	if ctx.decoratedLogger == nil {
		return errDecoratedLoggerNotSet
	}

	// Verify that the service logger and the decorated logger are the same instance
	if ctx.currentLogger != ctx.decoratedLogger {
		return errServiceLoggerMismatch
	}

	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) theFirstMessageShouldHaveLevel(expectedLevel string) error {
	// Find the appropriate logger to check - could be base, initial, or primary
	var targetLogger *TestLogger

	if ctx.baseLogger != nil {
		targetLogger = ctx.baseLogger
	} else if ctx.initialLogger != nil {
		targetLogger = ctx.initialLogger
	} else if ctx.primaryLogger != nil {
		targetLogger = ctx.primaryLogger
	} else {
		return errBaseLoggerNotSet
	}

	entries := targetLogger.GetEntries()
	if len(entries) == 0 {
		return errNoMessagesLogged
	}

	firstEntry := entries[0]
	if firstEntry.Level != expectedLevel {
		return fmt.Errorf("expected first message level to be '%s', but got '%s'", expectedLevel, firstEntry.Level)
	}
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) theSecondMessageShouldHaveLevel(expectedLevel string) error {
	// Find the appropriate logger to check - could be base, initial, or primary
	var targetLogger *TestLogger

	if ctx.baseLogger != nil {
		targetLogger = ctx.baseLogger
	} else if ctx.initialLogger != nil {
		targetLogger = ctx.initialLogger
	} else if ctx.primaryLogger != nil {
		targetLogger = ctx.primaryLogger
	} else {
		return errBaseLoggerNotSet
	}

	entries := targetLogger.GetEntries()
	if len(entries) < 2 {
		return fmt.Errorf("expected at least 2 messages, but got %d", len(entries))
	}

	secondEntry := entries[1]
	if secondEntry.Level != expectedLevel {
		return fmt.Errorf("expected second message level to be '%s', but got '%s'", expectedLevel, secondEntry.Level)
	}
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) theMessagesShouldHaveLevels(expectedLevels string) error {
	// Find the appropriate logger to check - could be base, initial, or primary
	var targetLogger *TestLogger

	if ctx.baseLogger != nil {
		targetLogger = ctx.baseLogger
	} else if ctx.initialLogger != nil {
		targetLogger = ctx.initialLogger
	} else if ctx.primaryLogger != nil {
		targetLogger = ctx.primaryLogger
	} else {
		return errBaseLoggerNotSet
	}

	levelList := strings.Split(strings.ReplaceAll(expectedLevels, `"`, ""), ", ")
	entries := targetLogger.GetEntries()

	if len(entries) != len(levelList) {
		return fmt.Errorf("expected %d messages, but got %d", len(levelList), len(entries))
	}

	for i, expectedLevel := range levelList {
		if entries[i].Level != expectedLevel {
			return fmt.Errorf("expected message %d to have level '%s', but got '%s'", i+1, expectedLevel, entries[i].Level)
		}
	}
	return nil
}

// InitializeLoggerDecoratorScenario initializes the BDD test context for logger decorator scenarios
func InitializeLoggerDecoratorScenario(ctx *godog.ScenarioContext) {
	testCtx := &LoggerDecoratorBDDTestContext{
		expectedArgs:   make(map[string]string),
		filterCriteria: make(map[string]any),
		levelMappings:  make(map[string]string),
	}

	// Background steps
	ctx.Step(`^I have a new modular application$`, testCtx.iHaveANewModularApplication)
	ctx.Step(`^I have a test logger configured$`, testCtx.iHaveATestLoggerConfigured)

	// Setup steps
	ctx.Step(`^I have a base logger$`, testCtx.iHaveABaseLogger)
	ctx.Step(`^I have a primary test logger$`, testCtx.iHaveAPrimaryTestLogger)
	ctx.Step(`^I have a secondary test logger$`, testCtx.iHaveASecondaryTestLogger)
	ctx.Step(`^I have an audit test logger$`, testCtx.iHaveAnAuditTestLogger)
	ctx.Step(`^I have an initial test logger in the application$`, testCtx.iHaveAnInitialTestLoggerInTheApplication)

	// Decorator application steps
	ctx.Step(`^I apply a prefix decorator with prefix "([^"]*)"$`, testCtx.iApplyAPrefixDecoratorWithPrefix)
	ctx.Step(`^I apply a value injection decorator with "([^"]*)", "([^"]*)"$`, testCtx.iApplyAValueInjectionDecoratorWith)
	ctx.Step(`^I apply a value injection decorator with "([^"]*)", "([^"]*)" and "([^"]*)", "([^"]*)"$`, testCtx.iApplyAValueInjectionDecoratorWithTwoKeyValuePairs)
	ctx.Step(`^I apply a dual writer decorator$`, testCtx.iApplyADualWriterDecorator)
	ctx.Step(`^I apply a filter decorator that blocks messages containing "([^"]*)"$`, testCtx.iApplyAFilterDecoratorThatBlocksMessagesContaining)
	ctx.Step(`^I apply a filter decorator that blocks debug level logs$`, testCtx.iApplyAFilterDecoratorThatBlocksDebugLevelLogs)
	ctx.Step(`^I apply a filter decorator that blocks logs where "([^"]*)" equals "([^"]*)"$`, testCtx.iApplyAFilterDecoratorThatBlocksLogsWhereEquals)
	ctx.Step(`^I apply a filter decorator that allows only (.+) levels$`, testCtx.iApplyAFilterDecoratorThatAllowsOnlyLevels)
	ctx.Step(`^I apply a level modifier decorator that maps "([^"]*)" to "([^"]*)"$`, testCtx.iApplyALevelModifierDecoratorThatMapsTo)

	// Logging action steps
	ctx.Step(`^I log an info message "([^"]*)"$`, testCtx.iLogAnInfoMessage)
	ctx.Step(`^I log an info message "([^"]*)" with args "([^"]*)", "([^"]*)"$`, testCtx.iLogAnInfoMessageWithArgs)
	ctx.Step(`^I log a debug message "([^"]*)"$`, testCtx.iLogADebugMessage)
	ctx.Step(`^I log a warn message "([^"]*)"$`, testCtx.iLogAWarnMessage)
	ctx.Step(`^I log an error message "([^"]*)"$`, testCtx.iLogAnErrorMessage)

	// SetLogger scenario steps
	ctx.Step(`^I create a decorated logger with prefix "([^"]*)"$`, testCtx.iCreateADecoratedLoggerWithPrefix)
	ctx.Step(`^I set the decorated logger on the application$`, testCtx.iSetTheDecoratedLoggerOnTheApplication)
	ctx.Step(`^I get the logger service from the application$`, testCtx.iGetTheLoggerServiceFromTheApplication)

	// Assertion steps
	ctx.Step(`^the logged message should contain "([^"]*)"$`, testCtx.theLoggedMessageShouldContain)
	ctx.Step(`^the logged args should contain "([^"]*)": "([^"]*)"$`, testCtx.theLoggedArgsShouldContain)
	ctx.Step(`^both the primary and secondary loggers should receive the message$`, testCtx.bothThePrimaryAndSecondaryLoggersShouldReceiveTheMessage)
	ctx.Step(`^the base logger should have received (\d+) messages?$`, testCtx.theBaseLoggerShouldHaveReceivedMessages)
	ctx.Step(`^the logged message should be "([^"]*)"$`, testCtx.theLoggedMessageShouldBe)
	ctx.Step(`^both the primary and audit loggers should have received (\d+) messages?$`, testCtx.bothThePrimaryAndAuditLoggersShouldHaveReceivedMessages)
	ctx.Step(`^the logger service should be the decorated logger$`, testCtx.theLoggerServiceShouldBeTheDecoratedLogger)
	ctx.Step(`^the first message should have level "([^"]*)"$`, testCtx.theFirstMessageShouldHaveLevel)
	ctx.Step(`^the second message should have level "([^"]*)"$`, testCtx.theSecondMessageShouldHaveLevel)
	ctx.Step(`^the messages should have levels (.+)$`, testCtx.theMessagesShouldHaveLevels)
}

// TestLoggerDecorator runs the BDD tests for logger decorator functionality
func TestLoggerDecorator(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeLoggerDecoratorScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/logger_decorator.feature"},
			TestingT: t,
			Strict:   true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
