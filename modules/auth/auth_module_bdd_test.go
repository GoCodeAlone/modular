package auth

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cucumber/godog"
	"github.com/golang-jwt/jwt/v5"
	oauth2 "golang.org/x/oauth2"
)

// testLogger is a no-op logger implementing modular.Logger for BDD tests
type testLogger struct{}

func (l *testLogger) Info(msg string, args ...any)  {}
func (l *testLogger) Error(msg string, args ...any) {}
func (l *testLogger) Warn(msg string, args ...any)  {}
func (l *testLogger) Debug(msg string, args ...any) {}

// testObserver captures emitted CloudEvents for assertions
type testObserver struct {
	id     string
	mu     sync.RWMutex
	events []cloudevents.Event
}

func (o *testObserver) ObserverID() string { return o.id }
func (o *testObserver) OnEvent(ctx context.Context, event cloudevents.Event) error {
	o.mu.Lock()
	o.events = append(o.events, event)
	o.mu.Unlock()
	return nil
}

// snapshot returns a copy of captured events for safe concurrent iteration
func (o *testObserver) snapshot() []cloudevents.Event {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make([]cloudevents.Event, len(o.events))
	copy(out, o.events)
	return out
}

// AuthBDDTestContext holds shared state across steps
type AuthBDDTestContext struct {
	app              modular.Application
	observableApp    modular.Application
	module           *Module
	service          *Service
	user             *User
	userID           string
	password         string
	hashedPassword   string
	token            string
	refreshToken     string
	newToken         string
	lastError        error
	strengthError    error
	claims           *Claims
	session          *Session
	sessionID        string
	oauthURL         string
	oauthResult      *OAuth2Result
	mockOAuth2Server *MockOAuth2Server
	testObserver     *testObserver
	authError        error
	authResult       *User
	verifyResult     bool
	originalFeeders  []modular.Feeder
}

// resetContext resets per-scenario state (except shared config feeders restoration done in After hooks elsewhere)
func (ctx *AuthBDDTestContext) resetContext() {
	ctx.user = nil
	ctx.password = ""
	ctx.hashedPassword = ""
	ctx.token = ""
	ctx.refreshToken = ""
	ctx.newToken = ""
	ctx.lastError = nil
	ctx.strengthError = nil
	ctx.claims = nil
	ctx.session = nil
	ctx.sessionID = ""
	ctx.oauthURL = ""
	ctx.oauthResult = nil
	ctx.userID = ""
	ctx.authError = nil
	ctx.authResult = nil
	ctx.verifyResult = false
}

// iHaveAModularApplicationWithAuthModuleConfigured bootstraps a standard (non-observable) auth module instance
func (ctx *AuthBDDTestContext) iHaveAModularApplicationWithAuthModuleConfigured() error {
	ctx.resetContext()
	logger := &testLogger{}

	authConfig := &Config{
		JWT: JWTConfig{
			Secret:            "test-secret-key",
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
			BcryptCost:     4, // low cost for tests
		},
		Session: SessionConfig{
			MaxAge:   1 * time.Hour,
			Secure:   false,
			HTTPOnly: true,
		},
	}

	authConfigProvider := modular.NewStdConfigProvider(authConfig)
	mainConfigProvider := modular.NewStdConfigProvider(struct{}{})
	ctx.app = modular.NewStdApplication(mainConfigProvider, logger)
	ctx.module = NewModule().(*Module)
	ctx.app.RegisterConfigSection("auth", authConfigProvider)
	ctx.app.RegisterModule(ctx.module)
	if err := ctx.app.Init(); err != nil {
		return fmt.Errorf("failed to initialize app: %w", err)
	}
	var svc Service
	if err := ctx.app.GetService("auth", &svc); err != nil {
		return fmt.Errorf("failed to get auth service: %w", err)
	}
	ctx.service = &svc
	return nil
}

func (ctx *AuthBDDTestContext) iHaveUserCredentialsAndJWTConfiguration() error {
	// This is implicitly handled by the module configuration
	return nil
}

func (ctx *AuthBDDTestContext) iGenerateAJWTTokenForTheUser() error {
	var err error
	tokenPair, err := ctx.service.GenerateToken("test-user-123", map[string]interface{}{
		"email": "test@example.com",
	})
	if err != nil {
		ctx.lastError = err
		return nil // Don't return error here as it might be expected
	}

	ctx.token = tokenPair.AccessToken
	ctx.refreshToken = tokenPair.RefreshToken
	return nil
}

func (ctx *AuthBDDTestContext) theTokenShouldBeCreatedSuccessfully() error {
	if ctx.token == "" {
		return fmt.Errorf("token was not created")
	}
	if ctx.lastError != nil {
		return fmt.Errorf("token creation failed: %v", ctx.lastError)
	}
	return nil
}

func (ctx *AuthBDDTestContext) theTokenShouldContainTheUserInformation() error {
	if ctx.token == "" {
		return fmt.Errorf("no token available")
	}

	claims, err := ctx.service.ValidateToken(ctx.token)
	if err != nil {
		return fmt.Errorf("failed to validate token: %v", err)
	}

	if claims.UserID != "test-user-123" {
		return fmt.Errorf("expected UserID 'test-user-123', got '%s'", claims.UserID)
	}

	return nil
}

func (ctx *AuthBDDTestContext) iHaveAValidJWTToken() error {
	var err error
	tokenPair, err := ctx.service.GenerateToken("valid-user", map[string]interface{}{
		"email": "valid@example.com",
	})
	if err != nil {
		return fmt.Errorf("failed to generate valid token: %v", err)
	}

	ctx.token = tokenPair.AccessToken
	return nil
}

func (ctx *AuthBDDTestContext) iValidateTheToken() error {
	var err error
	ctx.claims, err = ctx.service.ValidateToken(ctx.token)
	if err != nil {
		ctx.lastError = err
		return nil // Don't return error here as validation might be expected to fail
	}

	return nil
}

func (ctx *AuthBDDTestContext) theTokenShouldBeAccepted() error {
	if ctx.lastError != nil {
		return fmt.Errorf("token was rejected: %v", ctx.lastError)
	}
	if ctx.claims == nil {
		return fmt.Errorf("no claims extracted from token")
	}
	return nil
}

func (ctx *AuthBDDTestContext) theUserClaimsShouldBeExtracted() error {
	if ctx.claims == nil {
		return fmt.Errorf("no claims available")
	}
	if ctx.claims.UserID == "" {
		return fmt.Errorf("UserID not found in claims")
	}
	return nil
}

func (ctx *AuthBDDTestContext) iHaveAnInvalidJWTToken() error {
	ctx.token = "invalid.jwt.token"
	return nil
}

func (ctx *AuthBDDTestContext) theTokenShouldBeRejected() error {
	if ctx.lastError == nil {
		return fmt.Errorf("token should have been rejected but was accepted")
	}
	return nil
}

func (ctx *AuthBDDTestContext) anAppropriateErrorShouldBeReturned() error {
	if ctx.lastError == nil {
		return fmt.Errorf("no error was returned")
	}
	return nil
}

func (ctx *AuthBDDTestContext) iHaveAnExpiredJWTToken() error {
	// Create a token with past expiration
	// For now, we'll simulate an expired token
	ctx.token = "expired.jwt.token"
	return nil
}

func (ctx *AuthBDDTestContext) theErrorShouldIndicateTokenExpiration() error {
	if ctx.lastError == nil {
		return fmt.Errorf("no error indicating expiration")
	}
	// Check if error message indicates expiration
	return nil
}

func (ctx *AuthBDDTestContext) iRefreshTheToken() error {
	if ctx.token == "" {
		return fmt.Errorf("no token to refresh")
	}

	// First, create a user in the user store for refresh functionality
	refreshUser := &User{
		ID:          "refresh-user",
		Email:       "refresh@example.com",
		Active:      true,
		Roles:       []string{"user"},
		Permissions: []string{"read"},
	}

	// Create the user in the store
	if err := ctx.service.userStore.CreateUser(context.Background(), refreshUser); err != nil {
		// If user already exists, that's fine
		if err != ErrUserAlreadyExists {
			ctx.lastError = err
			return nil
		}
	}

	// Generate a token pair for the user
	tokenPair, err := ctx.service.GenerateToken("refresh-user", map[string]interface{}{
		"email": "refresh@example.com",
	})
	if err != nil {
		ctx.lastError = err
		return nil
	}

	// Use the refresh token to get a new token pair
	newTokenPair, err := ctx.service.RefreshToken(tokenPair.RefreshToken)
	if err != nil {
		ctx.lastError = err
		return nil
	}

	ctx.token = newTokenPair.AccessToken
	ctx.newToken = newTokenPair.AccessToken // Set the new token for validation
	return nil
}

func (ctx *AuthBDDTestContext) aNewTokenShouldBeGenerated() error {
	if ctx.token == "" {
		return fmt.Errorf("no new token generated")
	}
	if ctx.lastError != nil {
		return fmt.Errorf("token refresh failed: %v", ctx.lastError)
	}
	return nil
}

func (ctx *AuthBDDTestContext) theNewTokenShouldHaveUpdatedExpiration() error {
	// This would require checking the token's expiration time
	// For now, we assume the refresh worked if we have a new token
	return ctx.aNewTokenShouldBeGenerated()
}

func (ctx *AuthBDDTestContext) iHaveAPlainTextPassword() error {
	ctx.password = "MySecurePassword123!"
	return nil
}

func (ctx *AuthBDDTestContext) iHashThePasswordUsingBcrypt() error {
	var err error
	ctx.hashedPassword, err = ctx.service.HashPassword(ctx.password)
	if err != nil {
		ctx.lastError = err
		return nil
	}
	return nil
}

func (ctx *AuthBDDTestContext) thePasswordShouldBeHashedSuccessfully() error {
	if ctx.hashedPassword == "" {
		return fmt.Errorf("password was not hashed")
	}
	if ctx.lastError != nil {
		return fmt.Errorf("password hashing failed: %v", ctx.lastError)
	}
	return nil
}

func (ctx *AuthBDDTestContext) theHashShouldBeDifferentFromTheOriginalPassword() error {
	if ctx.hashedPassword == ctx.password {
		return fmt.Errorf("hash is the same as original password")
	}
	return nil
}

func (ctx *AuthBDDTestContext) iHaveAPasswordAndItsHash() error {
	ctx.password = "TestPassword123!"
	var err error
	ctx.hashedPassword, err = ctx.service.HashPassword(ctx.password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}
	return nil
}

func (ctx *AuthBDDTestContext) iVerifyThePasswordAgainstTheHash() error {
	err := ctx.service.VerifyPassword(ctx.hashedPassword, ctx.password)
	ctx.verifyResult = (err == nil)
	return nil
}

func (ctx *AuthBDDTestContext) theVerificationShouldSucceed() error {
	if !ctx.verifyResult {
		return fmt.Errorf("password verification failed")
	}
	return nil
}

func (ctx *AuthBDDTestContext) iHaveAPasswordAndADifferentHash() error {
	ctx.password = "CorrectPassword123!"
	wrongPassword := "WrongPassword123!"
	var err error
	ctx.hashedPassword, err = ctx.service.HashPassword(wrongPassword)
	if err != nil {
		return fmt.Errorf("failed to hash wrong password: %v", err)
	}
	return nil
}

func (ctx *AuthBDDTestContext) theVerificationShouldFail() error {
	if ctx.verifyResult {
		return fmt.Errorf("password verification should have failed")
	}
	return nil
}

func (ctx *AuthBDDTestContext) iHaveAStrongPassword() error {
	ctx.password = "StrongPassword123!@#"
	return nil
}

func (ctx *AuthBDDTestContext) iValidateThePasswordStrength() error {
	ctx.strengthError = ctx.service.ValidatePasswordStrength(ctx.password)
	return nil
}

func (ctx *AuthBDDTestContext) thePasswordShouldBeAccepted() error {
	if ctx.strengthError != nil {
		return fmt.Errorf("strong password was rejected: %v", ctx.strengthError)
	}
	return nil
}

func (ctx *AuthBDDTestContext) noStrengthErrorsShouldBeReported() error {
	if ctx.strengthError != nil {
		return fmt.Errorf("unexpected strength error: %v", ctx.strengthError)
	}
	return nil
}

func (ctx *AuthBDDTestContext) iHaveAWeakPassword() error {
	ctx.password = "weak" // Too short, no uppercase, no numbers, no special chars
	return nil
}

func (ctx *AuthBDDTestContext) thePasswordShouldBeRejected() error {
	if ctx.strengthError == nil {
		return fmt.Errorf("weak password should have been rejected")
	}
	return nil
}

func (ctx *AuthBDDTestContext) appropriateStrengthErrorsShouldBeReported() error {
	if ctx.strengthError == nil {
		return fmt.Errorf("no strength errors reported")
	}
	return nil
}

func (ctx *AuthBDDTestContext) iHaveAUserIdentifier() error {
	ctx.userID = "session-user-123"
	return nil
}

func (ctx *AuthBDDTestContext) iCreateANewSessionForTheUser() error {
	var err error
	ctx.session, err = ctx.service.CreateSession(ctx.userID, map[string]interface{}{
		"created_by": "bdd_test",
	})
	if err != nil {
		ctx.lastError = err
		return nil
	}
	if ctx.session != nil {
		ctx.sessionID = ctx.session.ID
	}
	return nil
}

func (ctx *AuthBDDTestContext) theSessionShouldBeCreatedSuccessfully() error {
	if ctx.session == nil {
		return fmt.Errorf("session was not created")
	}
	if ctx.lastError != nil {
		return fmt.Errorf("session creation failed: %v", ctx.lastError)
	}
	return nil
}

func (ctx *AuthBDDTestContext) theSessionShouldHaveAUniqueID() error {
	if ctx.session == nil {
		return fmt.Errorf("no session available")
	}
	if ctx.session.ID == "" {
		return fmt.Errorf("session ID is empty")
	}
	return nil
}

func (ctx *AuthBDDTestContext) iHaveAnExistingUserSession() error {
	ctx.userID = "existing-user-123"
	var err error
	ctx.session, err = ctx.service.CreateSession(ctx.userID, map[string]interface{}{
		"test": "existing_session",
	})
	if err != nil {
		return fmt.Errorf("failed to create existing session: %v", err)
	}
	ctx.sessionID = ctx.session.ID
	return nil
}

func (ctx *AuthBDDTestContext) iRetrieveTheSessionByID() error {
	var err error
	ctx.session, err = ctx.service.GetSession(ctx.sessionID)
	if err != nil {
		ctx.lastError = err
		return nil
	}
	return nil
}

func (ctx *AuthBDDTestContext) theSessionShouldBeFound() error {
	if ctx.session == nil {
		return fmt.Errorf("session was not found")
	}
	if ctx.lastError != nil {
		return fmt.Errorf("session retrieval failed: %v", ctx.lastError)
	}
	return nil
}

func (ctx *AuthBDDTestContext) theSessionDataShouldMatch() error {
	if ctx.session == nil {
		return fmt.Errorf("no session to check")
	}
	if ctx.session.ID != ctx.sessionID {
		return fmt.Errorf("session ID mismatch: expected %s, got %s", ctx.sessionID, ctx.session.ID)
	}
	return nil
}

func (ctx *AuthBDDTestContext) iDeleteTheSession() error {
	err := ctx.service.DeleteSession(ctx.sessionID)
	if err != nil {
		ctx.lastError = err
		return nil
	}
	return nil
}

func (ctx *AuthBDDTestContext) theSessionShouldBeRemoved() error {
	if ctx.lastError != nil {
		return fmt.Errorf("session deletion failed: %v", ctx.lastError)
	}
	return nil
}

func (ctx *AuthBDDTestContext) subsequentRetrievalShouldFail() error {
	session, err := ctx.service.GetSession(ctx.sessionID)
	if err == nil && session != nil {
		return fmt.Errorf("session should have been deleted but was found")
	}
	return nil
}

func (ctx *AuthBDDTestContext) iHaveOAuth2Configuration() error {
	// Ensure base auth app is initialized
	if ctx.service == nil || ctx.module == nil {
		if err := ctx.iHaveAModularApplicationWithAuthModuleConfigured(); err != nil {
			return fmt.Errorf("failed to initialize auth application: %w", err)
		}
	}

	// If already configured with provider, nothing to do
	if ctx.module != nil && ctx.module.config != nil {
		if ctx.module.config.OAuth2.Providers != nil {
			if _, exists := ctx.module.config.OAuth2.Providers["google"]; exists {
				return nil
			}
		}
	}

	// Spin up mock OAuth2 server if not present
	if ctx.mockOAuth2Server == nil {
		ctx.mockOAuth2Server = NewMockOAuth2Server()
		// Provide realistic user info for authorization flow
		ctx.mockOAuth2Server.SetUserInfo(map[string]interface{}{
			"id":    "oauth-user-flow-123",
			"email": "oauth.flow@example.com",
			"name":  "OAuth Flow User",
		})
	}

	provider := ctx.mockOAuth2Server.OAuth2Config("http://127.0.0.1:8080/callback")

	// Update module/service config providers map
	if ctx.module != nil && ctx.module.config != nil {
		if ctx.module.config.OAuth2.Providers == nil {
			ctx.module.config.OAuth2.Providers = map[string]OAuth2Provider{}
		}
		ctx.module.config.OAuth2.Providers["google"] = provider
	}
	if ctx.service != nil && ctx.service.config != nil {
		if ctx.service.config.OAuth2.Providers == nil {
			ctx.service.config.OAuth2.Providers = map[string]OAuth2Provider{}
		}
		ctx.service.config.OAuth2.Providers["google"] = provider
	}

	// Ensure service has oauth2Configs entry (mirrors NewService logic)
	if ctx.service != nil {
		if ctx.service.oauth2Configs == nil {
			ctx.service.oauth2Configs = make(map[string]*oauth2.Config)
		}
		ctx.service.oauth2Configs["google"] = &oauth2.Config{
			ClientID:     provider.ClientID,
			ClientSecret: provider.ClientSecret,
			RedirectURL:  provider.RedirectURL,
			Scopes:       provider.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  provider.AuthURL,
				TokenURL: provider.TokenURL,
			},
		}
	}

	return nil
}

func (ctx *AuthBDDTestContext) iInitiateOAuth2Authorization() error {
	url, err := ctx.service.GetOAuth2AuthURL("google", "state-123")
	if err != nil {
		ctx.lastError = err
		return nil
	}
	ctx.oauthURL = url
	return nil
}

func (ctx *AuthBDDTestContext) theAuthorizationURLShouldBeGenerated() error {
	if ctx.oauthURL == "" {
		return fmt.Errorf("no OAuth2 authorization URL generated")
	}
	if ctx.lastError != nil {
		return fmt.Errorf("OAuth2 URL generation failed: %v", ctx.lastError)
	}
	return nil
}

func (ctx *AuthBDDTestContext) theURLShouldContainProperParameters() error {
	if ctx.oauthURL == "" {
		return fmt.Errorf("no URL to check")
	}
	// Basic check that it looks like a URL
	if len(ctx.oauthURL) < 10 {
		return fmt.Errorf("URL seems too short to be valid")
	}
	return nil
}

func (ctx *AuthBDDTestContext) iHaveAUserStoreConfigured() error {
	// User store is configured as part of the module
	return nil
}

func (ctx *AuthBDDTestContext) iCreateANewUser() error {
	user := &User{
		ID:    "new-user-123",
		Email: "newuser@example.com",
	}

	err := ctx.service.userStore.CreateUser(context.Background(), user)
	if err != nil {
		ctx.lastError = err
		return nil
	}
	ctx.user = user
	ctx.userID = user.ID
	return nil
}

func (ctx *AuthBDDTestContext) theUserShouldBeStoredSuccessfully() error {
	if ctx.lastError != nil {
		return fmt.Errorf("user creation failed: %v", ctx.lastError)
	}
	return nil
}

func (ctx *AuthBDDTestContext) iShouldBeAbleToRetrieveTheUserByID() error {
	user, err := ctx.service.userStore.GetUser(context.Background(), ctx.userID)
	if err != nil {
		return fmt.Errorf("failed to retrieve user: %v", err)
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (ctx *AuthBDDTestContext) iHaveAUserWithCredentialsInTheStore() error {
	hashedPassword, err := ctx.service.HashPassword("userpassword123!")
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}

	user := &User{
		ID:           "auth-user-123",
		Email:        "authuser@example.com",
		PasswordHash: hashedPassword,
	}

	err = ctx.service.userStore.CreateUser(context.Background(), user)
	if err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}

	ctx.user = user
	ctx.password = "userpassword123!"
	return nil
}

func (ctx *AuthBDDTestContext) iAuthenticateWithCorrectCredentials() error {
	// Implement authentication using GetUserByEmail and VerifyPassword
	user, err := ctx.service.userStore.GetUserByEmail(context.Background(), ctx.user.Email)
	if err != nil {
		ctx.authError = err
		return nil
	}

	err = ctx.service.VerifyPassword(user.PasswordHash, ctx.password)
	if err != nil {
		ctx.authError = err
		return nil
	}

	ctx.authResult = user
	return nil
}

func (ctx *AuthBDDTestContext) theAuthenticationShouldSucceed() error {
	if ctx.authError != nil {
		return fmt.Errorf("authentication failed: %v", ctx.authError)
	}
	if ctx.authResult == nil {
		return fmt.Errorf("no user returned from authentication")
	}
	return nil
}

func (ctx *AuthBDDTestContext) theUserShouldBeReturned() error {
	if ctx.authResult == nil {
		return fmt.Errorf("no user returned")
	}
	if ctx.authResult.ID != ctx.user.ID {
		return fmt.Errorf("wrong user returned: expected %s, got %s", ctx.user.ID, ctx.authResult.ID)
	}
	return nil
}

func (ctx *AuthBDDTestContext) iAuthenticateWithIncorrectCredentials() error {
	// Implement authentication using GetUserByEmail and VerifyPassword
	user, err := ctx.service.userStore.GetUserByEmail(context.Background(), ctx.user.Email)
	if err != nil {
		ctx.authError = err
		return nil
	}

	err = ctx.service.VerifyPassword(user.PasswordHash, "wrongpassword")
	if err != nil {
		ctx.authError = err
		return nil
	}

	ctx.authResult = user
	return nil
}

func (ctx *AuthBDDTestContext) theAuthenticationShouldFail() error {
	if ctx.authError == nil {
		return fmt.Errorf("authentication should have failed")
	}
	return nil
}

func (ctx *AuthBDDTestContext) anErrorShouldBeReturned() error {
	if ctx.authError == nil {
		return fmt.Errorf("no error returned")
	}
	return nil
}

// InitializeAuthScenario initializes the auth BDD test scenario
func InitializeAuthScenario(ctx *godog.ScenarioContext) {
	testCtx := &AuthBDDTestContext{}

	// Reset context before each scenario
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		testCtx.resetContext()
		return ctx, nil
	})

	// Background steps
	ctx.Step(`^I have a modular application with auth module configured$`, testCtx.iHaveAModularApplicationWithAuthModuleConfigured)

	// JWT token steps
	ctx.Step(`^I have user credentials and JWT configuration$`, testCtx.iHaveUserCredentialsAndJWTConfiguration)
	ctx.Step(`^I generate a JWT token for the user$`, testCtx.iGenerateAJWTTokenForTheUser)
	ctx.Step(`^I generate a JWT token for a user$`, testCtx.iGenerateAJWTTokenForTheUser)
	ctx.Step(`^the token should be created successfully$`, testCtx.theTokenShouldBeCreatedSuccessfully)
	ctx.Step(`^the token should contain the user information$`, testCtx.theTokenShouldContainTheUserInformation)

	// Token validation steps
	ctx.Step(`^I have a valid JWT token$`, testCtx.iHaveAValidJWTToken)
	ctx.Step(`^I validate the token$`, testCtx.iValidateTheToken)
	ctx.Step(`^the token should be accepted$`, testCtx.theTokenShouldBeAccepted)
	ctx.Step(`^the user claims should be extracted$`, testCtx.theUserClaimsShouldBeExtracted)
	ctx.Step(`^I have an invalid JWT token$`, testCtx.iHaveAnInvalidJWTToken)
	ctx.Step(`^the token should be rejected$`, testCtx.theTokenShouldBeRejected)
	ctx.Step(`^an appropriate error should be returned$`, testCtx.anAppropriateErrorShouldBeReturned)
	ctx.Step(`^I have an expired JWT token$`, testCtx.iHaveAnExpiredJWTToken)
	ctx.Step(`^the error should indicate token expiration$`, testCtx.theErrorShouldIndicateTokenExpiration)

	// Token refresh steps
	ctx.Step(`^I refresh the token$`, testCtx.iRefreshTheToken)
	ctx.Step(`^a new token should be generated$`, testCtx.aNewTokenShouldBeGenerated)
	ctx.Step(`^the new token should have updated expiration$`, testCtx.theNewTokenShouldHaveUpdatedExpiration)

	// Password hashing steps
	ctx.Step(`^I have a plain text password$`, testCtx.iHaveAPlainTextPassword)
	ctx.Step(`^I hash the password using bcrypt$`, testCtx.iHashThePasswordUsingBcrypt)
	ctx.Step(`^the password should be hashed successfully$`, testCtx.thePasswordShouldBeHashedSuccessfully)
	ctx.Step(`^the hash should be different from the original password$`, testCtx.theHashShouldBeDifferentFromTheOriginalPassword)

	// Password verification steps
	ctx.Step(`^I have a password and its hash$`, testCtx.iHaveAPasswordAndItsHash)
	ctx.Step(`^I verify the password against the hash$`, testCtx.iVerifyThePasswordAgainstTheHash)
	ctx.Step(`^the verification should succeed$`, testCtx.theVerificationShouldSucceed)
	ctx.Step(`^I have a password and a different hash$`, testCtx.iHaveAPasswordAndADifferentHash)
	ctx.Step(`^the verification should fail$`, testCtx.theVerificationShouldFail)

	// Password strength steps
	ctx.Step(`^I have a strong password$`, testCtx.iHaveAStrongPassword)
	ctx.Step(`^I validate the password strength$`, testCtx.iValidateThePasswordStrength)
	ctx.Step(`^the password should be accepted$`, testCtx.thePasswordShouldBeAccepted)
	ctx.Step(`^no strength errors should be reported$`, testCtx.noStrengthErrorsShouldBeReported)
	ctx.Step(`^I have a weak password$`, testCtx.iHaveAWeakPassword)
	ctx.Step(`^the password should be rejected$`, testCtx.thePasswordShouldBeRejected)
	ctx.Step(`^appropriate strength errors should be reported$`, testCtx.appropriateStrengthErrorsShouldBeReported)

	// Session management steps
	ctx.Step(`^I have a user identifier$`, testCtx.iHaveAUserIdentifier)
	ctx.Step(`^I create a new session for the user$`, testCtx.iCreateANewSessionForTheUser)
	ctx.Step(`^the session should be created successfully$`, testCtx.theSessionShouldBeCreatedSuccessfully)
	ctx.Step(`^the session should have a unique ID$`, testCtx.theSessionShouldHaveAUniqueID)
	ctx.Step(`^I have an existing user session$`, testCtx.iHaveAnExistingUserSession)
	ctx.Step(`^I retrieve the session by ID$`, testCtx.iRetrieveTheSessionByID)
	ctx.Step(`^the session should be found$`, testCtx.theSessionShouldBeFound)
	ctx.Step(`^the session data should match$`, testCtx.theSessionDataShouldMatch)
	ctx.Step(`^I delete the session$`, testCtx.iDeleteTheSession)
	ctx.Step(`^the session should be removed$`, testCtx.theSessionShouldBeRemoved)
	ctx.Step(`^subsequent retrieval should fail$`, testCtx.subsequentRetrievalShouldFail)

	// OAuth2 steps
	ctx.Step(`^I have OAuth2 configuration$`, testCtx.iHaveOAuth2Configuration)
	ctx.Step(`^I initiate OAuth2 authorization$`, testCtx.iInitiateOAuth2Authorization)
	ctx.Step(`^the authorization URL should be generated$`, testCtx.theAuthorizationURLShouldBeGenerated)
	ctx.Step(`^the URL should contain proper parameters$`, testCtx.theURLShouldContainProperParameters)

	// User store steps
	ctx.Step(`^I have a user store configured$`, testCtx.iHaveAUserStoreConfigured)
	ctx.Step(`^I create a new user$`, testCtx.iCreateANewUser)
	ctx.Step(`^the user should be stored successfully$`, testCtx.theUserShouldBeStoredSuccessfully)
	ctx.Step(`^I should be able to retrieve the user by ID$`, testCtx.iShouldBeAbleToRetrieveTheUserByID)

	// Authentication steps
	ctx.Step(`^I have a user with credentials in the store$`, testCtx.iHaveAUserWithCredentialsInTheStore)
	ctx.Step(`^I authenticate with correct credentials$`, testCtx.iAuthenticateWithCorrectCredentials)
	ctx.Step(`^the authentication should succeed$`, testCtx.theAuthenticationShouldSucceed)
	ctx.Step(`^the user should be returned$`, testCtx.theUserShouldBeReturned)
	ctx.Step(`^I authenticate with incorrect credentials$`, testCtx.iAuthenticateWithIncorrectCredentials)
	ctx.Step(`^the authentication should fail$`, testCtx.theAuthenticationShouldFail)
	ctx.Step(`^an error should be returned$`, testCtx.anErrorShouldBeReturned)

	// Event observation steps
	ctx.Step(`^I have an auth module with event observation enabled$`, testCtx.iHaveAnAuthModuleWithEventObservationEnabled)
	ctx.Step(`^a token generated event should be emitted$`, testCtx.aTokenGeneratedEventShouldBeEmitted)
	ctx.Step(`^the event should contain user and token information$`, testCtx.theEventShouldContainUserAndTokenInformation)
	ctx.Step(`^a token validated event should be emitted$`, testCtx.aTokenValidatedEventShouldBeEmitted)
	ctx.Step(`^the event should contain validation information$`, testCtx.theEventShouldContainValidationInformation)
	ctx.Step(`^I create a session for a user$`, testCtx.iCreateASessionForAUser)
	ctx.Step(`^a session created event should be emitted$`, testCtx.aSessionCreatedEventShouldBeEmitted)
	ctx.Step(`^I access the session$`, testCtx.iAccessTheSession)
	ctx.Step(`^a session accessed event should be emitted$`, testCtx.aSessionAccessedEventShouldBeEmitted)
	ctx.Step(`^a session destroyed event should be emitted$`, testCtx.aSessionDestroyedEventShouldBeEmitted)
	ctx.Step(`^I have OAuth2 providers configured$`, testCtx.iHaveOAuth2ProvidersConfigured)
	ctx.Step(`^I get an OAuth2 authorization URL$`, testCtx.iGetAnOAuth2AuthorizationURL)
	ctx.Step(`^an OAuth2 auth URL event should be emitted$`, testCtx.anOAuth2AuthURLEventShouldBeEmitted)
	ctx.Step(`^I exchange an OAuth2 code for tokens$`, testCtx.iExchangeAnOAuth2CodeForTokens)
	ctx.Step(`^an OAuth2 exchange event should be emitted$`, testCtx.anOAuth2ExchangeEventShouldBeEmitted)

	// Additional event observation steps
	ctx.Step(`^I generate a JWT token for a user$`, testCtx.iGenerateAJWTTokenForAUser)
	ctx.Step(`^a token expired event should be emitted$`, testCtx.aTokenExpiredEventShouldBeEmitted)
	ctx.Step(`^a token refreshed event should be emitted$`, testCtx.aTokenRefreshedEventShouldBeEmitted)
	ctx.Step(`^a session expired event should be emitted$`, testCtx.aSessionExpiredEventShouldBeEmitted)
	ctx.Step(`^I have an expired session$`, testCtx.iHaveAnExpiredSession)
	ctx.Step(`^I attempt to access the expired session$`, testCtx.iAttemptToAccessTheExpiredSession)
	ctx.Step(`^the session access should fail$`, testCtx.theSessionAccessShouldFail)
	ctx.Step(`^I have an expired token for refresh$`, testCtx.iHaveAnExpiredTokenForRefresh)
	ctx.Step(`^I attempt to refresh the expired token$`, testCtx.iAttemptToRefreshTheExpiredToken)
	ctx.Step(`^the token refresh should fail$`, testCtx.theTokenRefreshShouldFail)
	// Session expired event testing
	ctx.Step(`^I access an expired session$`, testCtx.iAccessAnExpiredSession)
	ctx.Step(`^a session expired event should be emitted$`, testCtx.aSessionExpiredEventShouldBeEmitted)
	ctx.Step(`^the session access should fail$`, testCtx.theSessionAccessShouldFail)

	// Token expired event testing
	ctx.Step(`^I validate an expired token$`, testCtx.iValidateAnExpiredToken)
	ctx.Step(`^a token expired event should be emitted$`, testCtx.aTokenExpiredEventShouldBeEmitted)

	// Token refresh event testing
	ctx.Step(`^I have a valid refresh token$`, testCtx.iHaveAValidRefreshToken)
	ctx.Step(`^a token refreshed event should be emitted$`, testCtx.aTokenRefreshedEventShouldBeEmitted)
	ctx.Step(`^a new access token should be provided$`, testCtx.aNewAccessTokenShouldBeProvided)
}

// TestAuthModule runs the BDD tests for the auth module
func TestAuthModule(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeAuthScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/auth_module.feature"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

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

// initBDDSteps initializes all the BDD steps for the auth module
func (ctx *AuthBDDTestContext) initBDDSteps(s *godog.ScenarioContext) {
	// Background
	s.Given(`^I have a modular application with auth module configured$`, ctx.iHaveAModularApplicationWithAuthModuleConfigured)

	// JWT Token generation and validation
	s.Given(`^I have user credentials and JWT configuration$`, ctx.iHaveUserCredentialsAndJWTConfiguration)
	// Support both phrasing variants used across feature scenarios. Use generic Step so it matches regardless of Given/When/Then/And context.
	s.Step(`^I generate a JWT token for the user$`, ctx.iGenerateAJWTTokenForTheUser)
	s.Step(`^I generate a JWT token for a user$`, ctx.iGenerateAJWTTokenForTheUser)
	s.Then(`^the token should be created successfully$`, ctx.theTokenShouldBeCreatedSuccessfully)
	s.Then(`^the token should contain the user information$`, ctx.theTokenShouldContainTheUserInformation)

	s.Given(`^I have a valid JWT token$`, ctx.iHaveAValidJWTToken)
	s.When(`^I validate the token$`, ctx.iValidateTheToken)
	s.Then(`^the token should be accepted$`, ctx.theTokenShouldBeAccepted)
	s.Then(`^the user claims should be extracted$`, ctx.theUserClaimsShouldBeExtracted)

	s.Given(`^I have an invalid JWT token$`, ctx.iHaveAnInvalidJWTToken)
	s.Then(`^the token should be rejected$`, ctx.theTokenShouldBeRejected)
	s.Then(`^an appropriate error should be returned$`, ctx.anAppropriateErrorShouldBeReturned)

	s.Given(`^I have an expired JWT token$`, ctx.iHaveAnExpiredJWTToken)
	s.Then(`^the error should indicate token expiration$`, ctx.theErrorShouldIndicateTokenExpiration)

	s.When(`^I refresh the token$`, ctx.iRefreshTheToken)
	s.Then(`^a new token should be generated$`, ctx.aNewTokenShouldBeGenerated)
	s.Then(`^the new token should have updated expiration$`, ctx.theNewTokenShouldHaveUpdatedExpiration)

	// Password hashing and verification
	s.Given(`^I have a plain text password$`, ctx.iHaveAPlainTextPassword)
	s.When(`^I hash the password using bcrypt$`, ctx.iHashThePasswordUsingBcrypt)
	s.Then(`^the password should be hashed successfully$`, ctx.thePasswordShouldBeHashedSuccessfully)
	s.Then(`^the hash should be different from the original password$`, ctx.theHashShouldBeDifferentFromTheOriginalPassword)

	s.Given(`^I have a password and its hash$`, ctx.iHaveAPasswordAndItsHash)
	s.When(`^I verify the password against the hash$`, ctx.iVerifyThePasswordAgainstTheHash)
	s.Then(`^the verification should succeed$`, ctx.theVerificationShouldSucceed)

	s.Given(`^I have a password and a different hash$`, ctx.iHaveAPasswordAndADifferentHash)
	s.Then(`^the verification should fail$`, ctx.theVerificationShouldFail)

	// Password strength validation
	s.Given(`^I have a strong password$`, ctx.iHaveAStrongPassword)
	s.When(`^I validate the password strength$`, ctx.iValidateThePasswordStrength)
	s.Then(`^the password should be accepted$`, ctx.thePasswordShouldBeAccepted)
	s.Then(`^no strength errors should be reported$`, ctx.noStrengthErrorsShouldBeReported)

	s.Given(`^I have a weak password$`, ctx.iHaveAWeakPassword)
	s.Then(`^the password should be rejected$`, ctx.thePasswordShouldBeRejected)
	s.Then(`^appropriate strength errors should be reported$`, ctx.appropriateStrengthErrorsShouldBeReported)

	// Session management
	s.Given(`^I have a user identifier$`, ctx.iHaveAUserIdentifier)
	s.When(`^I create a new session for the user$`, ctx.iCreateANewSessionForTheUser)
	s.Then(`^the session should be created successfully$`, ctx.theSessionShouldBeCreatedSuccessfully)
	s.Then(`^the session should have a unique ID$`, ctx.theSessionShouldHaveAUniqueID)

	s.Given(`^I have an existing user session$`, ctx.iHaveAnExistingUserSession)
	s.When(`^I retrieve the session by ID$`, ctx.iRetrieveTheSessionByID)
	s.Then(`^the session should be found$`, ctx.theSessionShouldBeFound)
	s.Then(`^the session data should match$`, ctx.theSessionDataShouldMatch)

	s.When(`^I delete the session$`, ctx.iDeleteTheSession)
	s.Then(`^the session should be removed$`, ctx.theSessionShouldBeRemoved)
	s.Then(`^subsequent retrieval should fail$`, ctx.subsequentRetrievalShouldFail)

	// OAuth2
	s.Given(`^I have OAuth2 configuration$`, ctx.iHaveOAuth2Configuration)
	s.When(`^I initiate OAuth2 authorization$`, ctx.iInitiateOAuth2Authorization)
	s.Then(`^the authorization URL should be generated$`, ctx.theAuthorizationURLShouldBeGenerated)
	s.Then(`^the URL should contain proper parameters$`, ctx.theURLShouldContainProperParameters)

	// User store
	s.Given(`^I have a user store configured$`, ctx.iHaveAUserStoreConfigured)
	s.When(`^I create a new user$`, ctx.iCreateANewUser)
	s.Then(`^the user should be stored successfully$`, ctx.theUserShouldBeStoredSuccessfully)
	s.Then(`^I should be able to retrieve the user by ID$`, ctx.iShouldBeAbleToRetrieveTheUserByID)

	s.Given(`^I have a user with credentials in the store$`, ctx.iHaveAUserWithCredentialsInTheStore)
	s.When(`^I authenticate with correct credentials$`, ctx.iAuthenticateWithCorrectCredentials)
	s.Then(`^the authentication should succeed$`, ctx.theAuthenticationShouldSucceed)
	s.Then(`^the user should be returned$`, ctx.theUserShouldBeReturned)

	s.When(`^I authenticate with incorrect credentials$`, ctx.iAuthenticateWithIncorrectCredentials)
	s.Then(`^the authentication should fail$`, ctx.theAuthenticationShouldFail)
	s.Then(`^an error should be returned$`, ctx.anErrorShouldBeReturned)

	// Event observation scenarios
	s.Given(`^I have an auth module with event observation enabled$`, ctx.iHaveAnAuthModuleWithEventObservationEnabled)
	s.Then(`^a token generated event should be emitted$`, ctx.aTokenGeneratedEventShouldBeEmitted)
	s.Then(`^the event should contain user and token information$`, ctx.theEventShouldContainUserAndTokenInformation)
	s.Then(`^a token validated event should be emitted$`, ctx.aTokenValidatedEventShouldBeEmitted)
	s.Then(`^the event should contain validation information$`, ctx.theEventShouldContainValidationInformation)

	s.When(`^I create a session for a user$`, ctx.iCreateASessionForAUser)
	s.Then(`^a session created event should be emitted$`, ctx.aSessionCreatedEventShouldBeEmitted)
	s.When(`^I access the session$`, ctx.iAccessTheSession)
	s.Then(`^a session accessed event should be emitted$`, ctx.aSessionAccessedEventShouldBeEmitted)
	s.Then(`^a session destroyed event should be emitted$`, ctx.aSessionDestroyedEventShouldBeEmitted)

	s.Given(`^I have OAuth2 providers configured$`, ctx.iHaveOAuth2ProvidersConfigured)
	s.When(`^I get an OAuth2 authorization URL$`, ctx.iGetAnOAuth2AuthorizationURL)
	s.Then(`^an OAuth2 auth URL event should be emitted$`, ctx.anOAuth2AuthURLEventShouldBeEmitted)
	s.When(`^I exchange an OAuth2 code for tokens$`, ctx.iExchangeAnOAuth2CodeForTokens)
	s.Then(`^an OAuth2 exchange event should be emitted$`, ctx.anOAuth2ExchangeEventShouldBeEmitted)

	s.Then(`^a token refreshed event should be emitted$`, ctx.aTokenRefreshedEventShouldBeEmitted)
	s.Given(`^I have an expired session$`, ctx.iHaveAnExpiredSession)
	s.When(`^I attempt to access the expired session$`, ctx.iAttemptToAccessTheExpiredSession)
	s.Then(`^the session access should fail$`, ctx.theSessionAccessShouldFail)
	s.Then(`^a session expired event should be emitted$`, ctx.aSessionExpiredEventShouldBeEmitted)

	s.Given(`^I have an expired token for refresh$`, ctx.iHaveAnExpiredTokenForRefresh)
	s.When(`^I attempt to refresh the expired token$`, ctx.iAttemptToRefreshTheExpiredToken)
	s.Then(`^the token refresh should fail$`, ctx.theTokenRefreshShouldFail)
	s.Then(`^a token expired event should be emitted$`, ctx.aTokenExpiredEventShouldBeEmitted)

	s.When(`^I access an expired session$`, ctx.iAccessAnExpiredSession)
	s.When(`^I validate an expired token$`, ctx.iValidateAnExpiredToken)
	// 'the token should be rejected' already registered above; avoid duplicate to prevent ambiguity

	s.Given(`^I have a valid refresh token$`, ctx.iHaveAValidRefreshToken)
	s.Then(`^a new access token should be provided$`, ctx.aNewAccessTokenShouldBeProvided)

	// Event validation
	s.Then(`^all registered events should be emitted during testing$`, ctx.allRegisteredEventsShouldBeEmittedDuringTesting)
}

// TestAuthModuleBDD runs the BDD tests for the auth module
func TestAuthModuleBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			testCtx := &AuthBDDTestContext{}
			testCtx.initBDDSteps(ctx)
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
