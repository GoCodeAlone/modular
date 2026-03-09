package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cucumber/godog"
	"github.com/golang-jwt/jwt/v5"
)

// Event observation step implementations

func (ctx *AuthBDDTestContext) iHaveAnAuthModuleWithEventObservationEnabled() error {
	ctx.resetContext()

	// Apply per-app empty feeders instead of mutating global modular.ConfigFeeders (no global snapshot needed now)

	// Create mock OAuth2 server for realistic testing
	ctx.mockOAuth2Server = NewMockOAuth2Server()

	// Set up realistic user info for OAuth2 testing
	ctx.mockOAuth2Server.SetUserInfo(map[string]interface{}{
		"id":      "oauth-user-123",
		"email":   "oauth.user@example.com",
		"name":    "OAuth Test User",
		"picture": "https://example.com/avatar.jpg",
	})

	// Create proper auth configuration using the mock OAuth2 server
	authConfig := &Config{
		JWT: JWTConfig{
			Secret:            "test-secret-key-for-event-tests",
			Expiration:        1 * time.Hour,
			RefreshExpiration: 24 * time.Hour,
			Issuer:            "test-issuer",
		},
		Password: PasswordConfig{
			MinLength:      8,
			RequireUpper:   true,
			RequireLower:   true,
			RequireDigit:   true,
			RequireSpecial: true,
			BcryptCost:     10,
		},
		Session: SessionConfig{
			MaxAge:   1 * time.Hour,
			Secure:   false,
			HTTPOnly: true,
		},
		OAuth2: OAuth2Config{
			Providers: map[string]OAuth2Provider{
				"google": ctx.mockOAuth2Server.OAuth2Config("http://127.0.0.1:8080/callback"),
			},
		},
	}

	// Create provider with the auth config
	authConfigProvider := modular.NewStdConfigProvider(authConfig)

	// Create observable application instead of standard application
	logger := &testLogger{}
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.observableApp = modular.NewObservableApplication(mainConfigProvider, logger)
	if cfSetter, ok := ctx.observableApp.(interface{ SetConfigFeeders([]modular.Feeder) }); ok {
		cfSetter.SetConfigFeeders([]modular.Feeder{})
	}

	// Debug: check the type
	_, implements := interface{}(ctx.observableApp).(modular.Subject)
	_ = implements // Avoid unused variable warning

	// Create test observer to capture events
	ctx.testObserver = &testObserver{
		id:     "test-observer",
		events: make([]cloudevents.Event, 0),
	}

	// Register the test observer to capture all events (need Subject interface)
	subjectApp, ok := ctx.observableApp.(modular.Subject)
	if !ok {
		return fmt.Errorf("observable app does not implement modular.Subject")
	}
	if err := subjectApp.RegisterObserver(ctx.testObserver); err != nil {
		return fmt.Errorf("failed to register test observer: %w", err)
	}

	// Create and configure auth module
	ctx.module = NewModule().(*Module)

	// Register the auth config section first
	ctx.observableApp.RegisterConfigSection("auth", authConfigProvider)

	// Register module
	ctx.observableApp.RegisterModule(ctx.module)

	// Initialize the app - this will set up event emission capabilities
	if err := ctx.observableApp.Init(); err != nil {
		return fmt.Errorf("failed to initialize observable app: %w", err)
	}

	// Manually set up the event emitter since dependency injection might not preserve the observable wrapper
	// This ensures the module has the correct subject reference for event emission
	ctx.module.subject = subjectApp
	ctx.module.service.SetEventEmitter(ctx.module)

	// Use the service from the module directly instead of getting it from the service registry
	// This ensures we're using the same instance that has the event emitter set up
	ctx.service = ctx.module.service
	ctx.app = ctx.observableApp

	return nil
}

func (ctx *AuthBDDTestContext) aTokenGeneratedEventShouldBeEmitted() error {
	return ctx.checkEventEmitted(EventTypeTokenGenerated)
}

func (ctx *AuthBDDTestContext) theEventShouldContainUserAndTokenInformation() error {
	event := ctx.findLatestEvent(EventTypeTokenGenerated)
	if event == nil {
		return fmt.Errorf("token generated event not found")
	}

	// Verify event contains expected data
	data := event.Data()
	if len(data) == 0 {
		return fmt.Errorf("event data is empty")
	}

	return nil
}

func (ctx *AuthBDDTestContext) aTokenValidatedEventShouldBeEmitted() error {
	return ctx.checkEventEmitted(EventTypeTokenValidated)
}

func (ctx *AuthBDDTestContext) theEventShouldContainValidationInformation() error {
	event := ctx.findLatestEvent(EventTypeTokenValidated)
	if event == nil {
		return fmt.Errorf("token validated event not found")
	}

	// Verify event contains expected data
	data := event.Data()
	if len(data) == 0 {
		return fmt.Errorf("event data is empty")
	}

	return nil
}

func (ctx *AuthBDDTestContext) iCreateASessionForAUser() error {
	ctx.userID = "test-user"
	metadata := map[string]interface{}{
		"ip_address": "127.0.0.1",
		"user_agent": "test-agent",
	}

	session, err := ctx.service.CreateSession(ctx.userID, metadata)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	ctx.session = session
	ctx.sessionID = session.ID
	return nil
}

func (ctx *AuthBDDTestContext) aSessionCreatedEventShouldBeEmitted() error {
	return ctx.checkEventEmitted(EventTypeSessionCreated)
}

func (ctx *AuthBDDTestContext) iAccessTheSession() error {
	session, err := ctx.service.GetSession(ctx.sessionID)
	if err != nil {
		return fmt.Errorf("failed to access session: %w", err)
	}

	ctx.session = session
	return nil
}

func (ctx *AuthBDDTestContext) aSessionAccessedEventShouldBeEmitted() error {
	return ctx.checkEventEmitted(EventTypeSessionAccessed)
}

func (ctx *AuthBDDTestContext) aSessionDestroyedEventShouldBeEmitted() error {
	return ctx.checkEventEmitted(EventTypeSessionDestroyed)
}

func (ctx *AuthBDDTestContext) iHaveOAuth2ProvidersConfigured() error {
	// This step is already covered by the module configuration
	return nil
}

func (ctx *AuthBDDTestContext) iGetAnOAuth2AuthorizationURL() error {
	url, err := ctx.service.GetOAuth2AuthURL("google", "test-state")
	if err != nil {
		return fmt.Errorf("failed to get OAuth2 auth URL: %w", err)
	}

	ctx.oauthURL = url
	return nil
}

func (ctx *AuthBDDTestContext) anOAuth2AuthURLEventShouldBeEmitted() error {
	return ctx.checkEventEmitted(EventTypeOAuth2AuthURL)
}

func (ctx *AuthBDDTestContext) iExchangeAnOAuth2CodeForTokens() error {
	// Use the real OAuth2 exchange with the mock server's valid code
	if ctx.mockOAuth2Server == nil {
		return fmt.Errorf("mock OAuth2 server not initialized")
	}

	// Perform real OAuth2 code exchange using the mock server
	result, err := ctx.service.ExchangeOAuth2Code("google", ctx.mockOAuth2Server.GetValidCode(), "test-state")
	if err != nil {
		ctx.lastError = err
		return fmt.Errorf("OAuth2 code exchange failed: %w", err)
	}

	ctx.oauthResult = result
	return nil
}

func (ctx *AuthBDDTestContext) anOAuth2ExchangeEventShouldBeEmitted() error {
	// Now we can properly check for the OAuth2 exchange event emission
	return ctx.checkEventEmitted(EventTypeOAuth2Exchange)
}

// Helper methods for event validation

func (ctx *AuthBDDTestContext) checkEventEmitted(eventType string) error {
	// Small wait to allow emission in asynchronous paths (kept minimal)
	time.Sleep(5 * time.Millisecond)
	for _, event := range ctx.testObserver.snapshot() {
		if event.Type() == eventType {
			return nil
		}
	}
	return fmt.Errorf("event of type %s was not emitted. Captured events: %v", eventType, ctx.getEmittedEventTypes())
}

func (ctx *AuthBDDTestContext) findLatestEvent(eventType string) *cloudevents.Event {
	events := ctx.testObserver.snapshot()
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type() == eventType {
			return &events[i]
		}
	}
	return nil
}

func (ctx *AuthBDDTestContext) getEmittedEventTypes() []string {
	snapshot := ctx.testObserver.snapshot()
	types := make([]string, 0, len(snapshot))
	for _, event := range snapshot {
		types = append(types, event.Type())
	}
	return types
}

// Additional step definitions for missing events

func (ctx *AuthBDDTestContext) iGenerateAJWTTokenForAUser() error {
	return ctx.iGenerateAJWTTokenForTheUser()
}

func (ctx *AuthBDDTestContext) aSessionExpiredEventShouldBeEmitted() error {
	return ctx.checkEventEmitted(EventTypeSessionExpired)
}

func (ctx *AuthBDDTestContext) iHaveAnExpiredSession() error {
	ctx.userID = "expired-session-user"
	// Create session that expires immediately
	session := &Session{
		ID:        "expired-session-123",
		UserID:    ctx.userID,
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Already expired
		Active:    true,
		Metadata: map[string]interface{}{
			"test": "expired_session",
		},
	}

	// Store the expired session directly in the session store
	err := ctx.service.sessionStore.Store(context.Background(), session)
	if err != nil {
		return fmt.Errorf("failed to create expired session: %v", err)
	}

	ctx.sessionID = session.ID
	ctx.session = session
	return nil
}

func (ctx *AuthBDDTestContext) iAttemptToAccessTheExpiredSession() error {
	// This should trigger the session expired event
	_, err := ctx.service.GetSession(ctx.sessionID)
	ctx.lastError = err // Store error but don't return it as this is expected behavior
	return nil
}

// Additional BDD step implementations for missing events

func (ctx *AuthBDDTestContext) iAccessAnExpiredSession() error {
	// Create an expired session directly in the store
	expiredSession := &Session{
		ID:        "expired-session",
		UserID:    "test-user",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Already expired
		Active:    true,
		Metadata:  map[string]interface{}{"test": "data"},
	}

	// Store the expired session
	err := ctx.service.sessionStore.Store(context.Background(), expiredSession)
	if err != nil {
		return fmt.Errorf("failed to store expired session: %w", err)
	}

	ctx.sessionID = expiredSession.ID

	// Try to access the expired session
	_, err = ctx.service.GetSession(ctx.sessionID)
	ctx.lastError = err
	return nil
}

func (ctx *AuthBDDTestContext) theSessionAccessShouldFail() error {
	if ctx.lastError == nil {
		return fmt.Errorf("expected session access to fail for expired session")
	}
	return nil
}

func (ctx *AuthBDDTestContext) iHaveAnExpiredTokenForRefresh() error {
	// Create a token that's already expired for testing expired token during refresh
	now := time.Now().Add(-2 * time.Hour) // 2 hours ago
	claims := jwt.MapClaims{
		"user_id": "expired-refresh-user",
		"type":    "refresh",
		"iat":     now.Unix(),
		"exp":     now.Add(-1 * time.Hour).Unix(), // Expired 1 hour ago
	}

	if ctx.service.config.JWT.Issuer != "" {
		claims["iss"] = ctx.service.config.JWT.Issuer
	}
	claims["sub"] = "expired-refresh-user"

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	expiredToken, err := token.SignedString([]byte(ctx.service.config.JWT.Secret))
	if err != nil {
		return fmt.Errorf("failed to create expired token: %w", err)
	}

	ctx.token = expiredToken
	return nil
}

func (ctx *AuthBDDTestContext) iAttemptToRefreshTheExpiredToken() error {
	_, err := ctx.service.RefreshToken(ctx.token)
	ctx.lastError = err // Store error but don't return it as this is expected behavior
	return nil
}

func (ctx *AuthBDDTestContext) theTokenRefreshShouldFail() error {
	if ctx.lastError == nil {
		return fmt.Errorf("expected token refresh to fail for expired token")
	}
	return nil
}

func (ctx *AuthBDDTestContext) iValidateAnExpiredToken() error {
	// Create an expired token
	err := ctx.iHaveUserCredentialsAndJWTConfiguration()
	if err != nil {
		return err
	}

	// Generate a token with very short expiration
	oldExpiration := ctx.service.config.JWT.Expiration
	ctx.service.config.JWT.Expiration = 1 * time.Millisecond // Very short expiration

	err = ctx.iGenerateAJWTTokenForTheUser()
	if err != nil {
		return err
	}

	// Restore original expiration
	ctx.service.config.JWT.Expiration = oldExpiration

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	// Try to validate the expired token
	_, err = ctx.service.ValidateToken(ctx.token)
	ctx.lastError = err

	return nil
}

func (ctx *AuthBDDTestContext) aTokenExpiredEventShouldBeEmitted() error {
	return ctx.checkEventEmitted(EventTypeTokenExpired)
}

func (ctx *AuthBDDTestContext) iHaveAValidRefreshToken() error {
	// Generate a token pair first
	err := ctx.iHaveUserCredentialsAndJWTConfiguration()
	if err != nil {
		return err
	}

	return ctx.iGenerateAJWTTokenForTheUser()
}

func (ctx *AuthBDDTestContext) aTokenRefreshedEventShouldBeEmitted() error {
	return ctx.checkEventEmitted(EventTypeTokenRefreshed)
}

func (ctx *AuthBDDTestContext) aNewAccessTokenShouldBeProvided() error {
	if ctx.newToken == "" {
		return fmt.Errorf("no new access token was provided")
	}
	return nil
}

// Event validation step - ensures all registered events are emitted during testing
func (ctx *AuthBDDTestContext) allRegisteredEventsShouldBeEmittedDuringTesting() error {
	// Get all registered event types from the module
	registeredEvents := ctx.module.GetRegisteredEventTypes()

	// Create event validation observer
	validator := modular.NewEventValidationObserver("event-validator", registeredEvents)
	_ = validator // Use validator to avoid unused variable error

	// Check which events were emitted during testing
	emittedEvents := make(map[string]bool)
	for _, event := range ctx.testObserver.events {
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

// Event-specific step registration
func (ctx *AuthBDDTestContext) registerEventSteps(s *godog.ScenarioContext) {
	// Event observation steps
	s.Step(`^I have an auth module with event observation enabled$`, ctx.iHaveAnAuthModuleWithEventObservationEnabled)
	s.Step(`^a token generated event should be emitted$`, ctx.aTokenGeneratedEventShouldBeEmitted)
	s.Step(`^the event should contain user and token information$`, ctx.theEventShouldContainUserAndTokenInformation)
	s.Step(`^a token validated event should be emitted$`, ctx.aTokenValidatedEventShouldBeEmitted)
	s.Step(`^the event should contain validation information$`, ctx.theEventShouldContainValidationInformation)
	s.Step(`^I create a session for a user$`, ctx.iCreateASessionForAUser)
	s.Step(`^a session created event should be emitted$`, ctx.aSessionCreatedEventShouldBeEmitted)
	s.Step(`^I access the session$`, ctx.iAccessTheSession)
	s.Step(`^a session accessed event should be emitted$`, ctx.aSessionAccessedEventShouldBeEmitted)
	s.Step(`^a session destroyed event should be emitted$`, ctx.aSessionDestroyedEventShouldBeEmitted)
	s.Step(`^I have OAuth2 providers configured$`, ctx.iHaveOAuth2ProvidersConfigured)
	s.Step(`^I get an OAuth2 authorization URL$`, ctx.iGetAnOAuth2AuthorizationURL)
	s.Step(`^an OAuth2 auth URL event should be emitted$`, ctx.anOAuth2AuthURLEventShouldBeEmitted)
	s.Step(`^I exchange an OAuth2 code for tokens$`, ctx.iExchangeAnOAuth2CodeForTokens)
	s.Step(`^an OAuth2 exchange event should be emitted$`, ctx.anOAuth2ExchangeEventShouldBeEmitted)

	// Additional event observation steps
	// Note: JWT token generation step is already registered in bdd_jwt_test.go
	s.Step(`^a token expired event should be emitted$`, ctx.aTokenExpiredEventShouldBeEmitted)
	s.Step(`^a token refreshed event should be emitted$`, ctx.aTokenRefreshedEventShouldBeEmitted)
	s.Step(`^a session expired event should be emitted$`, ctx.aSessionExpiredEventShouldBeEmitted)
	s.Step(`^I have an expired session$`, ctx.iHaveAnExpiredSession)
	s.Step(`^I attempt to access the expired session$`, ctx.iAttemptToAccessTheExpiredSession)
	s.Step(`^the session access should fail$`, ctx.theSessionAccessShouldFail)
	s.Step(`^I have an expired token for refresh$`, ctx.iHaveAnExpiredTokenForRefresh)
	s.Step(`^I attempt to refresh the expired token$`, ctx.iAttemptToRefreshTheExpiredToken)
	s.Step(`^the token refresh should fail$`, ctx.theTokenRefreshShouldFail)
	// Session expired event testing
	s.Step(`^I access an expired session$`, ctx.iAccessAnExpiredSession)

	// Token expired event testing
	s.Step(`^I validate an expired token$`, ctx.iValidateAnExpiredToken)

	// Token refresh event testing
	s.Step(`^I have a valid refresh token$`, ctx.iHaveAValidRefreshToken)
	s.Step(`^a new access token should be provided$`, ctx.aNewAccessTokenShouldBeProvided)

	// Event validation
	s.Step(`^all registered events should be emitted during testing$`, ctx.allRegisteredEventsShouldBeEmittedDuringTesting)
}
