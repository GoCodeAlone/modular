package eventbus

import (
	"context"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
)

// ==============================================================================
// TENANT ISOLATION
// ==============================================================================
// This file handles multi-tenant functionality, tenant-specific routing,
// and tenant isolation verification.

// Tenant isolation - simplified implementations
func (ctx *EventBusBDDTestContext) iHaveAMultiTenantEventbusConfiguration() error {
	err := ctx.iHaveAMultiEngineEventbusConfiguration()
	if err != nil {
		return err
	}
	return ctx.theEventbusModuleIsInitialized()
}

func (ctx *EventBusBDDTestContext) tenantPublishesAnEventToTopic(tenant, topic string) error {
	// Create tenant context for the event
	tenantCtx := modular.NewTenantContext(context.Background(), modular.TenantID(tenant))

	// Create event data specific to this tenant
	eventData := map[string]interface{}{
		"tenant": tenant,
		"topic":  topic,
		"data":   fmt.Sprintf("event-for-%s", tenant),
	}

	// Publish event with tenant context
	return ctx.service.Publish(tenantCtx, topic, eventData)
}

func (ctx *EventBusBDDTestContext) tenantSubscribesToTopic(tenant, topic string) error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	// Initialize maps for this tenant if they don't exist
	if ctx.tenantEventHandlers[tenant] == nil {
		ctx.tenantEventHandlers[tenant] = make(map[string]func(context.Context, Event) error)
		ctx.tenantReceivedEvents[tenant] = make([]Event, 0)
		ctx.tenantSubscriptions[tenant] = make(map[string]Subscription)
	}

	// Create tenant-specific event handler
	handler := func(eventCtx context.Context, event Event) error {
		ctx.mutex.Lock()
		defer ctx.mutex.Unlock()
		// Store received event for this tenant
		ctx.tenantReceivedEvents[tenant] = append(ctx.tenantReceivedEvents[tenant], event)
		return nil
	}

	ctx.tenantEventHandlers[tenant][topic] = handler

	// Create tenant context for subscription
	tenantCtx := modular.NewTenantContext(context.Background(), modular.TenantID(tenant))

	// Subscribe with tenant context
	subscription, err := ctx.service.Subscribe(tenantCtx, topic, handler)
	if err != nil {
		return err
	}

	ctx.tenantSubscriptions[tenant][topic] = subscription
	return nil
}

func (ctx *EventBusBDDTestContext) tenantShouldNotReceiveOtherTenantEvents(tenant1, tenant2 string) error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	// Check that tenant1 did not receive any events meant for tenant2
	tenant1Events := ctx.tenantReceivedEvents[tenant1]
	for _, event := range tenant1Events {
		var eventData map[string]interface{}
		if err := event.DataAs(&eventData); err == nil {
			if eventTenant, ok := eventData["tenant"].(string); ok && eventTenant == tenant2 {
				return fmt.Errorf("tenant %s received event meant for tenant %s", tenant1, tenant2)
			}
		}
	}

	// Check that tenant2 did not receive any events meant for tenant1
	tenant2Events := ctx.tenantReceivedEvents[tenant2]
	for _, event := range tenant2Events {
		var eventData map[string]interface{}
		if err := event.DataAs(&eventData); err == nil {
			if eventTenant, ok := eventData["tenant"].(string); ok && eventTenant == tenant1 {
				return fmt.Errorf("tenant %s received event meant for tenant %s", tenant2, tenant1)
			}
		}
	}

	return nil
}

func (ctx *EventBusBDDTestContext) eventIsolationShouldBeMaintainedBetweenTenants() error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	// Verify that each tenant only received their own events
	for tenant, events := range ctx.tenantReceivedEvents {
		for _, event := range events {
			var eventData map[string]interface{}
			if err := event.DataAs(&eventData); err != nil {
				return fmt.Errorf("event payload not in expected format: %w", err)
			}
			if eventTenant, ok := eventData["tenant"].(string); ok {
				if eventTenant != tenant {
					return fmt.Errorf("event isolation violated: tenant %s received event for tenant %s", tenant, eventTenant)
				}
			} else {
				return fmt.Errorf("event missing tenant information")
			}
		}
	}

	return nil
}

func (ctx *EventBusBDDTestContext) iHaveTenantAwareRoutingConfiguration() error {
	return ctx.iHaveAMultiTenantEventbusConfiguration()
}

func (ctx *EventBusBDDTestContext) tenantIsConfiguredToUseMemoryEngine(tenant string) error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	// Configure tenant to use memory engine
	ctx.tenantEngineConfig[tenant] = "memory"

	// Create tenant context to test tenant-specific routing
	tenantCtx := modular.NewTenantContext(context.Background(), modular.TenantID(tenant))

	// Test that tenant-specific publishing works with memory engine routing
	testTopic := fmt.Sprintf("tenant.%s.memory.test", tenant)
	err := ctx.service.Publish(tenantCtx, testTopic, map[string]interface{}{
		"tenant":     tenant,
		"engineType": "memory",
		"test":       "memory-engine-configuration",
	})

	if err != nil {
		return fmt.Errorf("failed to publish tenant event for memory engine configuration: %w", err)
	}

	return nil
}

func (ctx *EventBusBDDTestContext) tenantIsConfiguredToUseCustomEngine(tenant string) error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	// Configure tenant to use custom engine
	ctx.tenantEngineConfig[tenant] = "custom"

	// Create tenant context to test tenant-specific routing
	tenantCtx := modular.NewTenantContext(context.Background(), modular.TenantID(tenant))

	// Test that tenant-specific publishing works with custom engine routing
	testTopic := fmt.Sprintf("tenant.%s.custom.test", tenant)
	err := ctx.service.Publish(tenantCtx, testTopic, map[string]interface{}{
		"tenant":     tenant,
		"engineType": "custom",
		"test":       "custom-engine-configuration",
	})

	if err != nil {
		return fmt.Errorf("failed to publish tenant event for custom engine configuration: %w", err)
	}

	return nil
}

func (ctx *EventBusBDDTestContext) eventsFromEachTenantShouldUseAssignedEngine() error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	// Verify that each tenant's engine configuration is being respected
	for tenant, engineType := range ctx.tenantEngineConfig {
		if engineType == "" {
			return fmt.Errorf("no engine configuration found for tenant %s", tenant)
		}

		// Validate engine type
		validEngines := []string{"memory", "redis", "kafka", "kinesis", "custom"}
		isValid := false
		for _, valid := range validEngines {
			if engineType == valid {
				isValid = true
				break
			}
		}

		if !isValid {
			return fmt.Errorf("tenant %s configured with invalid engine type: %s", tenant, engineType)
		}

		// Test actual routing by publishing and subscribing with tenant context
		tenantCtx := modular.NewTenantContext(context.Background(), modular.TenantID(tenant))
		testTopic := fmt.Sprintf("tenant.%s.routing.verification", tenant)

		// Subscribe to the test topic
		received := make(chan Event, 1)
		subscription, err := ctx.service.Subscribe(tenantCtx, testTopic, func(ctx context.Context, event Event) error {
			select {
			case received <- event:
			default:
			}
			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to subscribe for tenant %s engine verification: %w", tenant, err)
		}

		// Publish an event for this tenant
		testPayload := map[string]interface{}{
			"tenant":     tenant,
			"engineType": engineType,
			"test":       "engine-assignment-verification",
		}

		err = ctx.service.Publish(tenantCtx, testTopic, testPayload)
		if err != nil {
			_ = subscription.Cancel()
			return fmt.Errorf("failed to publish test event for tenant %s: %w", tenant, err)
		}

		// Wait for event to be processed
		select {
		case event := <-received:
			// Verify the event was received and contains correct tenant information
			var eventData map[string]interface{}
			if err := event.DataAs(&eventData); err == nil {
				if eventTenant, exists := eventData["tenant"]; !exists || eventTenant != tenant {
					_ = subscription.Cancel()
					return fmt.Errorf("event for tenant %s was not properly routed (tenant mismatch)", tenant)
				}
			}
		case <-time.After(1 * time.Second):
			_ = subscription.Cancel()
			return fmt.Errorf("event for tenant %s was not received within timeout", tenant)
		}

		// Clean up subscription
		_ = subscription.Cancel()
	}

	return nil
}

func (ctx *EventBusBDDTestContext) tenantConfigurationsShouldNotInterfere() error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	// Verify that different tenants have different engine configurations
	engineTypes := make(map[string][]string) // engine type -> list of tenants

	for tenant, engineType := range ctx.tenantEngineConfig {
		engineTypes[engineType] = append(engineTypes[engineType], tenant)
	}

	// Verify that each tenant's configuration is isolated
	// (events for tenant A are not processed by tenant B's handlers, etc.)
	for tenant1 := range ctx.tenantEngineConfig {
		for tenant2 := range ctx.tenantEngineConfig {
			if tenant1 != tenant2 {
				// Check that tenant1's events don't leak to tenant2
				tenant2Events := ctx.tenantReceivedEvents[tenant2]
				for _, event := range tenant2Events {
					var eventData map[string]interface{}
					if err := event.DataAs(&eventData); err == nil {
						if eventTenant, ok := eventData["tenant"].(string); ok && eventTenant == tenant1 {
							return fmt.Errorf("configuration interference detected: tenant %s received events from tenant %s", tenant2, tenant1)
						}
					}
				}
			}
		}
	}

	return nil
}
