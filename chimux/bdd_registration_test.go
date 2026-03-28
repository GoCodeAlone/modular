package chimux

import (
	"testing"

	"github.com/cucumber/godog"
)

// InitializeChiMuxScenario initializes the BDD scenario steps
func InitializeChiMuxScenario(ctx *godog.ScenarioContext) {
	testCtx := &ChiMuxBDDTestContext{}

	// Background
	ctx.Step(`^I have a modular application with chimux module configured$`, testCtx.iHaveAModularApplicationWithChimuxModuleConfigured)

	// Initialization steps
	ctx.Step(`^the chimux module is initialized$`, testCtx.theChimuxModuleIsInitialized)
	ctx.Step(`^the router service should be available$`, testCtx.theRouterServiceShouldBeAvailable)
	ctx.Step(`^the Chi router service should be available$`, testCtx.theChiRouterServiceShouldBeAvailable)
	ctx.Step(`^the basic router service should be available$`, testCtx.theBasicRouterServiceShouldBeAvailable)

	// Service availability
	ctx.Step(`^I have a router service available$`, testCtx.iHaveARouterServiceAvailable)
	ctx.Step(`^I have a basic router service available$`, testCtx.iHaveABasicRouterServiceAvailable)
	ctx.Step(`^I have access to the Chi router service$`, testCtx.iHaveAccessToTheChiRouterService)

	// Route registration
	ctx.Step(`^I register a GET route "([^"]*)" with handler$`, testCtx.iRegisterAGETRouteWithHandler)
	ctx.Step(`^I register a POST route "([^"]*)" with handler$`, testCtx.iRegisterAPOSTRouteWithHandler)
	ctx.Step(`^the routes should be registered successfully$`, testCtx.theRoutesShouldBeRegisteredSuccessfully)

	// CORS configuration
	ctx.Step(`^I have a chimux configuration with CORS settings$`, testCtx.iHaveAChimuxConfigurationWithCORSSettings)
	ctx.Step(`^the chimux module is initialized with CORS$`, testCtx.theChimuxModuleIsInitializedWithCORS)
	ctx.Step(`^the CORS middleware should be configured$`, testCtx.theCORSMiddlewareShouldBeConfigured)
	ctx.Step(`^allowed origins should include the configured values$`, testCtx.allowedOriginsShouldIncludeTheConfiguredValues)

	// Middleware
	ctx.Step(`^I have middleware provider services available$`, testCtx.iHaveMiddlewareProviderServicesAvailable)
	ctx.Step(`^the chimux module discovers middleware providers$`, testCtx.theChimuxModuleDiscoversMiddlewareProviders)
	ctx.Step(`^the middleware should be applied to the router$`, testCtx.theMiddlewareShouldBeAppliedToTheRouter)
	ctx.Step(`^requests should pass through the middleware chain$`, testCtx.requestsShouldPassThroughTheMiddlewareChain)

	// Base path
	ctx.Step(`^I have a chimux configuration with base path "([^"]*)"$`, testCtx.iHaveAChimuxConfigurationWithBasePath)
	ctx.Step(`^I register routes with the configured base path$`, testCtx.iRegisterRoutesWithTheConfiguredBasePath)
	ctx.Step(`^all routes should be prefixed with the base path$`, testCtx.allRoutesShouldBePrefixedWithTheBasePath)

	// Timeout
	ctx.Step(`^I have a chimux configuration with timeout settings$`, testCtx.iHaveAChimuxConfigurationWithTimeoutSettings)
	ctx.Step(`^the chimux module applies timeout configuration$`, testCtx.theChimuxModuleAppliesTimeoutConfiguration)
	ctx.Step(`^the timeout middleware should be configured$`, testCtx.theTimeoutMiddlewareShouldBeConfigured)
	ctx.Step(`^requests should respect the timeout settings$`, testCtx.requestsShouldRespectTheTimeoutSettings)

	// Chi-specific features
	ctx.Step(`^I use Chi-specific routing features$`, testCtx.iUseChiSpecificRoutingFeatures)
	ctx.Step(`^I should be able to create route groups$`, testCtx.iShouldBeAbleToCreateRouteGroups)
	ctx.Step(`^I should be able to mount sub-routers$`, testCtx.iShouldBeAbleToMountSubRouters)

	// HTTP methods
	ctx.Step(`^I register routes for different HTTP methods$`, testCtx.iRegisterRoutesForDifferentHTTPMethods)
	ctx.Step(`^GET routes should be handled correctly$`, testCtx.gETRoutesShouldBeHandledCorrectly)
	ctx.Step(`^POST routes should be handled correctly$`, testCtx.pOSTRoutesShouldBeHandledCorrectly)
	ctx.Step(`^PUT routes should be handled correctly$`, testCtx.pUTRoutesShouldBeHandledCorrectly)
	ctx.Step(`^DELETE routes should be handled correctly$`, testCtx.dELETERoutesShouldBeHandledCorrectly)

	// Route parameters
	ctx.Step(`^I register parameterized routes$`, testCtx.iRegisterParameterizedRoutes)
	ctx.Step(`^route parameters should be extracted correctly$`, testCtx.routeParametersShouldBeExtractedCorrectly)
	ctx.Step(`^wildcard routes should match appropriately$`, testCtx.wildcardRoutesShouldMatchAppropriately)

	// Middleware ordering
	ctx.Step(`^I have multiple middleware providers$`, testCtx.iHaveMultipleMiddlewareProviders)
	ctx.Step(`^middleware is applied to the router$`, testCtx.middlewareIsAppliedToTheRouter)
	ctx.Step(`^middleware should be applied in the correct order$`, testCtx.middlewareShouldBeAppliedInTheCorrectOrder)
	ctx.Step(`^request processing should follow the middleware chain$`, testCtx.requestProcessingShouldFollowTheMiddlewareChain)

	// Event observation steps
	ctx.Step(`^I have a chimux module with event observation enabled$`, testCtx.iHaveAChimuxModuleWithEventObservationEnabled)
	ctx.Step(`^a config loaded event should be emitted$`, testCtx.aConfigLoadedEventShouldBeEmitted)
	ctx.Step(`^a router created event should be emitted$`, testCtx.aRouterCreatedEventShouldBeEmitted)
	ctx.Step(`^a module started event should be emitted$`, testCtx.aModuleStartedEventShouldBeEmitted)
	ctx.Step(`^route registered events should be emitted$`, testCtx.routeRegisteredEventsShouldBeEmitted)
	ctx.Step(`^the events should contain the correct route information$`, testCtx.theEventsShouldContainTheCorrectRouteInformation)
	ctx.Step(`^a CORS configured event should be emitted$`, testCtx.aCORSConfiguredEventShouldBeEmitted)
	ctx.Step(`^a CORS enabled event should be emitted$`, testCtx.aCORSEnabledEventShouldBeEmitted)
	ctx.Step(`^middleware added events should be emitted$`, testCtx.middlewareAddedEventsShouldBeEmitted)
	ctx.Step(`^the events should contain middleware information$`, testCtx.theEventsShouldContainMiddlewareInformation)

	// New event observation steps for missing events
	ctx.Step(`^I have a chimux configuration with validation requirements$`, testCtx.iHaveAChimuxConfigurationWithValidationRequirements)
	ctx.Step(`^the chimux module validates the configuration$`, testCtx.theChimuxModuleValidatesTheConfiguration)
	ctx.Step(`^a config validated event should be emitted$`, testCtx.aConfigValidatedEventShouldBeEmitted)
	ctx.Step(`^the event should contain validation results$`, testCtx.theEventShouldContainValidationResults)
	ctx.Step(`^the router is started$`, testCtx.theRouterIsStarted)
	ctx.Step(`^a router started event should be emitted$`, testCtx.aRouterStartedEventShouldBeEmitted)
	ctx.Step(`^the router is stopped$`, testCtx.theRouterIsStopped)
	ctx.Step(`^a router stopped event should be emitted$`, testCtx.aRouterStoppedEventShouldBeEmitted)
	ctx.Step(`^I have registered routes$`, testCtx.iHaveRegisteredRoutes)
	ctx.Step(`^I remove a route from the router$`, testCtx.iRemoveARouteFromTheRouter)
	ctx.Step(`^a route removed event should be emitted$`, testCtx.aRouteRemovedEventShouldBeEmitted)
	ctx.Step(`^the event should contain the removed route information$`, testCtx.theEventShouldContainTheRemovedRouteInformation)
	ctx.Step(`^I have middleware applied to the router$`, testCtx.iHaveMiddlewareAppliedToTheRouter)
	ctx.Step(`^I remove middleware from the router$`, testCtx.iRemoveMiddlewareFromTheRouter)
	ctx.Step(`^a middleware removed event should be emitted$`, testCtx.aMiddlewareRemovedEventShouldBeEmitted)
	ctx.Step(`^the event should contain the removed middleware information$`, testCtx.theEventShouldContainTheRemovedMiddlewareInformation)
	ctx.Step(`^the chimux module is started$`, testCtx.theChimuxModuleIsStarted)
	ctx.Step(`^the chimux module is stopped$`, testCtx.theChimuxModuleIsStopped)
	ctx.Step(`^a module stopped event should be emitted$`, testCtx.aModuleStoppedEventShouldBeEmitted)
	ctx.Step(`^the event should contain module stop information$`, testCtx.theEventShouldContainModuleStopInformation)
	ctx.Step(`^I have routes registered for request handling$`, testCtx.iHaveRoutesRegisteredForRequestHandling)
	ctx.Step(`^I make an HTTP request to the router$`, testCtx.iMakeAnHTTPRequestToTheRouter)
	ctx.Step(`^a request received event should be emitted$`, testCtx.aRequestReceivedEventShouldBeEmitted)
	ctx.Step(`^a request processed event should be emitted$`, testCtx.aRequestProcessedEventShouldBeEmitted)
	ctx.Step(`^the events should contain request processing information$`, testCtx.theEventsShouldContainRequestProcessingInformation)
	ctx.Step(`^I have routes that can fail$`, testCtx.iHaveRoutesThatCanFail)
	ctx.Step(`^I make a request that causes a failure$`, testCtx.iMakeARequestThatCausesAFailure)
	ctx.Step(`^a request failed event should be emitted$`, testCtx.aRequestFailedEventShouldBeEmitted)
	ctx.Step(`^the event should contain failure information$`, testCtx.theEventShouldContainFailureInformation)
}

// TestChiMuxModule runs the BDD tests for the chimux module
func TestChiMuxModule(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeChiMuxScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/chimux_module.feature"},
			TestingT: t,
			Strict:   true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
