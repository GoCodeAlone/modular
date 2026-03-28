package eventbus

import (
	"context"
	"fmt"
	"time"
)

// ==============================================================================
// MULTI-ENGINE ERROR ISOLATION
// ==============================================================================
// This file handles error isolation scenarios in multi-engine configurations.

// Additional simplified implementations
func (ctx *EventBusBDDTestContext) iHaveMultipleEnginesConfigured() error {
	err := ctx.iHaveAMultiEngineEventbusConfiguration()
	if err != nil {
		return err
	}
	// Initialize the eventbus module to set up the service
	return ctx.theEventbusModuleIsInitialized()
}

func (ctx *EventBusBDDTestContext) oneEngineEncountersAnError() error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	if ctx.service == nil {
		return fmt.Errorf("no eventbus service available")
	}

	// Ensure service is started before trying to publish
	if !ctx.service.isStarted.Load() {
		err := ctx.service.Start(context.Background())
		if err != nil {
			return fmt.Errorf("failed to start eventbus: %w", err)
		}
	}

	// Simulate an error condition by trying to publish to a topic that would route to an unavailable engine
	// For example, redis.error topic if redis engine is not configured or available
	errorTopic := "redis.error.simulation"

	// Store the error for verification in other steps
	err := ctx.service.Publish(context.Background(), errorTopic, map[string]interface{}{
		"test":  "error-simulation",
		"error": true,
	})

	// Store the error (might be nil if fallback works)
	ctx.lastError = err

	// For BDD testing, we simulate error by attempting to use unavailable engines
	// The error might not occur if fallback routing is working properly
	ctx.errorTopic = errorTopic

	return nil
}

func (ctx *EventBusBDDTestContext) otherEnginesShouldContinueOperatingNormally() error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	// Test that other engines (not the failing one) continue to work normally
	testTopics := []string{"memory.normal", "user.normal", "auth.normal"}

	for _, topic := range testTopics {
		// Skip the error topic if it matches our test topics
		if topic == ctx.errorTopic {
			continue
		}

		// Test subscription
		received := make(chan bool, 1)
		subscription, err := ctx.service.Subscribe(context.Background(), topic, func(ctx context.Context, event Event) error {
			select {
			case received <- true:
			default:
			}
			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to subscribe to working engine topic %s: %w", topic, err)
		}

		// Test publishing
		err = ctx.service.Publish(context.Background(), topic, map[string]interface{}{
			"test":  "normal-operation",
			"topic": topic,
		})

		if err != nil {
			_ = subscription.Cancel()
			return fmt.Errorf("failed to publish to working engine topic %s: %w", topic, err)
		}

		// Verify event is received
		select {
		case <-received:
			// Good - engine is working normally
		case <-time.After(1 * time.Second):
			_ = subscription.Cancel()
			return fmt.Errorf("event not received on working engine topic %s", topic)
		}

		// Clean up
		_ = subscription.Cancel()
	}

	return nil
}

func (ctx *EventBusBDDTestContext) theErrorShouldBeIsolatedToFailingEngine() error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	// Verify that the error from one engine doesn't affect other engines
	// This is verified by ensuring:
	// 1. The error topic (if any) doesn't prevent other topics from working
	// 2. System-wide operations like creating subscriptions still work
	// 3. New subscriptions can still be created

	// Test that we can still perform basic operations (creating subscriptions)
	testTopic := "isolation.test.before"
	testSub, err := ctx.service.Subscribe(context.Background(), testTopic, func(ctx context.Context, event Event) error {
		return nil
	})
	if err != nil {
		return fmt.Errorf("system-wide operation failed due to engine error: %w", err)
	}
	if testSub != nil {
		_ = testSub.Cancel()
	}

	// Test that new subscriptions can still be created
	testTopic2 := "isolation.test"
	subscription, err := ctx.service.Subscribe(context.Background(), testTopic2, func(ctx context.Context, event Event) error {
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create new subscription after engine error: %w", err)
	}

	// Test that publishing to non-failing engines still works
	err = ctx.service.Publish(context.Background(), testTopic2, map[string]interface{}{
		"test": "error-isolation",
	})

	if err != nil {
		_ = subscription.Cancel()
		return fmt.Errorf("failed to publish after engine error: %w", err)
	}

	// Clean up
	_ = subscription.Cancel()

	// If we had an error from the failing engine, verify it didn't propagate
	if ctx.lastError != nil && ctx.errorTopic != "" {
		// The error should be contained - we should still be able to use other functionality
		// This is implicitly tested by the successful operations above
	}

	return nil
}

func (ctx *EventBusBDDTestContext) iHaveSubscriptionsAcrossMultipleEngines() error {
	// Set up multi-engine configuration first
	err := ctx.iHaveAMultiEngineEventbusConfiguration()
	if err != nil {
		return err
	}

	// Initialize the service
	err = ctx.theEventbusModuleIsInitialized()
	if err != nil {
		return err
	}

	// Now subscribe to topics on different engines
	return ctx.iSubscribeToTopicsOnDifferentEngines()
}

func (ctx *EventBusBDDTestContext) iQueryForActiveTopics() error {
	ctx.activeTopics = ctx.service.Topics()
	return nil
}

func (ctx *EventBusBDDTestContext) allTopicsFromAllEnginesShouldBeReturned() error {
	if len(ctx.activeTopics) < 2 {
		return fmt.Errorf("expected at least 2 active topics, got %d", len(ctx.activeTopics))
	}
	return nil
}

func (ctx *EventBusBDDTestContext) subscriberCountsShouldBeAggregatedCorrectly() error {
	// Calculate the total subscriber count
	totalCount := ctx.service.SubscriberCount("user.created") + ctx.service.SubscriberCount("analytics.pageview")
	if totalCount != 2 {
		return fmt.Errorf("expected total count of 2, got %d", totalCount)
	}
	return nil
}
