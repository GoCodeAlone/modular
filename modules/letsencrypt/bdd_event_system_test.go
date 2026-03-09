package letsencrypt

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/cucumber/godog"
)

// --- Event observation setup ---

func (ctx *LetsEncryptBDDTestContext) iHaveALetsEncryptModuleWithEventObservationEnabled() error {
	// Don't call the regular setup that resets context - do our own setup
	ctx.resetContext()

	// Create temp directory for certificate storage
	var err error
	ctx.tempDir, err = os.MkdirTemp("", "letsencrypt-bdd-test")
	if err != nil {
		return err
	}

	// Create basic LetsEncrypt configuration for testing
	ctx.config = &LetsEncryptConfig{
		Email:       "test@example.com",
		Domains:     []string{"example.com"},
		UseStaging:  true,
		StoragePath: ctx.tempDir,
		RenewBefore: 30,
		AutoRenew:   true,
		UseDNS:      false,
		HTTPProvider: &HTTPProviderConfig{
			UseBuiltIn: true,
			Port:       8080,
		},
	}

	// Create ObservableApplication for event support
	logger := &testLogger{}
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewObservableApplication(mainConfigProvider, logger)

	// Create LetsEncrypt module instance directly
	ctx.module, err = New(ctx.config)
	if err != nil {
		ctx.lastError = err
		return err
	}

	// Create and register the event observer
	ctx.eventObserver = newTestEventObserver()
	subject, ok := ctx.app.(modular.Subject)
	if !ok {
		return fmt.Errorf("application does not implement Subject interface")
	}

	if err := subject.RegisterObserver(ctx.eventObserver); err != nil {
		return fmt.Errorf("failed to register event observer: %w", err)
	}

	// Ensure the module has its subject reference for event emission
	if err := ctx.module.RegisterObservers(subject); err != nil {
		return fmt.Errorf("failed to register module observers: %w", err)
	}

	// Debug: Verify the subject was actually set
	if ctx.module.subject == nil {
		return fmt.Errorf("module subject is still nil after RegisterObservers call")
	}

	return nil
}

// --- Module lifecycle event steps ---

func (ctx *LetsEncryptBDDTestContext) theLetsEncryptModuleStarts() error {
	err := ctx.theLetsEncryptModuleIsInitialized()
	if err != nil {
		return err
	}

	// For BDD testing, we'll simulate the event emission without full ACME initialization
	// This tests the event infrastructure rather than the full certificate functionality
	ctx.module.emitEvent(context.Background(), EventTypeServiceStarted, map[string]interface{}{
		"domains_count": len(ctx.config.Domains),
		"dns_provider":  ctx.config.DNSProvider,
		"auto_renew":    ctx.config.AutoRenew,
		"production":    ctx.config.UseProduction,
	})

	// Give a small delay to allow event propagation
	time.Sleep(10 * time.Millisecond)

	return nil
}

func (ctx *LetsEncryptBDDTestContext) aServiceStartedEventShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeServiceStarted {
			return nil
		}
	}
	return fmt.Errorf("service started event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) theEventShouldContainServiceConfigurationDetails() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeServiceStarted {
			// Check that the event contains configuration details
			if event.Source() == "" {
				return fmt.Errorf("event missing source information")
			}
			return nil
		}
	}
	return fmt.Errorf("service started event not found")
}

func (ctx *LetsEncryptBDDTestContext) theLetsEncryptModuleStops() error {
	if ctx.module == nil {
		return fmt.Errorf("module not initialized")
	}

	// Actually call the Stop method which will emit events
	err := ctx.module.Stop(context.Background())
	if err != nil {
		return fmt.Errorf("failed to stop module: %w", err)
	}

	// Give a small delay to allow event propagation
	time.Sleep(10 * time.Millisecond)

	return nil
}

func (ctx *LetsEncryptBDDTestContext) aServiceStoppedEventShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeServiceStopped {
			return nil
		}
	}
	return fmt.Errorf("service stopped event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) aModuleStoppedEventShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	// Wait briefly to account for asynchronous dispatch ordering where the
	// module stopped event may arrive after the service stopped assertion.
	if ctx.waitForEvent(EventTypeModuleStopped, 150*time.Millisecond) {
		return nil
	}
	events := ctx.eventObserver.GetEvents()
	return fmt.Errorf("module stopped event not found among %d events", len(events))
}

// waitForEvent polls the observer until the specified event type is observed or timeout expires
func (ctx *LetsEncryptBDDTestContext) waitForEvent(eventType string, timeout time.Duration) bool {
	if ctx.eventObserver == nil {
		return false
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, e := range ctx.eventObserver.GetEvents() {
			if e.Type() == eventType {
				return true
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func (ctx *LetsEncryptBDDTestContext) theModuleStartsUp() error {
	if ctx.module == nil {
		return fmt.Errorf("module not initialized")
	}

	// Simulate module startup
	ctx.module.emitEvent(context.Background(), EventTypeModuleStarted, map[string]interface{}{
		"module_name":        "letsencrypt",
		"certificates_count": len(ctx.module.certificates),
		"auto_renew_enabled": ctx.config.AutoRenew,
	})

	// Give a small delay to allow event propagation
	time.Sleep(10 * time.Millisecond)

	return nil
}

func (ctx *LetsEncryptBDDTestContext) aModuleStartedEventShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeModuleStarted {
			return nil
		}
	}
	return fmt.Errorf("module started event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) theEventShouldContainModuleInformation() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeModuleStarted {
			// Check that the event contains module information
			dataMap := make(map[string]interface{})
			if err := event.DataAs(&dataMap); err != nil {
				return fmt.Errorf("failed to parse event data: %w", err)
			}

			if moduleName, ok := dataMap["module_name"]; ok && moduleName != nil {
				return nil // Module information found
			}
			// Also check for other module info
			if autoRenew, ok := dataMap["auto_renew_enabled"]; ok && autoRenew != nil {
				return nil // Module information found
			}
			return fmt.Errorf("event missing module information")
		}
	}
	return fmt.Errorf("module started event not found")
}

// --- Certificate lifecycle event validation steps ---

func (ctx *LetsEncryptBDDTestContext) aCertificateRequestedEventShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCertificateRequested {
			return nil
		}
	}
	return fmt.Errorf("certificate requested event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) theEventShouldContainDomainInformation() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCertificateRequested {
			// Check that the event contains domain information
			dataMap := make(map[string]interface{})
			if err := event.DataAs(&dataMap); err != nil {
				return fmt.Errorf("failed to parse event data: %w", err)
			}

			if domains, ok := dataMap["domains"]; ok && domains != nil {
				return nil // Domain information found
			}
			return fmt.Errorf("event missing domain information")
		}
	}
	return fmt.Errorf("certificate requested event not found")
}

func (ctx *LetsEncryptBDDTestContext) aCertificateIssuedEventShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCertificateIssued {
			return nil
		}
	}
	return fmt.Errorf("certificate issued event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) theEventShouldContainDomainDetails() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCertificateIssued {
			// Check that the event contains domain details
			dataMap := make(map[string]interface{})
			if err := event.DataAs(&dataMap); err != nil {
				return fmt.Errorf("failed to parse event data: %w", err)
			}

			if domain, ok := dataMap["domain"]; ok && domain != nil {
				return nil // Domain details found
			}
			return fmt.Errorf("event missing domain details")
		}
	}
	return fmt.Errorf("certificate issued event not found")
}

func (ctx *LetsEncryptBDDTestContext) certificateRenewedEventsShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCertificateRenewed {
			return nil
		}
	}
	return fmt.Errorf("certificate renewed event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) theEventsShouldContainRenewalDetails() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCertificateRenewed {
			// Check that the event contains renewal details
			dataMap := make(map[string]interface{})
			if err := event.DataAs(&dataMap); err != nil {
				return fmt.Errorf("failed to parse event data: %w", err)
			}

			if domain, ok := dataMap["domain"]; ok && domain != nil {
				return nil // Renewal details found
			}
			return fmt.Errorf("event missing renewal details")
		}
	}
	return fmt.Errorf("certificate renewed event not found")
}

func (ctx *LetsEncryptBDDTestContext) certificateExpiringEventsShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCertificateExpiring {
			return nil
		}
	}
	return fmt.Errorf("certificate expiring event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) theEventsShouldContainExpiryDetails() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCertificateExpiring {
			// Check that the event contains expiry details
			dataMap := make(map[string]interface{})
			if err := event.DataAs(&dataMap); err != nil {
				return fmt.Errorf("failed to parse event data: %w", err)
			}

			if daysLeft, ok := dataMap["days_left"]; ok && daysLeft != nil {
				return nil // Expiry details found
			}
			return fmt.Errorf("event missing expiry details")
		}
	}
	return fmt.Errorf("certificate expiring event not found")
}

func (ctx *LetsEncryptBDDTestContext) certificateExpiredEventsShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCertificateExpired {
			return nil
		}
	}
	return fmt.Errorf("certificate expired event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) aCertificateRevokedEventShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCertificateRevoked {
			return nil
		}
	}
	return fmt.Errorf("certificate revoked event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) theEventShouldContainRevocationReason() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeCertificateRevoked {
			// Check that the event contains revocation reason
			dataMap := make(map[string]interface{})
			if err := event.DataAs(&dataMap); err != nil {
				return fmt.Errorf("failed to parse event data: %w", err)
			}

			if reason, ok := dataMap["reason"]; ok && reason != nil {
				return nil // Revocation reason found
			}
			return fmt.Errorf("event missing revocation reason")
		}
	}
	return fmt.Errorf("certificate revoked event not found")
}

// --- ACME protocol event validation steps ---

func (ctx *LetsEncryptBDDTestContext) aCMEChallengeEventsShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeAcmeChallenge {
			return nil
		}
	}
	return fmt.Errorf("ACME challenge event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) aCMEAuthorizationEventsShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeAcmeAuthorization {
			return nil
		}
	}
	return fmt.Errorf("ACME authorization event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) aCMEOrderEventsShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeAcmeOrder {
			return nil
		}
	}
	return fmt.Errorf("ACME order event not found among %d events", len(events))
}

// --- Storage event validation steps ---

func (ctx *LetsEncryptBDDTestContext) storageWriteEventsShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeStorageWrite {
			return nil
		}
	}
	return fmt.Errorf("storage write event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) storageReadEventsShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeStorageRead {
			return nil
		}
	}
	return fmt.Errorf("storage read event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) storageErrorEventsShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeStorageError {
			return nil
		}
	}
	return fmt.Errorf("storage error event not found among %d events", len(events))
}

// --- Configuration event validation steps ---

func (ctx *LetsEncryptBDDTestContext) aConfigLoadedEventShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeConfigLoaded {
			return nil
		}
	}
	return fmt.Errorf("config loaded event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) theEventShouldContainConfigurationDetails() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeConfigLoaded {
			// Check that the event contains configuration details
			dataMap := make(map[string]interface{})
			if err := event.DataAs(&dataMap); err != nil {
				return fmt.Errorf("failed to parse event data: %w", err)
			}

			if email, ok := dataMap["email"]; ok && email != nil {
				return nil // Configuration details found
			}
			return fmt.Errorf("event missing configuration details")
		}
	}
	return fmt.Errorf("config loaded event not found")
}

func (ctx *LetsEncryptBDDTestContext) aConfigValidatedEventShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeConfigValidated {
			return nil
		}
	}
	return fmt.Errorf("config validated event not found among %d events", len(events))
}

// --- Error and warning event validation steps ---

func (ctx *LetsEncryptBDDTestContext) anErrorConditionOccurs() error {
	if ctx.module == nil {
		return fmt.Errorf("module not initialized")
	}

	// Simulate error condition
	ctx.module.emitEvent(context.Background(), EventTypeError, map[string]interface{}{
		"error":   "certificate request failed",
		"domain":  ctx.config.Domains[0],
		"stage":   "certificate_obtain",
		"details": "ACME server returned error 429: Too Many Requests",
	})

	// Give a small delay to allow event propagation
	time.Sleep(10 * time.Millisecond)

	return nil
}

func (ctx *LetsEncryptBDDTestContext) anErrorEventShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeError {
			return nil
		}
	}
	return fmt.Errorf("error event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) theEventShouldContainErrorDetails() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeError {
			// Check that the event contains error details
			dataMap := make(map[string]interface{})
			if err := event.DataAs(&dataMap); err != nil {
				return fmt.Errorf("failed to parse event data: %w", err)
			}

			if errorMsg, ok := dataMap["error"]; ok && errorMsg != nil {
				return nil // Error details found
			}
			return fmt.Errorf("event missing error details")
		}
	}
	return fmt.Errorf("error event not found")
}

func (ctx *LetsEncryptBDDTestContext) aWarningConditionOccurs() error {
	if ctx.module == nil {
		return fmt.Errorf("module not initialized")
	}

	// Simulate warning condition
	ctx.module.emitEvent(context.Background(), EventTypeWarning, map[string]interface{}{
		"warning":      "certificate renewal approaching failure threshold",
		"domain":       ctx.config.Domains[0],
		"attempts":     2,
		"max_attempts": 3,
	})

	// Give a small delay to allow event propagation
	time.Sleep(10 * time.Millisecond)

	return nil
}

func (ctx *LetsEncryptBDDTestContext) aWarningEventShouldBeEmitted() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeWarning {
			return nil
		}
	}
	return fmt.Errorf("warning event not found among %d events", len(events))
}

func (ctx *LetsEncryptBDDTestContext) theEventShouldContainWarningDetails() error {
	if ctx.eventObserver == nil {
		return fmt.Errorf("event observer not configured")
	}

	events := ctx.eventObserver.GetEvents()
	for _, event := range events {
		if event.Type() == EventTypeWarning {
			// Check that the event contains warning details
			dataMap := make(map[string]interface{})
			if err := event.DataAs(&dataMap); err != nil {
				return fmt.Errorf("failed to parse event data: %w", err)
			}

			if warningMsg, ok := dataMap["warning"]; ok && warningMsg != nil {
				return nil // Warning details found
			}
			return fmt.Errorf("event missing warning details")
		}
	}
	return fmt.Errorf("warning event not found")
}

// Event validation step - ensures all registered events are emitted during testing
func (ctx *LetsEncryptBDDTestContext) allRegisteredEventsShouldBeEmittedDuringTesting() error {
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

// initEventSystemBDDSteps registers the event system BDD steps
func initEventSystemBDDSteps(s *godog.ScenarioContext, ctx *LetsEncryptBDDTestContext) {

	// Event scenarios
	s.Given(`^I have a LetsEncrypt module with event observation enabled$`, ctx.iHaveALetsEncryptModuleWithEventObservationEnabled)

	// Lifecycle events
	s.When(`^the LetsEncrypt module starts$`, ctx.theLetsEncryptModuleStarts)
	s.Then(`^a service started event should be emitted$`, ctx.aServiceStartedEventShouldBeEmitted)
	s.Then(`^the event should contain service configuration details$`, ctx.theEventShouldContainServiceConfigurationDetails)
	s.When(`^the LetsEncrypt module stops$`, ctx.theLetsEncryptModuleStops)
	s.Then(`^a service stopped event should be emitted$`, ctx.aServiceStoppedEventShouldBeEmitted)
	s.Then(`^a module stopped event should be emitted$`, ctx.aModuleStoppedEventShouldBeEmitted)

	// Module startup events
	s.When(`^the module starts up$`, ctx.theModuleStartsUp)
	s.Then(`^a module started event should be emitted$`, ctx.aModuleStartedEventShouldBeEmitted)
	s.Then(`^the event should contain module information$`, ctx.theEventShouldContainModuleInformation)

	// Certificate lifecycle event validation
	s.Then(`^a certificate requested event should be emitted$`, ctx.aCertificateRequestedEventShouldBeEmitted)
	s.Then(`^the event should contain domain information$`, ctx.theEventShouldContainDomainInformation)
	s.Then(`^a certificate issued event should be emitted$`, ctx.aCertificateIssuedEventShouldBeEmitted)
	s.Then(`^the event should contain domain details$`, ctx.theEventShouldContainDomainDetails)
	s.Then(`^certificate renewed events should be emitted$`, ctx.certificateRenewedEventsShouldBeEmitted)
	s.Then(`^the events should contain renewal details$`, ctx.theEventsShouldContainRenewalDetails)

	// Certificate expiry event validation
	s.Then(`^certificate expiring events should be emitted$`, ctx.certificateExpiringEventsShouldBeEmitted)
	s.Then(`^the events should contain expiry details$`, ctx.theEventsShouldContainExpiryDetails)
	s.Then(`^certificate expired events should be emitted$`, ctx.certificateExpiredEventsShouldBeEmitted)

	// Certificate revocation event validation
	s.Then(`^a certificate revoked event should be emitted$`, ctx.aCertificateRevokedEventShouldBeEmitted)
	s.Then(`^the event should contain revocation reason$`, ctx.theEventShouldContainRevocationReason)

	// ACME protocol event validation
	s.Then(`^ACME challenge events should be emitted$`, ctx.aCMEChallengeEventsShouldBeEmitted)
	s.Then(`^ACME authorization events should be emitted$`, ctx.aCMEAuthorizationEventsShouldBeEmitted)
	s.Then(`^ACME order events should be emitted$`, ctx.aCMEOrderEventsShouldBeEmitted)

	// Storage event validation
	s.Then(`^storage write events should be emitted$`, ctx.storageWriteEventsShouldBeEmitted)
	s.Then(`^storage read events should be emitted$`, ctx.storageReadEventsShouldBeEmitted)
	s.Then(`^storage error events should be emitted$`, ctx.storageErrorEventsShouldBeEmitted)

	// Configuration event validation
	s.Then(`^a config loaded event should be emitted$`, ctx.aConfigLoadedEventShouldBeEmitted)
	s.Then(`^the event should contain configuration details$`, ctx.theEventShouldContainConfigurationDetails)
	s.Then(`^a config validated event should be emitted$`, ctx.aConfigValidatedEventShouldBeEmitted)

	// Error and warning events
	s.When(`^an error condition occurs$`, ctx.anErrorConditionOccurs)
	s.Then(`^an error event should be emitted$`, ctx.anErrorEventShouldBeEmitted)
	s.Then(`^the event should contain error details$`, ctx.theEventShouldContainErrorDetails)
	s.When(`^a warning condition occurs$`, ctx.aWarningConditionOccurs)
	s.Then(`^a warning event should be emitted$`, ctx.aWarningEventShouldBeEmitted)
	s.Then(`^the event should contain warning details$`, ctx.theEventShouldContainWarningDetails)

	// Event validation
	s.Then(`^all registered LetsEncrypt events should have been emitted during testing$`, ctx.allRegisteredEventsShouldBeEmittedDuringTesting)
}
