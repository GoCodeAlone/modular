package auth

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cucumber/godog"
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

	// Register all step definitions from other files
	testCtx.registerJWTSteps(ctx)
	testCtx.registerPasswordSteps(ctx)
	testCtx.registerSessionSteps(ctx)
	testCtx.registerOAuthSteps(ctx)
	testCtx.registerUserStoreSteps(ctx)
	testCtx.registerEventSteps(ctx)
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

// initBDDSteps initializes all the BDD steps for the auth module
func (ctx *AuthBDDTestContext) initBDDSteps(s *godog.ScenarioContext) {
	// Background
	s.Given(`^I have a modular application with auth module configured$`, ctx.iHaveAModularApplicationWithAuthModuleConfigured)

	// Register all step definitions from specialized methods
	ctx.registerJWTSteps(s)
	ctx.registerPasswordSteps(s)
	ctx.registerSessionSteps(s)
	ctx.registerOAuthSteps(s)
	ctx.registerUserStoreSteps(s)
	ctx.registerEventSteps(s)
}
