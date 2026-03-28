package eventlogger

import (
	"testing"

	"github.com/cucumber/godog"
)

// TestEventLoggerModuleBDD runs the BDD tests for the EventLogger module
func TestEventLoggerModuleBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(s *godog.ScenarioContext) {
			ctx := &EventLoggerBDDTestContext{}

			// Background
			s.Given(`^I have a modular application with event logger module configured$`, ctx.iHaveAModularApplicationWithEventLoggerModuleConfigured)

			// Core module functionality
			s.Then(`^the event logger service should be available$`, ctx.theEventLoggerServiceShouldBeAvailable)
			s.Then(`^the module should register as an observer$`, ctx.theModuleShouldRegisterAsAnObserver)

			// Console output
			s.Given(`^I have an event logger with console output configured$`, ctx.iHaveAnEventLoggerWithConsoleOutputConfigured)
			s.When(`^I emit a test event with type "([^"]*)" and data "([^"]*)"$`, ctx.iEmitATestEventWithTypeAndData)
			s.Then(`^the event should be logged to console output$`, ctx.theEventShouldBeLoggedToConsoleOutput)
			s.Then(`^the log entry should contain the event type and data$`, ctx.theLogEntryShouldContainTheEventTypeAndData)

			// File output
			s.Given(`^I have an event logger with file output configured$`, ctx.iHaveAnEventLoggerWithFileOutputConfigured)
			s.When(`^I emit multiple events with different types$`, ctx.iEmitMultipleEventsWithDifferentTypes)
			s.Then(`^all events should be logged to the file$`, ctx.allEventsShouldBeLoggedToTheFile)
			s.Then(`^the file should contain structured log entries$`, ctx.theFileShouldContainStructuredLogEntries)

			// Event filtering
			s.Given(`^I have an event logger with event type filters configured$`, ctx.iHaveAnEventLoggerWithEventTypeFiltersConfigured)
			s.When(`^I emit events with different types$`, ctx.iEmitEventsWithDifferentTypes)
			s.Then(`^only filtered event types should be logged$`, ctx.onlyFilteredEventTypesShouldBeLogged)
			s.Then(`^non-matching events should be ignored$`, ctx.nonMatchingEventsShouldBeIgnored)

			// Log level filtering
			s.Given(`^I have an event logger with INFO log level configured$`, ctx.iHaveAnEventLoggerWithINFOLogLevelConfigured)
			s.When(`^I emit events with different log levels$`, ctx.iEmitEventsWithDifferentLogLevels)
			s.Then(`^only INFO and higher level events should be logged$`, ctx.onlyINFOAndHigherLevelEventsShouldBeLogged)
			s.Then(`^DEBUG events should be filtered out$`, ctx.dEBUGEventsShouldBeFilteredOut)

			// Buffer management
			s.Given(`^I have an event logger with buffer size configured$`, ctx.iHaveAnEventLoggerWithBufferSizeConfigured)
			s.When(`^I emit more events than the buffer can hold$`, ctx.iEmitMoreEventsThanTheBufferCanHold)
			s.Then(`^older events should be dropped$`, ctx.olderEventsShouldBeDropped)
			s.Then(`^buffer overflow should be handled gracefully$`, ctx.bufferOverflowShouldBeHandledGracefully)

			// Multiple targets
			s.Given(`^I have an event logger with multiple output targets configured$`, ctx.iHaveAnEventLoggerWithMultipleOutputTargetsConfigured)
			s.When(`^I emit an event$`, ctx.iEmitAnEvent)
			s.Then(`^the event should be logged to all configured targets$`, ctx.theEventShouldBeLoggedToAllConfiguredTargets)
			s.Then(`^each target should receive the same event data$`, ctx.eachTargetShouldReceiveTheSameEventData)

			// Metadata
			s.Given(`^I have an event logger with metadata inclusion enabled$`, ctx.iHaveAnEventLoggerWithMetadataInclusionEnabled)
			s.When(`^I emit an event with metadata$`, ctx.iEmitAnEventWithMetadata)
			s.Then(`^the logged event should include the metadata$`, ctx.theLoggedEventShouldIncludeTheMetadata)
			s.Then(`^CloudEvent fields should be preserved$`, ctx.cloudEventFieldsShouldBePreserved)

			// Shutdown
			s.Given(`^I have an event logger with pending events$`, ctx.iHaveAnEventLoggerWithPendingEvents)
			s.When(`^the module is stopped$`, ctx.theModuleIsStopped)
			s.Then(`^all pending events should be flushed$`, ctx.allPendingEventsShouldBeFlushed)
			s.Then(`^output targets should be closed properly$`, ctx.outputTargetsShouldBeClosedProperly)

			// Error handling
			s.Given(`^I have an event logger with faulty output target$`, ctx.iHaveAnEventLoggerWithFaultyOutputTarget)
			s.When(`^I emit events$`, ctx.iEmitEvents)
			s.Then(`^errors should be handled gracefully$`, ctx.errorsShouldBeHandledGracefully)
			s.Then(`^other output targets should continue working$`, ctx.otherOutputTargetsShouldContinueWorking)

			// Event observation step registrations
			s.Given(`^I have an event logger with event observation enabled$`, ctx.iHaveAnEventLoggerWithEventObservationEnabled)
			s.When(`^the event logger module starts$`, func() error { return nil }) // Already started in Given step
			s.Then(`^a logger started event should be emitted$`, ctx.aLoggerStartedEventShouldBeEmitted)
			s.Then(`^the event should contain output count and buffer size$`, ctx.theEventShouldContainOutputCountAndBufferSize)
			s.When(`^the event logger module stops$`, ctx.theEventLoggerModuleStops)
			s.Then(`^a logger stopped event should be emitted$`, ctx.aLoggerStoppedEventShouldBeEmitted)

			// Configuration events
			s.When(`^the event logger module is initialized$`, func() error {
				return ctx.theEventLoggerModuleIsInitialized() // Always call regular initialization
			})
			s.Then(`^a config loaded event should be emitted$`, ctx.aConfigLoadedEventShouldBeEmitted)
			s.Then(`^output registered events should be emitted for each target$`, ctx.outputRegisteredEventsShouldBeEmittedForEachTarget)
			s.Then(`^the events should contain configuration details$`, ctx.theEventsShouldContainConfigurationDetails)

			// Processing events
			s.When(`^I emit a test event for processing$`, ctx.iEmitATestEventForProcessing)
			s.Then(`^an event received event should be emitted$`, ctx.anEventReceivedEventShouldBeEmitted)
			s.Then(`^an event processed event should be emitted$`, ctx.anEventProcessedEventShouldBeEmitted)
			s.Then(`^an output success event should be emitted$`, ctx.anOutputSuccessEventShouldBeEmitted)

			// Buffer overflow events
			s.Given(`^I have an event logger with small buffer and event observation enabled$`, ctx.iHaveAnEventLoggerWithSmallBufferAndEventObservationEnabled)
			s.When(`^I rapidly emit more events than the buffer can hold$`, ctx.iRapidlyEmitMoreEventsThanTheBufferCanHold)
			s.Then(`^buffer full events should be emitted$`, ctx.bufferFullEventsShouldBeEmitted)
			s.Then(`^event dropped events should be emitted$`, ctx.eventDroppedEventsShouldBeEmitted)
			s.Then(`^the events should contain drop reasons$`, ctx.theEventsShouldContainDropReasons)

			// Output error events
			s.Given(`^I have an event logger with faulty output target and event observation enabled$`, ctx.iHaveAnEventLoggerWithFaultyOutputTargetAndEventObservationEnabled)
			s.Then(`^an output error event should be emitted$`, ctx.anOutputErrorEventShouldBeEmitted)
			s.Then(`^the error event should contain error details$`, ctx.theErrorEventShouldContainErrorDetails)

			// Queue-until-ready scenarios
			s.Given(`^I have an event logger module configured but not started$`, ctx.iHaveAnEventLoggerModuleConfiguredButNotStarted)
			s.When(`^I emit events before the eventlogger starts$`, ctx.iEmitEventsBeforeTheEventloggerStarts)
			s.Then(`^the events should be queued without errors$`, ctx.theEventsShouldBeQueuedWithoutErrors)
			s.When(`^the eventlogger starts$`, ctx.theEventloggerStarts)
			s.Then(`^all queued events should be processed and logged$`, ctx.allQueuedEventsShouldBeProcessedAndLogged)
			s.Then(`^the events should be processed in order$`, ctx.theEventsShouldBeProcessedInOrder)

			// Queue overflow scenarios
			s.Given(`^I have an event logger module configured with queue overflow testing$`, ctx.iHaveAnEventLoggerModuleConfiguredWithQueueOverflowTesting)
			s.When(`^I emit more events than the queue can hold before start$`, ctx.iEmitMoreEventsThanTheQueueCanHoldBeforeStart)
			s.Then(`^older events should be dropped from the queue$`, ctx.olderEventsShouldBeDroppedFromTheQueue)
			s.Then(`^newer events should be preserved in the queue$`, ctx.newerEventsShouldBePreservedInTheQueue)
			s.Then(`^only the preserved events should be processed$`, ctx.onlyThePreservedEventsShouldBeProcessed)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/eventlogger_module.feature"},
			TestingT: t,
			Strict:   true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
