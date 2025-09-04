package modular

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cucumber/godog"
)

// Static errors for logger decorator BDD tests
var (
	errLoggerNotSet           = errors.New("logger not set")
	errBaseLoggerNotSet       = errors.New("base logger not set")
	errPrimaryLoggerNotSet    = errors.New("primary logger not set")
	errSecondaryLoggerNotSet  = errors.New("secondary logger not set")
	errDecoratedLoggerNotSet  = errors.New("decorated logger not set")
	errNoMessagesLogged       = errors.New("no messages logged")
	errUnexpectedMessageCount = errors.New("unexpected message count")
	errMessageNotFound        = errors.New("message not found")
	errArgNotFound            = errors.New("argument not found")
	errUnexpectedLogLevel     = errors.New("unexpected log level")
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
	filterCriteria   map[string]interface{}
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

func (ctx *LoggerDecoratorBDDTestContext) iApplyAPrefixDecoratorWithPrefix(prefix string) error {
	if ctx.currentLogger == nil {
		return errBaseLoggerNotSet
	}
	ctx.decoratedLogger = NewPrefixLoggerDecorator(ctx.currentLogger, prefix)
	ctx.currentLogger = ctx.decoratedLogger
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iApplyAValueInjectionDecoratorWith(key1, value1 string) error {
	if ctx.currentLogger == nil {
		return errBaseLoggerNotSet
	}
	ctx.decoratedLogger = NewValueInjectionLoggerDecorator(ctx.currentLogger, key1, value1)
	ctx.currentLogger = ctx.decoratedLogger
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iApplyAValueInjectionDecoratorWithTwoKeyValuePairs(key1, value1, key2, value2 string) error {
	if ctx.currentLogger == nil {
		return errBaseLoggerNotSet
	}
	ctx.decoratedLogger = NewValueInjectionLoggerDecorator(ctx.currentLogger, key1, value1, key2, value2)
	ctx.currentLogger = ctx.decoratedLogger
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iApplyADualWriterDecorator() error {
	var primary, secondary Logger

	// Try different combinations of available loggers
	if ctx.primaryLogger != nil && ctx.secondaryLogger != nil {
		primary, secondary = ctx.primaryLogger, ctx.secondaryLogger
	} else if ctx.primaryLogger != nil && ctx.auditLogger != nil {
		primary, secondary = ctx.primaryLogger, ctx.auditLogger
	} else if ctx.baseLogger != nil && ctx.primaryLogger != nil {
		primary, secondary = ctx.baseLogger, ctx.primaryLogger
	} else if ctx.baseLogger != nil && ctx.auditLogger != nil {
		primary, secondary = ctx.baseLogger, ctx.auditLogger
	} else {
		return fmt.Errorf("dual writer decorator requires two loggers, but insufficient loggers are configured")
	}

	ctx.decoratedLogger = NewDualWriterLoggerDecorator(primary, secondary)
	ctx.currentLogger = ctx.decoratedLogger
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iApplyAFilterDecoratorThatBlocksMessagesContaining(blockedText string) error {
	if ctx.currentLogger == nil {
		return errBaseLoggerNotSet
	}
	ctx.decoratedLogger = NewFilterLoggerDecorator(ctx.currentLogger, []string{blockedText}, nil, nil)
	ctx.currentLogger = ctx.decoratedLogger
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iApplyAFilterDecoratorThatBlocksDebugLevelLogs() error {
	if ctx.currentLogger == nil {
		return errBaseLoggerNotSet
	}
	levelFilters := map[string]bool{"debug": false, "info": true, "warn": true, "error": true}
	ctx.decoratedLogger = NewFilterLoggerDecorator(ctx.currentLogger, nil, nil, levelFilters)
	ctx.currentLogger = ctx.decoratedLogger
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iApplyAFilterDecoratorThatBlocksLogsWhereEquals(key, value string) error {
	if ctx.currentLogger == nil {
		return errBaseLoggerNotSet
	}
	keyFilters := map[string]string{key: value}
	ctx.decoratedLogger = NewFilterLoggerDecorator(ctx.currentLogger, nil, keyFilters, nil)
	ctx.currentLogger = ctx.decoratedLogger
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iApplyAFilterDecoratorThatAllowsOnlyLevels(levels string) error {
	if ctx.currentLogger == nil {
		return errBaseLoggerNotSet
	}

	// Parse level names from Gherkin format like '"info" and "error"'
	// Extract quoted level names
	var levelList []string
	parts := strings.Split(levels, `"`)
	for i, part := range parts {
		// Every odd index (1, 3, 5...) contains the quoted content
		if i%2 == 1 && strings.TrimSpace(part) != "" {
			levelList = append(levelList, strings.TrimSpace(part))
		}
	}

	levelFilters := map[string]bool{
		"debug": false,
		"info":  false,
		"warn":  false,
		"error": false,
	}
	for _, level := range levelList {
		levelFilters[level] = true
	}
	ctx.decoratedLogger = NewFilterLoggerDecorator(ctx.currentLogger, nil, nil, levelFilters)
	ctx.currentLogger = ctx.decoratedLogger
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iApplyALevelModifierDecoratorThatMapsTo(fromLevel, toLevel string) error {
	if ctx.currentLogger == nil {
		return errBaseLoggerNotSet
	}
	levelMappings := map[string]string{fromLevel: toLevel}
	ctx.decoratedLogger = NewLevelModifierLoggerDecorator(ctx.currentLogger, levelMappings)
	ctx.currentLogger = ctx.decoratedLogger
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iLogAnInfoMessage(message string) error {
	if ctx.currentLogger == nil {
		return errLoggerNotSet
	}
	ctx.currentLogger.Info(message)
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iLogAnInfoMessageWithArgs(message, key, value string) error {
	if ctx.currentLogger == nil {
		return errLoggerNotSet
	}
	ctx.currentLogger.Info(message, key, value)
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iLogADebugMessage(message string) error {
	if ctx.currentLogger == nil {
		return errLoggerNotSet
	}
	ctx.currentLogger.Debug(message)
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iLogAWarnMessage(message string) error {
	if ctx.currentLogger == nil {
		return errLoggerNotSet
	}
	ctx.currentLogger.Warn(message)
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iLogAnErrorMessage(message string) error {
	if ctx.currentLogger == nil {
		return errLoggerNotSet
	}
	ctx.currentLogger.Error(message)
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iCreateADecoratedLoggerWithPrefix(prefix string) error {
	if ctx.initialLogger == nil {
		return errBaseLoggerNotSet
	}
	ctx.decoratedLogger = NewPrefixLoggerDecorator(ctx.initialLogger, prefix)
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iSetTheDecoratedLoggerOnTheApplication() error {
	if ctx.decoratedLogger == nil {
		return errDecoratedLoggerNotSet
	}
	ctx.app.SetLogger(ctx.decoratedLogger)
	return nil
}

func (ctx *LoggerDecoratorBDDTestContext) iGetTheLoggerServiceFromTheApplication() error {
	var serviceLogger Logger
	err := ctx.app.GetService("logger", &serviceLogger)
	if err != nil {
		return err
	}
	ctx.currentLogger = serviceLogger
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
		filterCriteria: make(map[string]interface{}),
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
