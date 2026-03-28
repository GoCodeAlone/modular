package httpserver

import (
	"testing"

	"github.com/cucumber/godog"
)

// setupCommonBDDScenarios sets up common BDD scenarios that are shared across multiple test files
// This reduces code duplication by centralizing common scenario registrations
func setupCommonBDDScenarios(ctx *godog.ScenarioContext, testCtx *HTTPServerBDDTestContext) {
	// Background step - shared across all scenarios
	ctx.Given(`^I have a modular application with httpserver module configured$`, testCtx.iHaveAModularApplicationWithHTTPServerModuleConfigured)

	// Basic HTTP server configuration scenarios
	ctx.Given(`^I have an HTTP server configuration$`, testCtx.iHaveAnHTTPServerConfiguration)
	ctx.When(`^the httpserver module is initialized$`, testCtx.theHTTPServerModuleIsInitialized)
	ctx.Then(`^the HTTP server service should be available$`, testCtx.theHTTPServerServiceShouldBeAvailable)
	ctx.Then(`^the server should be configured with default settings$`, testCtx.theServerShouldBeConfiguredWithDefaultSettings)

	// Basic server lifecycle scenarios
	ctx.When(`^the HTTP server is started$`, testCtx.theHTTPServerIsStarted)
	ctx.Then(`^the server should listen on the configured address$`, testCtx.theServerShouldListenOnTheConfiguredAddress)
	ctx.Then(`^the server should accept HTTP requests$`, testCtx.theServerShouldAcceptHTTPRequests)
}

// TestHTTPServerModuleBDD runs the complete BDD test suite for the HTTP server module
// This is the main entry point that registers all scenario steps from all themed test files
func TestHTTPServerModuleBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			testCtx := &HTTPServerBDDTestContext{}

			// Set up common scenarios to reduce duplication
			setupCommonBDDScenarios(ctx, testCtx)

			// ============ CORE MODULE FUNCTIONALITY ============
			// Additional specific steps not covered in common scenarios

			// ============ SERVER LIFECYCLE ============
			// Additional lifecycle steps

			// Steps for timeout configuration
			ctx.Given(`^I have an HTTP server with custom timeout settings$`, testCtx.iHaveAnHTTPServerWithCustomTimeoutSettings)
			ctx.When(`^the server processes requests$`, testCtx.theServerProcessesRequests)
			ctx.Then(`^the read timeout should be respected$`, testCtx.theReadTimeoutShouldBeRespected)
			ctx.Then(`^the write timeout should be respected$`, testCtx.theWriteTimeoutShouldBeRespected)
			ctx.Then(`^the idle timeout should be respected$`, testCtx.theIdleTimeoutShouldBeRespected)

			// Steps for graceful shutdown
			ctx.Given(`^I have a running HTTP server$`, testCtx.iHaveARunningHTTPServer)
			ctx.Given(`^I have an HTTP server running$`, testCtx.iHaveAnHTTPServerRunning)
			ctx.When(`^the server shutdown is initiated$`, testCtx.theServerShutdownIsInitiated)
			ctx.Then(`^the server should stop accepting new connections$`, testCtx.theServerShouldStopAcceptingNewConnections)
			ctx.Then(`^existing connections should be allowed to complete$`, testCtx.existingConnectionsShouldBeAllowedToComplete)
			ctx.Then(`^the shutdown should complete within the timeout$`, testCtx.theShutdownShouldCompleteWithinTheTimeout)

			// ============ TLS/SSL HANDLING ============
			// Steps for HTTPS server
			ctx.Given(`^I have an HTTPS server configuration with TLS enabled$`, testCtx.iHaveAnHTTPSServerConfigurationWithTLSEnabled)
			ctx.When(`^the HTTPS server is started$`, testCtx.theHTTPSServerIsStarted)
			ctx.Then(`^the server should listen on the configured TLS port$`, testCtx.theServerShouldListenOnTheConfiguredTLSPort)
			ctx.Then(`^the server should accept HTTPS requests$`, testCtx.theServerShouldAcceptHTTPSRequests)

			// Steps for TLS auto-generation
			ctx.Given(`^I have a TLS configuration without certificate files$`, testCtx.iHaveATLSConfigurationWithoutCertificateFiles)
			ctx.When(`^the HTTPS server is started with auto-generation$`, testCtx.theHTTPSServerIsStartedWithAutoGeneration)
			ctx.Then(`^the server should generate self-signed certificates$`, testCtx.theServerShouldGenerateSelfSignedCertificates)
			ctx.Then(`^the server should use the generated certificates$`, testCtx.theServerShouldUseTheGeneratedCertificates)

			// ============ REQUEST PROCESSING ============
			// Steps for handler registration
			ctx.Given(`^I have an HTTP server service available$`, testCtx.iHaveAnHTTPServerServiceAvailable)
			ctx.When(`^I register custom handlers with the server$`, testCtx.iRegisterCustomHandlersWithTheServer)
			ctx.Then(`^the handlers should be available for requests$`, testCtx.theHandlersShouldBeAvailableForRequests)
			ctx.Then(`^the server should route requests to the correct handlers$`, testCtx.theServerShouldRouteRequestsToTheCorrectHandlers)

			// Steps for middleware
			ctx.Given(`^I have an HTTP server with middleware configured$`, testCtx.iHaveAnHTTPServerWithMiddlewareConfigured)
			ctx.When(`^requests are processed through the server$`, testCtx.requestsAreProcessedThroughTheServer)
			ctx.Then(`^the middleware should be applied to requests$`, testCtx.theMiddlewareShouldBeAppliedToRequests)
			ctx.Then(`^the middleware chain should execute in order$`, testCtx.theMiddlewareChainShouldExecuteInOrder)

			// Steps for error handling
			ctx.When(`^an error occurs during request processing$`, testCtx.anErrorOccursDuringRequestProcessing)
			ctx.Then(`^the server should handle errors gracefully$`, testCtx.theServerShouldHandleErrorsGracefully)
			ctx.Then(`^appropriate error responses should be returned$`, testCtx.appropriateErrorResponsesShouldBeReturned)

			// ============ HEALTH CHECKS & MONITORING ============
			// Steps for health checks
			ctx.Given(`^I have an HTTP server with health checks enabled$`, testCtx.iHaveAnHTTPServerWithHealthChecksEnabled)
			ctx.When(`^I request the health check endpoint$`, testCtx.iRequestTheHealthCheckEndpoint)
			ctx.Then(`^the health check should return server status$`, testCtx.theHealthCheckShouldReturnServerStatus)
			ctx.Then(`^the response should indicate server health$`, testCtx.theResponseShouldIndicateServerHealth)

			// Steps for monitoring
			ctx.Given(`^I have an HTTP server with monitoring enabled$`, testCtx.iHaveAnHTTPServerWithMonitoringEnabled)
			ctx.Then(`^server metrics should be collected$`, testCtx.serverMetricsShouldBeCollected)
			ctx.Then(`^the metrics should include request counts and response times$`, testCtx.theMetricsShouldIncludeRequestCountsAndResponseTimes)

			// ============ EVENT OBSERVATION ============
			// Event observation BDD scenarios
			ctx.Given(`^I have an httpserver with event observation enabled$`, testCtx.iHaveAnHTTPServerWithEventObservationEnabled)
			ctx.When(`^the httpserver module starts$`, func() error { return nil }) // Already started in Given step
			ctx.Then(`^a server started event should be emitted$`, testCtx.aServerStartedEventShouldBeEmitted)
			ctx.Then(`^a config loaded event should be emitted$`, testCtx.aConfigLoadedEventShouldBeEmitted)
			ctx.Then(`^the events should contain server configuration details$`, testCtx.theEventsShouldContainServerConfigurationDetails)

			// TLS configuration events
			ctx.Given(`^I have an httpserver with TLS and event observation enabled$`, testCtx.iHaveAnHTTPServerWithTLSAndEventObservationEnabled)
			ctx.When(`^the TLS server module starts$`, func() error { return nil }) // Already started in Given step
			ctx.Then(`^a TLS enabled event should be emitted$`, testCtx.aTLSEnabledEventShouldBeEmitted)
			ctx.Then(`^a TLS configured event should be emitted$`, testCtx.aTLSConfiguredEventShouldBeEmitted)
			ctx.Then(`^the events should contain TLS configuration details$`, testCtx.theEventsShouldContainTLSConfigurationDetails)

			// Request handling events
			ctx.When(`^the httpserver processes a request$`, testCtx.theHTTPServerProcessesARequest)
			ctx.Then(`^a request received event should be emitted$`, testCtx.aRequestReceivedEventShouldBeEmitted)
			ctx.Then(`^a request handled event should be emitted$`, testCtx.aRequestHandledEventShouldBeEmitted)
			ctx.Then(`^the events should contain request details$`, testCtx.theEventsShouldContainRequestDetails)

			// Event validation (mega-scenario)
			ctx.Then(`^all registered events should be emitted during testing$`, testCtx.allRegisteredEventsShouldBeEmittedDuringTesting)
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
