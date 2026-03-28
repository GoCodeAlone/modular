package httpclient

import (
	"testing"

	"github.com/cucumber/godog"
)

// TestHTTPClientModuleBDD runs the BDD tests for the HTTPClient module
func TestHTTPClientModuleBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			testCtx := &HTTPClientBDDTestContext{}

			// Background
			ctx.Given(`^I have a modular application with httpclient module configured$`, testCtx.iHaveAModularApplicationWithHTTPClientModuleConfigured)

			// Steps for module initialization
			ctx.When(`^the httpclient module is initialized$`, testCtx.theHTTPClientModuleIsInitialized)
			ctx.Then(`^the httpclient service should be available$`, testCtx.theHTTPClientServiceShouldBeAvailable)
			ctx.Then(`^the client should be configured with default settings$`, testCtx.theClientShouldBeConfiguredWithDefaultSettings)

			// Steps for basic requests
			ctx.Given(`^I have an httpclient service available$`, testCtx.iHaveAnHTTPClientServiceAvailable)
			ctx.When(`^I make a GET request to a test endpoint$`, testCtx.iMakeAGETRequestToATestEndpoint)
			ctx.Then(`^the request should be successful$`, testCtx.theRequestShouldBeSuccessful)
			ctx.Then(`^the response should be received$`, testCtx.theResponseShouldBeReceived)

			// Steps for timeout configuration
			ctx.Given(`^I have an httpclient configuration with custom timeouts$`, testCtx.iHaveAnHTTPClientConfigurationWithCustomTimeouts)
			ctx.Then(`^the client should have the configured request timeout$`, testCtx.theClientShouldHaveTheConfiguredRequestTimeout)
			ctx.Then(`^the client should have the configured TLS timeout$`, testCtx.theClientShouldHaveTheConfiguredTLSTimeout)
			ctx.Then(`^the client should have the configured idle connection timeout$`, testCtx.theClientShouldHaveTheConfiguredIdleConnectionTimeout)

			// Steps for connection pooling
			ctx.Given(`^I have an httpclient configuration with connection pooling$`, testCtx.iHaveAnHTTPClientConfigurationWithConnectionPooling)
			ctx.Then(`^the client should have the configured max idle connections$`, testCtx.theClientShouldHaveTheConfiguredMaxIdleConnections)
			ctx.Then(`^the client should have the configured max idle connections per host$`, testCtx.theClientShouldHaveTheConfiguredMaxIdleConnectionsPerHost)
			ctx.Then(`^connection reuse should be enabled$`, testCtx.connectionReuseShouldBeEnabled)

			// Steps for POST requests
			ctx.When(`^I make a POST request with JSON data$`, testCtx.iMakeAPOSTRequestWithJSONData)
			ctx.Then(`^the request body should be sent correctly$`, testCtx.theRequestBodyShouldBeSentCorrectly)

			// Steps for custom headers
			ctx.When(`^I set a request modifier for custom headers$`, testCtx.iSetARequestModifierForCustomHeaders)
			ctx.When(`^I make a request with the modified client$`, testCtx.iMakeARequestWithTheModifiedClient)
			ctx.Then(`^the custom headers should be included in the request$`, testCtx.theCustomHeadersShouldBeIncludedInTheRequest)

			// Steps for authentication
			ctx.When(`^I set a request modifier for authentication$`, testCtx.iSetARequestModifierForAuthentication)
			ctx.When(`^I make a request to a protected endpoint$`, testCtx.iMakeARequestToAProtectedEndpoint)
			ctx.Then(`^the authentication headers should be included$`, testCtx.theAuthenticationHeadersShouldBeIncluded)
			ctx.Then(`^the request should be authenticated$`, testCtx.theRequestShouldBeAuthenticated)

			// Steps for verbose logging
			ctx.Given(`^I have an httpclient configuration with verbose logging enabled$`, testCtx.iHaveAnHTTPClientConfigurationWithVerboseLoggingEnabled)
			ctx.When(`^I make HTTP requests$`, testCtx.iMakeHTTPRequests)
			ctx.Then(`^request and response details should be logged$`, testCtx.requestAndResponseDetailsShouldBeLogged)
			ctx.Then(`^the logs should include headers and timing information$`, testCtx.theLogsShouldIncludeHeadersAndTimingInformation)

			// Steps for timeout handling
			ctx.When(`^I make a request with a custom timeout$`, testCtx.iMakeARequestWithACustomTimeout)
			ctx.When(`^the request takes longer than the timeout$`, testCtx.theRequestTakesLongerThanTheTimeout)
			ctx.Then(`^the request should timeout appropriately$`, testCtx.theRequestShouldTimeoutAppropriately)
			ctx.Then(`^a timeout error should be returned$`, testCtx.aTimeoutErrorShouldBeReturned)

			// Steps for compression
			ctx.Given(`^I have an httpclient configuration with compression enabled$`, testCtx.iHaveAnHTTPClientConfigurationWithCompressionEnabled)
			ctx.When(`^I make requests to endpoints that support compression$`, testCtx.iMakeRequestsToEndpointsThatSupportCompression)
			ctx.Then(`^the client should handle gzip compression$`, testCtx.theClientShouldHandleGzipCompression)
			ctx.Then(`^compressed responses should be automatically decompressed$`, testCtx.compressedResponsesShouldBeAutomaticallyDecompressed)

			// Steps for keep-alive
			ctx.Given(`^I have an httpclient configuration with keep-alive disabled$`, testCtx.iHaveAnHTTPClientConfigurationWithKeepAliveDisabled)
			ctx.Then(`^each request should use a new connection$`, testCtx.eachRequestShouldUseANewConnection)
			ctx.Then(`^connections should not be reused$`, testCtx.connectionsShouldNotBeReused)

			// Steps for error handling
			ctx.When(`^I make a request to an invalid endpoint$`, testCtx.iMakeARequestToAnInvalidEndpoint)
			ctx.Then(`^an appropriate error should be returned$`, testCtx.anAppropriateErrorShouldBeReturned)
			ctx.Then(`^the error should contain meaningful information$`, testCtx.theErrorShouldContainMeaningfulInformation)

			// Steps for retry logic
			ctx.When(`^I make a request that initially fails$`, testCtx.iMakeARequestThatInitiallyFails)
			ctx.When(`^retry logic is configured$`, testCtx.retryLogicIsConfigured)
			ctx.Then(`^the client should retry the request$`, testCtx.theClientShouldRetryTheRequest)
			ctx.Then(`^eventually succeed or return the final error$`, testCtx.eventuallySucceedOrReturnTheFinalError)

			// Event observation BDD scenarios
			ctx.Given(`^I have an httpclient with event observation enabled$`, testCtx.iHaveAnHTTPClientWithEventObservationEnabled)
			ctx.When(`^the httpclient module starts$`, func() error { return nil }) // Already started in Given step
			ctx.Then(`^a client started event should be emitted$`, testCtx.aClientStartedEventShouldBeEmitted)
			ctx.Then(`^a config loaded event should be emitted$`, testCtx.aConfigLoadedEventShouldBeEmitted)
			ctx.Then(`^the events should contain client configuration details$`, testCtx.theEventsShouldContainClientConfigurationDetails)

			// Request modification events
			ctx.When(`^I add a request modifier$`, testCtx.iAddARequestModifier)
			ctx.Then(`^a modifier added event should be emitted$`, testCtx.aModifierAddedEventShouldBeEmitted)
			ctx.When(`^I remove a request modifier$`, testCtx.iRemoveARequestModifier)
			ctx.Then(`^a modifier removed event should be emitted$`, testCtx.aModifierRemovedEventShouldBeEmitted)

			// Timeout change events
			ctx.When(`^I change the client timeout$`, testCtx.iChangeTheClientTimeout)
			ctx.Then(`^a timeout changed event should be emitted$`, testCtx.aTimeoutChangedEventShouldBeEmitted)
			ctx.Then(`^the event should contain the new timeout value$`, testCtx.theEventShouldContainTheNewTimeoutValue)
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
