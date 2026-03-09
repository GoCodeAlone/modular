package eventbus

import (
	"fmt"
	"testing"

	"github.com/GoCodeAlone/modular"
	"github.com/cucumber/godog"
)

// ==============================================================================
// TEST REGISTRATION
// ==============================================================================
// This file contains the main BDD test function and all scenario registrations.

// TestEventBusModuleBDD runs the BDD tests for the EventBus module
func TestEventBusModuleBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			testCtx := &EventBusBDDTestContext{}

			// Background
			ctx.Given(`^I have a modular application with eventbus module configured$`, testCtx.iHaveAModularApplicationWithEventbusModuleConfigured)

			// Steps for module initialization
			ctx.When(`^the eventbus module is initialized$`, testCtx.theEventbusModuleIsInitialized)
			ctx.Then(`^the eventbus service should be available$`, testCtx.theEventbusServiceShouldBeAvailable)
			ctx.Then(`^the service should be configured with default settings$`, testCtx.theServiceShouldBeConfiguredWithDefaultSettings)

			// Steps for basic event handling
			ctx.Given(`^I have an eventbus service available$`, testCtx.iHaveAnEventbusServiceAvailable)
			ctx.When(`^I subscribe to topic "([^"]*)" with a handler$`, testCtx.iSubscribeToTopicWithAHandler)
			ctx.When(`^I publish an event to topic "([^"]*)" with payload "([^"]*)"$`, testCtx.iPublishAnEventToTopicWithPayload)
			ctx.Then(`^the handler should receive the event$`, testCtx.theHandlerShouldReceiveTheEvent)
			ctx.Then(`^the payload should match "([^"]*)"$`, testCtx.thePayloadShouldMatch)

			// Steps for multiple subscribers
			ctx.When(`^I subscribe to topic "([^"]*)" with handler "([^"]*)"$`, testCtx.iSubscribeToTopicWithHandler)
			ctx.Then(`^both handlers should receive the event$`, testCtx.bothHandlersShouldReceiveTheEvent)

			// Steps for wildcard subscriptions
			ctx.Then(`^the handler should receive both events$`, testCtx.theHandlerShouldReceiveBothEvents)
			ctx.Then(`^the payloads should match "([^"]*)" and "([^"]*)"$`, testCtx.thePayloadsShouldMatchAnd)

			// Steps for async processing
			ctx.When(`^I subscribe asynchronously to topic "([^"]*)" with a handler$`, testCtx.iSubscribeAsynchronouslyToTopicWithAHandler)
			ctx.Then(`^the handler should process the event asynchronously$`, testCtx.theHandlerShouldProcessTheEventAsynchronously)
			ctx.Then(`^the publishing should not block$`, testCtx.thePublishingShouldNotBlock)

			// Steps for subscription management
			ctx.When(`^I get the subscription details$`, testCtx.iGetTheSubscriptionDetails)
			ctx.Then(`^the subscription should have a unique ID$`, testCtx.theSubscriptionShouldHaveAUniqueID)
			ctx.Then(`^the subscription topic should be "([^"]*)"$`, testCtx.theSubscriptionTopicShouldBe)
			ctx.Then(`^the subscription should not be async by default$`, testCtx.theSubscriptionShouldNotBeAsyncByDefault)

			// Steps for unsubscribing
			ctx.When(`^I unsubscribe from the topic$`, testCtx.iUnsubscribeFromTheTopic)
			ctx.Then(`^the handler should not receive the event$`, testCtx.theHandlerShouldNotReceiveTheEvent)

			// Steps for active topics
			ctx.Then(`^the active topics should include "([^"]*)" and "([^"]*)"$`, testCtx.theActiveTopicsShouldIncludeAnd)
			ctx.Then(`^the subscriber count for each topic should be (\d+)$`, testCtx.theSubscriberCountForEachTopicShouldBe)

			// Steps for memory engine
			ctx.Given(`^I have an eventbus configuration with memory engine$`, testCtx.iHaveAnEventbusConfigurationWithMemoryEngine)
			ctx.Then(`^the memory engine should be used$`, testCtx.theMemoryEngineShouldBeUsed)
			ctx.Then(`^events should be processed in-memory$`, testCtx.eventsShouldBeProcessedInMemory)

			// Steps for error handling
			ctx.When(`^I subscribe to topic "([^"]*)" with a failing handler$`, testCtx.iSubscribeToTopicWithAFailingHandler)
			ctx.Then(`^the eventbus should handle the error gracefully$`, testCtx.theEventbusShouldHandleTheErrorGracefully)
			ctx.Then(`^the error should be logged appropriately$`, testCtx.theErrorShouldBeLoggedAppropriately)

			// Steps for TTL and retention
			ctx.Given(`^I have an eventbus configuration with event TTL$`, testCtx.iHaveAnEventbusConfigurationWithEventTTL)
			ctx.When(`^events are published with TTL settings$`, testCtx.eventsArePublishedWithTTLSettings)
			ctx.Then(`^old events should be cleaned up automatically$`, testCtx.oldEventsShouldBeCleanedUpAutomatically)
			ctx.Then(`^the retention policy should be respected$`, testCtx.theRetentionPolicyShouldBeRespected)

			// Steps for shutdown
			ctx.Given(`^I have a running eventbus service$`, testCtx.iHaveARunningEventbusService)
			ctx.When(`^the eventbus is stopped$`, testCtx.theEventbusIsStopped)
			ctx.Then(`^all subscriptions should be cancelled$`, testCtx.allSubscriptionsShouldBeCancelled)
			ctx.Then(`^worker pools should be shut down gracefully$`, testCtx.workerPoolsShouldBeShutDownGracefully)
			ctx.Then(`^no memory leaks should occur$`, testCtx.noMemoryLeaksShouldOccur)

			// Event observation steps
			ctx.Given(`^I have an eventbus service with event observation enabled$`, testCtx.iHaveAnEventbusServiceWithEventObservationEnabled)
			ctx.Then(`^a message published event should be emitted$`, testCtx.aMessagePublishedEventShouldBeEmitted)
			ctx.Then(`^a subscription created event should be emitted$`, testCtx.aSubscriptionCreatedEventShouldBeEmitted)
			ctx.Then(`^a subscription removed event should be emitted$`, testCtx.aSubscriptionRemovedEventShouldBeEmitted)
			ctx.Then(`^a message received event should be emitted$`, testCtx.aMessageReceivedEventShouldBeEmitted)
			ctx.Then(`^a topic created event should be emitted$`, testCtx.aTopicCreatedEventShouldBeEmitted)
			ctx.Then(`^a topic deleted event should be emitted$`, testCtx.aTopicDeletedEventShouldBeEmitted)
			ctx.Then(`^a message failed event should be emitted$`, testCtx.aMessageFailedEventShouldBeEmitted)
			ctx.When(`^the eventbus module starts$`, testCtx.theEventbusModuleStarts)
			ctx.Then(`^a config loaded event should be emitted$`, testCtx.aConfigLoadedEventShouldBeEmitted)
			ctx.Then(`^a bus started event should be emitted$`, testCtx.aBusStartedEventShouldBeEmitted)
			ctx.Then(`^a bus stopped event should be emitted$`, testCtx.aBusStoppedEventShouldBeEmitted)

			// Steps for multi-engine scenarios
			ctx.Given(`^I have a multi-engine eventbus configuration with memory and custom engines$`, testCtx.iHaveAMultiEngineEventbusConfiguration)
			ctx.Then(`^both engines should be available$`, testCtx.bothEnginesShouldBeAvailable)
			ctx.Then(`^the engine router should be configured correctly$`, testCtx.theEngineRouterShouldBeConfiguredCorrectly)

			ctx.Given(`^I have a multi-engine eventbus with topic routing configured$`, testCtx.iHaveAMultiEngineEventbusWithTopicRouting)
			ctx.When(`^I publish an event to topic "([^"]*)"$`, testCtx.iPublishAnEventToTopic)
			ctx.Then(`^"([^"]*)" should be routed to the memory engine$`, testCtx.topicShouldBeRoutedToMemoryEngine)
			ctx.Then(`^"([^"]*)" should be routed to the custom engine$`, testCtx.topicShouldBeRoutedToCustomEngine)

			ctx.Given(`^I register a custom engine type "([^"]*)"$`, testCtx.iRegisterACustomEngineType)
			ctx.When(`^I configure eventbus to use the custom engine$`, testCtx.iConfigureEventbusToUseCustomEngine)
			ctx.Then(`^the custom engine should be used for event processing$`, testCtx.theCustomEngineShouldBeUsed)
			ctx.Then(`^events should be handled by the custom implementation$`, testCtx.eventsShouldBeHandledByCustomImplementation)

			ctx.Given(`^I have engines with different configuration settings$`, testCtx.iHaveEnginesWithDifferentConfigurations)
			ctx.When(`^the eventbus is initialized with engine-specific configs$`, testCtx.theEventbusIsInitializedWithEngineConfigs)
			ctx.Then(`^each engine should use its specific configuration$`, testCtx.eachEngineShouldUseItsConfiguration)
			ctx.Then(`^engine behavior should reflect the configured settings$`, testCtx.engineBehaviorShouldReflectSettings)

			ctx.Given(`^I have multiple engines running$`, testCtx.iHaveMultipleEnginesRunning)
			ctx.When(`^I subscribe to topics on different engines$`, testCtx.iSubscribeToTopicsOnDifferentEngines)
			ctx.When(`^I check subscription counts across engines$`, testCtx.iCheckSubscriptionCountsAcrossEngines)
			ctx.Then(`^each engine should report its subscriptions correctly$`, testCtx.eachEngineShouldReportSubscriptionsCorrectly)
			ctx.Then(`^total subscriber counts should aggregate across engines$`, testCtx.totalSubscriberCountsShouldAggregate)

			ctx.Given(`^I have routing rules with wildcards and exact matches$`, testCtx.iHaveRoutingRulesWithWildcardsAndExactMatches)
			ctx.When(`^I publish events with various topic patterns$`, testCtx.iPublishEventsWithVariousTopicPatterns)
			ctx.Then(`^events should be routed according to the first matching rule$`, testCtx.eventsShouldBeRoutedAccordingToFirstMatchingRule)
			ctx.Then(`^fallback routing should work for unmatched topics$`, testCtx.fallbackRoutingShouldWorkForUnmatchedTopics)

			ctx.Given(`^I have multiple engines configured$`, testCtx.iHaveMultipleEnginesConfigured)
			ctx.When(`^one engine encounters an error$`, testCtx.oneEngineEncountersAnError)
			ctx.Then(`^other engines should continue operating normally$`, testCtx.otherEnginesShouldContinueOperatingNormally)
			ctx.Then(`^the error should be isolated to the failing engine$`, testCtx.theErrorShouldBeIsolatedToFailingEngine)

			ctx.Given(`^I have subscriptions across multiple engines$`, testCtx.iHaveSubscriptionsAcrossMultipleEngines)
			ctx.When(`^I query for active topics$`, testCtx.iQueryForActiveTopics)
			ctx.Then(`^all topics from all engines should be returned$`, testCtx.allTopicsFromAllEnginesShouldBeReturned)
			ctx.Then(`^subscriber counts should be aggregated correctly$`, testCtx.subscriberCountsShouldBeAggregatedCorrectly)

			// Event validation (mega-scenario)
			ctx.Then(`^all registered events should be emitted during testing$`, testCtx.allRegisteredEventsShouldBeEmittedDuringTesting)

			// Steps for tenant isolation scenarios
			ctx.Given(`^I have a multi-tenant eventbus configuration$`, testCtx.iHaveAMultiTenantEventbusConfiguration)
			ctx.When(`^tenant "([^"]*)" publishes an event to "([^"]*)"$`, testCtx.tenantPublishesAnEventToTopic)
			ctx.When(`^tenant "([^"]*)" subscribes to "([^"]*)"$`, testCtx.tenantSubscribesToTopic)
			ctx.Then(`^"([^"]*)" should not receive "([^"]*)" events$`, testCtx.tenantShouldNotReceiveOtherTenantEvents)
			ctx.Then(`^event isolation should be maintained between tenants$`, testCtx.eventIsolationShouldBeMaintainedBetweenTenants)

			ctx.Given(`^I have tenant-aware routing configuration$`, testCtx.iHaveTenantAwareRoutingConfiguration)
			ctx.When(`^"([^"]*)" is configured to use memory engine$`, testCtx.tenantIsConfiguredToUseMemoryEngine)
			ctx.When(`^"([^"]*)" is configured to use custom engine$`, testCtx.tenantIsConfiguredToUseCustomEngine)
			ctx.Then(`^events from each tenant should use their assigned engine$`, testCtx.eventsFromEachTenantShouldUseAssignedEngine)
			ctx.Then(`^tenant configurations should not interfere with each other$`, testCtx.tenantConfigurationsShouldNotInterfere)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
			Strict:   true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

// Event validation step - ensures all registered events are emitted during testing
func (ctx *EventBusBDDTestContext) allRegisteredEventsShouldBeEmittedDuringTesting() error {
	// Get all registered event types from the module
	registeredEvents := ctx.module.GetRegisteredEventTypes()

	// Create event validation observer
	validator := modular.NewEventValidationObserver("event-validator", registeredEvents)
	_ = validator // Use validator to avoid unused variable error

	// Check which events were emitted during testing
	emittedEvents := make(map[string]bool)
	for _, event := range ctx.eventObserver.GetEvents() {
		emittedEvents[event.Type()] = true
	}

	// Check for missing events
	var missingEvents []string
	for _, eventType := range registeredEvents {
		if !emittedEvents[eventType] {
			missingEvents = append(missingEvents, eventType)
		}
	}

	if len(missingEvents) > 0 {
		return fmt.Errorf("the following registered events were not emitted during testing: %v", missingEvents)
	}

	return nil
}
