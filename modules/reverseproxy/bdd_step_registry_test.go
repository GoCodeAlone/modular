package reverseproxy

import "github.com/cucumber/godog"

// registerAllStepDefinitions registers all step definitions from all BDD files
// This function consolidates step registrations from all split BDD test files
func registerAllStepDefinitions(s *godog.ScenarioContext, ctx *ReverseProxyBDDTestContext) {
	// Background step - common setup
	s.Given(`^I have a modular application with reverse proxy module configured$`, ctx.iHaveAModularApplicationWithReverseProxyModuleConfigured)

	// Basic Module Scenarios (from bdd_core_module_test.go)
	s.When(`^the reverse proxy module is initialized$`, ctx.theReverseProxyModuleIsInitialized)
	s.Then(`^the proxy service should be available$`, ctx.theProxyServiceShouldBeAvailable)
	s.Then(`^the module should be ready to route requests$`, ctx.theModuleShouldBeReadyToRouteRequests)

	// Single Backend Scenarios (from bdd_routing_loadbalancing_test.go)
	s.Given(`^I have a reverse proxy configured with a single backend$`, ctx.iHaveAReverseProxyConfiguredWithASingleBackend)
	s.When(`^I send a request to the proxy$`, ctx.iSendARequestToTheProxy)
	s.Then(`^the request should be forwarded to the backend$`, ctx.theRequestShouldBeForwardedToTheBackend)
	s.Then(`^the response should be returned to the client$`, ctx.theResponseShouldBeReturnedToTheClient)

	// Multiple Backend Scenarios (from bdd_routing_loadbalancing_test.go)
	s.Given(`^I have a reverse proxy configured with multiple backends$`, ctx.iHaveAReverseProxyConfiguredWithMultipleBackends)
	s.When(`^I send multiple requests to the proxy$`, ctx.iSendMultipleRequestsToTheProxy)
	s.Then(`^requests should be distributed across all backends$`, ctx.requestsShouldBeDistributedAcrossAllBackends)
	s.Then(`^load balancing should be applied$`, ctx.loadBalancingShouldBeApplied)

	// Health Check Scenarios (from bdd_health_circuit_test.go)
	s.Given(`^I have a reverse proxy with health checks enabled$`, ctx.iHaveAReverseProxyWithHealthChecksEnabled)
	s.When(`^a backend becomes unavailable$`, ctx.aBackendBecomesUnavailable)
	s.Then(`^the proxy should detect the failure$`, ctx.theProxyShouldDetectTheFailure)
	s.Then(`^route traffic only to healthy backends$`, ctx.routeTrafficOnlyToHealthyBackends)

	// Circuit Breaker Scenarios (from bdd_health_circuit_test.go)
	s.Given(`^I have a reverse proxy with circuit breaker enabled$`, ctx.iHaveAReverseProxyWithCircuitBreakerEnabled)
	s.When(`^a backend fails repeatedly$`, ctx.aBackendFailsRepeatedly)
	s.Then(`^the circuit breaker should open$`, ctx.theCircuitBreakerShouldOpen)
	s.Then(`^requests should be handled gracefully$`, ctx.requestsShouldBeHandledGracefully)

	// Caching Scenarios (from bdd_caching_tenant_test.go)
	s.Given(`^I have a reverse proxy with caching enabled$`, ctx.iHaveAReverseProxyWithCachingEnabled)
	s.When(`^I send the same request multiple times$`, ctx.iSendTheSameRequestMultipleTimes)
	s.Then(`^the first request should hit the backend$`, ctx.theFirstRequestShouldHitTheBackend)
	s.Then(`^subsequent requests should be served from cache$`, ctx.subsequentRequestsShouldBeServedFromCache)

	// Tenant-Aware Scenarios (from bdd_caching_tenant_test.go)
	s.Given(`^I have a tenant-aware reverse proxy configured$`, ctx.iHaveATenantAwareReverseProxyConfigured)
	s.When(`^I send requests with different tenant contexts$`, ctx.iSendRequestsWithDifferentTenantContexts)
	s.Then(`^requests should be routed based on tenant configuration$`, ctx.requestsShouldBeRoutedBasedOnTenantConfiguration)
	s.Then(`^tenant isolation should be maintained$`, ctx.tenantIsolationShouldBeMaintained)

	// Composite Response Scenarios (from bdd_caching_tenant_test.go)
	s.Given(`^I have a reverse proxy configured for composite responses$`, ctx.iHaveAReverseProxyConfiguredForCompositeResponses)
	s.When(`^I send a request that requires multiple backend calls$`, ctx.iSendARequestThatRequiresMultipleBackendCalls)
	s.Then(`^the proxy should call all required backends$`, ctx.theProxyShouldCallAllRequiredBackends)
	s.Then(`^combine the responses into a single response$`, ctx.combineTheResponsesIntoASingleResponse)

	// Request Transformation Scenarios (from bdd_caching_tenant_test.go)
	s.Given(`^I have a reverse proxy with request transformation configured$`, ctx.iHaveAReverseProxyWithRequestTransformationConfigured)
	s.Then(`^the request should be transformed before forwarding$`, ctx.theRequestShouldBeTransformedBeforeForwarding)
	s.Then(`^the backend should receive the transformed request$`, ctx.theBackendShouldReceiveTheTransformedRequest)

	// Graceful Shutdown Scenarios (from bdd_shutdown_performance_test.go)
	s.Given(`^I have an active reverse proxy with ongoing requests$`, ctx.iHaveAnActiveReverseProxyWithOngoingRequests)
	s.When(`^the module is stopped$`, ctx.theModuleIsStopped)
	s.Then(`^ongoing requests should be completed$`, ctx.ongoingRequestsShouldBeCompleted)
	s.Then(`^new requests should be rejected gracefully$`, ctx.newRequestsShouldBeRejectedGracefully)

	// Event observation scenarios (from bdd_events_test.go)
	s.Given(`^I have a reverse proxy with event observation enabled$`, ctx.iHaveAReverseProxyWithEventObservationEnabled)
	s.When(`^the reverse proxy module starts$`, ctx.theReverseProxyModuleStarts)
	s.Then(`^a proxy created event should be emitted$`, ctx.aProxyCreatedEventShouldBeEmitted)
	s.Then(`^a proxy started event should be emitted$`, ctx.aProxyStartedEventShouldBeEmitted)
	s.Then(`^a module started event should be emitted$`, ctx.aModuleStartedEventShouldBeEmitted)
	s.Then(`^the events should contain proxy configuration details$`, ctx.theEventsShouldContainProxyConfigurationDetails)
	s.When(`^the reverse proxy module stops$`, ctx.theReverseProxyModuleStops)
	s.Then(`^a proxy stopped event should be emitted$`, ctx.aProxyStoppedEventShouldBeEmitted)
	s.Then(`^a module stopped event should be emitted$`, ctx.aModuleStoppedEventShouldBeEmitted)

	// Request routing events
	s.Given(`^I have a backend service configured$`, ctx.iHaveABackendServiceConfigured)
	s.When(`^I send a request to the reverse proxy$`, ctx.iSendARequestToTheReverseProxy)
	s.Then(`^a request received event should be emitted$`, ctx.aRequestReceivedEventShouldBeEmitted)
	s.Then(`^the event should contain request details$`, ctx.theEventShouldContainRequestDetails)
	s.When(`^the request is successfully proxied to the backend$`, ctx.theRequestIsSuccessfullyProxiedToTheBackend)
	s.Then(`^a request proxied event should be emitted$`, ctx.aRequestProxiedEventShouldBeEmitted)
	s.Then(`^the event should contain backend and response details$`, ctx.theEventShouldContainBackendAndResponseDetails)

	// Request failure events
	s.Given(`^I have an unavailable backend service configured$`, ctx.iHaveAnUnavailableBackendServiceConfigured)
	s.When(`^the request fails to reach the backend$`, ctx.theRequestFailsToReachTheBackend)
	s.Then(`^a request failed event should be emitted$`, ctx.aRequestFailedEventShouldBeEmitted)
	s.Then(`^the event should contain error details$`, ctx.theEventShouldContainErrorDetails)

	// Metrics and Debug Scenarios (from bdd_metrics_debug_test.go)
	s.Given(`^I have a reverse proxy with metrics enabled$`, ctx.iHaveAReverseProxyWithMetricsEnabled)
	s.When(`^requests are processed through the proxy$`, ctx.whenRequestsAreProcessedThroughTheProxy)
	s.Then(`^metrics should be collected and exposed$`, ctx.thenMetricsShouldBeCollectedAndExposed)
	s.Then(`^metric values should reflect proxy activity$`, ctx.metricValuesShouldReflectProxyActivity)

	// Custom metrics endpoint
	s.Given(`^I have a reverse proxy with custom metrics endpoint$`, ctx.iHaveAReverseProxyWithCustomMetricsEndpoint)
	s.Given(`^I have a custom metrics endpoint configured$`, ctx.iHaveACustomMetricsEndpointConfigured)
	s.When(`^the metrics endpoint is accessed$`, ctx.whenTheMetricsEndpointIsAccessed)
	s.Then(`^metrics should be available at the configured path$`, ctx.thenMetricsShouldBeAvailableAtTheConfiguredPath)
	s.Then(`^metrics data should be properly formatted$`, ctx.andMetricsDataShouldBeProperlyFormatted)

	// Debug endpoints
	s.Given(`^I have a reverse proxy with debug endpoints enabled$`, ctx.iHaveAReverseProxyWithDebugEndpointsEnabled)
	s.Given(`^I have a debug endpoints enabled reverse proxy$`, ctx.iHaveADebugEndpointsEnabledReverseProxy)
	s.When(`^debug endpoints are accessed$`, ctx.whenDebugEndpointsAreAccessed)
	s.Then(`^configuration information should be exposed$`, ctx.thenConfigurationInformationShouldBeExposed)
	s.Then(`^debug data should be properly formatted$`, ctx.andDebugDataShouldBeProperlyFormatted)

	// Specific debug endpoint scenarios
	s.When(`^the debug info endpoint is accessed$`, ctx.theDebugInfoEndpointIsAccessed)
	s.Then(`^general proxy information should be returned$`, ctx.generalProxyInformationShouldBeReturned)
	s.Then(`^configuration details should be included$`, ctx.configurationDetailsShouldBeIncluded)

	s.When(`^the debug backends endpoint is accessed$`, ctx.theDebugBackendsEndpointIsAccessed)
	s.Then(`^backend configuration should be returned$`, ctx.backendConfigurationShouldBeReturned)
	s.Then(`^backend health status should be included$`, ctx.backendHealthStatusShouldBeIncluded)

	// Debug endpoints with feature flags
	s.Given(`^I have a reverse proxy with debug endpoints and feature flags enabled$`, ctx.iHaveADebugEndpointsAndFeatureFlagsEnabledReverseProxy)
	s.When(`^the debug flags endpoint is accessed$`, ctx.theDebugFlagsEndpointIsAccessed)
	s.Then(`^current feature flag states should be returned$`, ctx.currentFeatureFlagStatesShouldBeReturned)
	s.Then(`^tenant-specific flags should be included$`, ctx.tenantSpecificFlagsShouldBeIncluded)

	// Debug endpoints with circuit breakers
	s.Given(`^I have a reverse proxy with debug endpoints and circuit breakers enabled$`, ctx.iHaveADebugEndpointsAndCircuitBreakersEnabledReverseProxy)
	s.When(`^the debug circuit breakers endpoint is accessed$`, ctx.theDebugCircuitBreakersEndpointIsAccessed)
	s.Then(`^circuit breaker states should be returned$`, ctx.circuitBreakerStatesShouldBeReturned)
	s.Then(`^circuit breaker metrics should be included$`, ctx.circuitBreakerMetricsShouldBeIncluded)

	// Debug endpoints with health checks
	s.Given(`^I have a reverse proxy with debug endpoints and health checks enabled$`, ctx.iHaveADebugEndpointsAndHealthChecksEnabledReverseProxy)
	s.When(`^the debug health checks endpoint is accessed$`, ctx.theDebugHealthChecksEndpointIsAccessed)
	s.Then(`^health check status should be returned$`, ctx.healthCheckStatusShouldBeReturned)
	s.Then(`^health check history should be included$`, ctx.healthCheckHistoryShouldBeIncluded)

	// Feature Flag steps
	s.Step(`^feature flags are enabled$`, ctx.featureFlagsAreEnabled)
	s.Given(`^I have multiple evaluators implementing FeatureFlagEvaluator with different service names$`, ctx.iHaveMultipleEvaluatorsImplementingFeatureFlagEvaluatorWithDifferentServiceNames)
	s.Step(`^the evaluators are registered with names "customEvaluator", "remoteFlags", and "rules-engine"$`, ctx.theEvaluatorsAreRegisteredWithNames)
	s.When(`^the feature flag aggregator discovers evaluators$`, ctx.theFeatureFlagAggregatorDiscoversEvaluators)
	s.Then(`^all evaluators should be discovered regardless of their service names$`, ctx.allEvaluatorsShouldBeDiscoveredRegardlessOfTheirServiceNames)
	s.Step(`^each evaluator should be assigned a unique internal name$`, ctx.eachEvaluatorShouldBeAssignedAUniqueInternalName)
	s.Given(`^I have three evaluators with weights 10, 50, and 100$`, ctx.iHaveThreeEvaluatorsWithWeights)
	s.When(`^a feature flag is evaluated$`, ctx.aFeatureFlagIsEvaluated)
	s.Then(`^evaluators should be called in ascending weight order$`, ctx.evaluatorsShouldBeCalledInAscendingWeightOrder)
	s.Step(`^the first evaluator returning a decision should determine the result$`, ctx.theFirstEvaluatorReturningADecisionShouldDetermineTheResult)
	s.Given(`^I have two evaluators registered with the same service name "evaluator"$`, ctx.iHaveTwoEvaluatorsRegisteredWithTheSameServiceName)
	s.Then(`^unique names should be automatically generated$`, ctx.uniqueNamesShouldBeAutomaticallyGenerated)
	s.Step(`^both evaluators should be available for evaluation$`, ctx.bothEvaluatorsShouldBeAvailableForEvaluation)
	s.Given(`^I have external evaluators that return ErrNoDecision$`, ctx.iHaveExternalEvaluatorsThatReturnErrNoDecision)
	s.Then(`^the built-in file evaluator should be called as fallback$`, ctx.theBuiltInFileEvaluatorShouldBeCalledAsFallback)
	s.Step(`^it should have the lowest priority \(weight 1000\)$`, ctx.itShouldHaveTheLowestPriority)
	s.Given(`^I have an external evaluator with weight 50$`, ctx.iHaveAnExternalEvaluatorWithWeight50)
	s.Step(`^the external evaluator returns true for flag "test-flag"$`, ctx.theExternalEvaluatorReturnsTrueForFlag)
	s.When(`^I evaluate flag "test-flag"$`, ctx.iEvaluateFlag)
	s.Then(`^the external evaluator result should be returned$`, ctx.theExternalEvaluatorResultShouldBeReturned)
	s.Step(`^the file evaluator should not be called$`, ctx.theFileEvaluatorShouldNotBeCalled)
	s.Given(`^I have two evaluators where the first returns ErrNoDecision$`, ctx.iHaveTwoEvaluatorsWhereTheFirstReturnsErrNoDecision)
	s.Step(`^the second evaluator returns true for flag "test-flag"$`, ctx.theSecondEvaluatorReturnsTrueForFlag)
	s.Then(`^evaluation should continue to the second evaluator$`, ctx.evaluationShouldContinueToTheSecondEvaluator)
	s.Step(`^the result should be true$`, ctx.theResultShouldBeTrue)
	s.Given(`^I have two evaluators where the first returns ErrEvaluatorFatal$`, ctx.iHaveTwoEvaluatorsWhereTheFirstReturnsErrEvaluatorFatal)
	s.Then(`^evaluation should stop immediately$`, ctx.evaluationShouldStopImmediately)
	s.Step(`^no further evaluators should be called$`, ctx.noFurtherEvaluatorsShouldBeCalled)
	s.Given(`^the aggregator is registered as "featureFlagEvaluator"$`, ctx.theAggregatorIsRegisteredAs)
	s.Step(`^external evaluators are also registered$`, ctx.externalEvaluatorsAreAlsoRegistered)
	s.When(`^evaluator discovery runs$`, ctx.evaluatorDiscoveryRuns)
	s.Then(`^the aggregator should not discover itself$`, ctx.theAggregatorShouldNotDiscoverItself)
	s.Step(`^only external evaluators should be included$`, ctx.onlyExternalEvaluatorsShouldBeIncluded)
	s.Given(`^module A registers an evaluator as "moduleA.flags"$`, ctx.moduleARegistersAnEvaluatorAs)
	s.Step(`^module B registers an evaluator as "moduleB.flags"$`, ctx.moduleBRegistersAnEvaluatorAs)
	s.Then(`^both evaluators should be discovered$`, ctx.bothEvaluatorsShouldBeDiscovered)
	s.Step(`^their unique names should reflect their origins$`, ctx.theirUniqueNamesShouldReflectTheirOrigins)

	// Path and Header Rewriting Scenarios (from bdd_advanced_routing_test.go)
	s.Given(`^I have a reverse proxy with per-backend path rewriting configured$`, ctx.iHaveAReverseProxyWithPerBackendPathRewritingConfigured)
	s.When(`^requests are routed to different backends$`, ctx.requestsAreRoutedToDifferentBackends)
	s.Then(`^paths should be rewritten according to backend configuration$`, ctx.pathsShouldBeRewrittenAccordingToBackendConfiguration)
	s.Then(`^original paths should be properly transformed$`, ctx.originalPathsShouldBeProperlyTransformed)

	s.Given(`^I have a reverse proxy with per-endpoint path rewriting configured$`, ctx.iHaveAReverseProxyWithPerEndpointPathRewritingConfigured)
	s.When(`^requests match specific endpoint patterns$`, ctx.requestsMatchSpecificEndpointPatterns)
	s.Then(`^paths should be rewritten according to endpoint configuration$`, ctx.pathsShouldBeRewrittenAccordingToEndpointConfiguration)
	s.Then(`^endpoint-specific rules should override backend rules$`, ctx.endpointSpecificRulesShouldOverrideBackendRules)

	// Hostname Handling Scenarios (from bdd_advanced_routing_test.go)
	s.Given(`^I have a reverse proxy with different hostname handling modes configured$`, ctx.iHaveAReverseProxyWithDifferentHostnameHandlingModesConfigured)
	s.When(`^requests are forwarded to backends$`, ctx.requestsAreForwardedToBackends)
	s.Then(`^Host headers should be handled according to configuration$`, ctx.hostHeadersShouldBeHandledAccordingToConfiguration)
	s.Then(`^custom hostnames should be applied when specified$`, ctx.customHostnamesShouldBeAppliedWhenSpecified)

	// Header Rewriting Scenarios (from bdd_advanced_routing_test.go)
	s.Given(`^I have a reverse proxy with header rewriting configured$`, ctx.iHaveAReverseProxyWithHeaderRewritingConfigured)
	s.Then(`^specified headers should be added or modified$`, ctx.specifiedHeadersShouldBeAddedOrModified)
	s.Then(`^specified headers should be removed from requests$`, ctx.specifiedHeadersShouldBeRemovedFromRequests)

	// Timeout and Error Handling Scenarios - only unique implementations from bdd_advanced_routing_test.go
	// Note: Main implementations are in bdd_advanced_circuit_cache_test.go which take precedence
	s.When(`^a long-running request is made$`, ctx.aLongRunningRequestIsMade)
	s.Then(`^the request should timeout according to global configuration$`, ctx.theRequestShouldTimeoutAccordingToGlobalConfiguration)
	s.When(`^requests are made to different routes$`, ctx.requestsAreMadeToDifferentRoutes)
	s.Then(`^timeouts should be applied per route configuration$`, ctx.timeoutsShouldBeAppliedPerRouteConfiguration)

	s.Given(`^I have a reverse proxy with error response handling configured$`, ctx.iHaveAReverseProxyWithErrorResponseHandlingConfigured)
	s.When(`^a backend returns error responses$`, ctx.aBackendReturnsErrorResponses)
	s.Then(`^error handling should be applied according to configuration$`, ctx.errorHandlingShouldBeAppliedAccordingToConfiguration)

	s.Given(`^I have a reverse proxy with connection failure handling configured$`, ctx.iHaveAReverseProxyWithConnectionFailureHandlingConfigured)

	// Advanced Health Check Scenarios - DNS Resolution
	s.Given(`^I have a reverse proxy with health checks configured for DNS resolution$`, ctx.iHaveAReverseProxyWithHealthChecksConfiguredForDNSResolution)
	s.When(`^health checks are performed$`, ctx.whenHealthChecksArePerformed)
	s.Then(`^DNS resolution should be validated$`, ctx.thenDNSResolutionShouldBeValidated)
	s.Then(`^unhealthy backends should be marked as down$`, ctx.andUnhealthyBackendsShouldBeMarkedAsDown)

	// Custom Health Endpoints Per Backend
	s.Given(`^I have a reverse proxy with custom health endpoints configured$`, ctx.iHaveAReverseProxyWithCustomHealthEndpointsConfigured)
	s.When(`^health checks are performed on different backends$`, ctx.whenHealthChecksArePerformedOnDifferentBackends)
	s.Then(`^each backend should be checked at its custom endpoint$`, ctx.thenEachBackendShouldBeCheckedAtItsCustomEndpoint)
	s.Then(`^health status should be properly tracked$`, ctx.andHealthStatusShouldBeProperlyTracked)

	// Per-Backend Health Check Configuration
	s.Given(`^I have a reverse proxy with per-backend health check settings$`, ctx.iHaveAReverseProxyWithPerBackendHealthCheckSettings)
	s.When(`^health checks run with different intervals and timeouts$`, ctx.whenHealthChecksRunWithDifferentIntervalsAndTimeouts)
	s.Then(`^each backend should use its specific configuration$`, ctx.thenEachBackendShouldUseItsSpecificConfiguration)
	s.Then(`^health check timing should be respected$`, ctx.andHealthCheckTimingShouldBeRespected)

	// Recent Request Threshold Behavior
	s.Given(`^I have a reverse proxy with recent request threshold configured$`, ctx.iHaveAReverseProxyWithRecentRequestThresholdConfigured)
	s.When(`^requests are made within the threshold window$`, ctx.whenRequestsAreMadeWithinTheThresholdWindow)
	s.Then(`^health checks should be skipped for recently used backends$`, ctx.thenHealthChecksShouldBeSkippedForRecentlyUsedBackends)
	s.Then(`^health checks should resume after threshold expires$`, ctx.andHealthChecksShouldResumeAfterThresholdExpires)

	// Health Check Expected Status Codes
	s.Given(`^I have a reverse proxy with custom expected status codes$`, ctx.iHaveAReverseProxyWithCustomExpectedStatusCodes)
	s.When(`^backends return various HTTP status codes$`, ctx.whenBackendsReturnVariousHTTPStatusCodes)
	s.Then(`^only configured status codes should be considered healthy$`, ctx.thenOnlyConfiguredStatusCodesShouldBeConsideredHealthy)
	s.Then(`^other status codes should mark backends as unhealthy$`, ctx.andOtherStatusCodesShouldMarkBackendsAsUnhealthy)

	// Event System Steps (from bdd_event_system_scenarios_test.go)
	// Note: Other event system steps are implemented in various BDD files (bdd_events_test.go, etc.)
	s.Then(`^circuit breaker behavior should be isolated per backend$`, ctx.circuitBreakerBehaviorShouldBeIsolatedPerBackend)

	// Circuit Breaker & Error Handling Scenarios (from bdd_circuit_error_scenarios_test.go)
	s.Then(`^circuit breakers should respond appropriately$`, ctx.circuitBreakersShouldRespondAppropriately)
	s.Then(`^circuit state should transition based on results$`, ctx.circuitStateShouldTransitionBasedOnResults)
	s.Then(`^appropriate client responses should be returned$`, ctx.appropriateClientResponsesShouldBeReturned)
	s.Then(`^appropriate error responses should be returned$`, ctx.appropriateErrorResponsesShouldBeReturned)

	// Feature Flag Scenario Steps (from bdd_feature_flag_scenarios_test.go and related files)
	s.When(`^I evaluate a feature flag$`, ctx.iEvaluateAFeatureFlag)
	s.When(`^the aggregator discovers evaluators$`, ctx.theAggregatorDiscoversEvaluators)
	s.Then(`^alternative backends should be used when flags are disabled$`, ctx.alternativeBackendsShouldBeUsedWhenFlagsAreDisabled)
	s.Then(`^alternative single backends should be used when disabled$`, ctx.alternativeSingleBackendsShouldBeUsedWhenDisabled)
	s.Then(`^tenant-specific routing should be applied$`, ctx.tenantSpecificRoutingShouldBeApplied)
	s.Then(`^comparison results should be logged with flag context$`, ctx.comparisonResultsShouldBeLoggedWithFlagContext)

	// Timeout, Caching & Headers Scenarios (from bdd_timeout_cache_header_scenarios_test.go)
	s.Then(`^timeout behavior should be applied per route$`, ctx.timeoutBehaviorShouldBeAppliedPerRoute)
	s.Then(`^fresh requests should hit backends after expiration$`, ctx.freshRequestsShouldHitBackendsAfterExpiration)
	// Removed duplicate - using the lowercase version on line 195 instead

	// Additional step implementations that were missing
	s.Then(`^connection failures should be handled gracefully$`, ctx.connectionFailuresShouldBeHandledGracefully)
	s.Then(`^error responses should be properly handled$`, ctx.errorResponsesShouldBeProperlyHandled)
	s.Then(`^each backend should use its specific circuit breaker configuration$`, ctx.eachBackendShouldUseItsSpecificCircuitBreakerConfiguration)
	s.Then(`^limited requests should be allowed through$`, ctx.limitedRequestsShouldBeAllowedThrough)
	s.Then(`^expired cache entries should be evicted$`, ctx.expiredCacheEntriesShouldBeEvicted)

	// Event-related steps that were missing registrations
	s.Then(`^a circuit breaker closed event should be emitted$`, ctx.aCircuitBreakerClosedEventShouldBeEmitted)
	s.Then(`^the event should contain health failure details$`, ctx.theEventShouldContainHealthFailureDetails)
	s.Then(`^the event should contain removal details$`, ctx.theEventShouldContainRemovalDetails)
	s.Then(`^the events should contain rotation details$`, ctx.theEventsShouldContainRotationDetails)

	// Feature flag steps that were missing registrations
	s.Then(`^appropriate backends should be compared based on flag state$`, ctx.appropriateBackendsShouldBeComparedBasedOnFlagState)
	s.Then(`^feature flags should be evaluated per tenant$`, ctx.featureFlagsShouldBeEvaluatedPerTenant)
	s.Then(`^feature flags should control backend selection$`, ctx.featureFlagsShouldControlBackendSelection)
	s.Then(`^feature flags should control route availability$`, ctx.featureFlagsShouldControlRouteAvailability)
	s.Then(`^feature flags should control routing decisions$`, ctx.featureFlagsShouldControlRoutingDecisions)

	// Timeout and comparison steps
	s.Then(`^requests should be terminated after timeout$`, ctx.requestsShouldBeTerminatedAfterTimeout)
	s.Then(`^responses should be compared and logged$`, ctx.responsesShouldBeComparedAndLogged)

	// Previously undefined steps - some exist but weren't registered, others newly implemented
	s.Then(`^a backend unhealthy event should be emitted$`, ctx.aBackendUnhealthyEventShouldBeEmitted)
	s.Then(`^a backend removed event should be emitted$`, ctx.aBackendRemovedEventShouldBeEmitted)
	s.Then(`^round-robin events should be emitted$`, ctx.roundRobinEventsShouldBeEmitted)
	s.When(`^a circuit breaker closes after recovery$`, ctx.aCircuitBreakerClosesAfterRecovery)
	s.When(`^a circuit breaker transitions to half-open$`, ctx.aCircuitBreakerTransitionsToHalfopen)
	s.Then(`^a circuit breaker half-open event should be emitted$`, ctx.aCircuitBreakerHalfopenEventShouldBeEmitted)
	s.Then(`^the event should contain failure threshold details$`, ctx.theEventShouldContainFailureThresholdDetails)
	s.When(`^requests are made to flagged routes$`, ctx.requestsAreMadeToFlaggedRoutes)
	s.When(`^requests target flagged backends$`, ctx.requestsTargetFlaggedBackends)
	s.When(`^requests are made to composite routes$`, ctx.requestsAreMadeToCompositeRoutes)
	s.When(`^requests are made with different tenant contexts$`, ctx.requestsAreMadeWithDifferentTenantContexts)
	s.Then(`^requests should be sent to both primary and comparison backends$`, ctx.requestsShouldBeSentToBothPrimaryAndComparisonBackends)
	s.When(`^feature flags control routing in dry run mode$`, ctx.featureFlagsControlRoutingInDryRunMode)

	// Newly implemented steps (previously missing)
	s.When(`^different backends fail at different rates$`, ctx.differentBackendsFailAtDifferentRates)
	s.When(`^test requests are sent through half-open circuits$`, ctx.testRequestsAreSentThroughHalfopenCircuits)
	s.When(`^cached responses age beyond TTL$`, ctx.cachedResponsesAgeBeyondTTL)
	s.When(`^backends return error responses$`, ctx.backendsReturnErrorResponses)
	s.When(`^backend connections fail$`, ctx.backendConnectionsFail)

	// Configuration setup steps that were missing
	s.Given(`^I have a reverse proxy configured for error handling$`, ctx.iHaveAReverseProxyConfiguredForErrorHandling)
	s.Given(`^I have a reverse proxy configured for connection failure handling$`, ctx.iHaveAReverseProxyConfiguredForConnectionFailureHandling)
	s.Given(`^I have a reverse proxy with per-backend circuit breaker settings$`, ctx.iHaveAReverseProxyWithPerBackendCircuitBreakerSettings)
	s.Given(`^I have a reverse proxy with circuit breakers in half-open state$`, ctx.iHaveAReverseProxyWithCircuitBreakersInHalfopenState)
	s.Given(`^I have a reverse proxy with specific cache TTL configured$`, ctx.iHaveAReverseProxyWithSpecificCacheTTLConfigured)
	// These already exist in other BDD files but need registration
	s.Given(`^I have a reverse proxy with route-level feature flags configured$`, ctx.iHaveAReverseProxyWithRouteLevelFeatureFlagsConfigured)
	s.Given(`^I have a reverse proxy with backend-level feature flags configured$`, ctx.iHaveAReverseProxyWithBackendLevelFeatureFlagsConfigured)
	s.Given(`^I have a reverse proxy with composite route feature flags configured$`, ctx.iHaveAReverseProxyWithCompositeRouteFeatureFlagsConfigured)
	s.Given(`^I have a reverse proxy with tenant-specific feature flags configured$`, ctx.iHaveAReverseProxyWithTenantSpecificFeatureFlagsConfigured)
	s.Given(`^I have a reverse proxy with dry run mode and feature flags configured$`, ctx.iHaveAReverseProxyWithDryRunModeAndFeatureFlagsConfigured)
	s.Given(`^I have a reverse proxy with dry run mode enabled$`, ctx.iHaveAReverseProxyWithDryRunModeEnabled)

	// Action steps that were missing - these already exist in other BDD files but need registration
	s.When(`^a backend becomes unhealthy$`, ctx.aBackendBecomesUnhealthy)
	s.When(`^a backend is removed from the configuration$`, ctx.aBackendIsRemovedFromTheConfiguration)
	s.When(`^round-robin load balancing is used$`, ctx.roundRobinLoadBalancingIsUsed)
	s.When(`^requests are processed in dry run mode$`, ctx.requestsAreProcessedInDryRunMode)
	s.When(`^backend requests exceed the timeout$`, ctx.backendRequestsExceedTheTimeout)
	s.Then(`^route-specific timeouts should override global settings$`, ctx.routeSpecificTimeoutsShouldOverrideGlobalSettings)
	s.Then(`^the event should contain backend configuration$`, ctx.theEventShouldContainBackendConfiguration)
	s.Then(`^the event should contain backend health details$`, ctx.theEventShouldContainBackendHealthDetails)
	s.Then(`^the events should contain selected backend information$`, ctx.theEventsShouldContainSelectedBackendInformation)

	// Final batch of missing step registrations
	s.Given(`^I have a reverse proxy with global request timeout configured$`, ctx.iHaveAReverseProxyWithGlobalRequestTimeoutConfigured)
	s.Then(`^a backend added event should be emitted$`, ctx.aBackendAddedEventShouldBeEmitted)
	s.Then(`^a backend healthy event should be emitted$`, ctx.aBackendHealthyEventShouldBeEmitted)
	s.Then(`^load balance decision events should be emitted$`, ctx.loadBalanceDecisionEventsShouldBeEmitted)
	s.When(`^requests are made to routes with specific timeouts$`, ctx.requestsAreMadeToRoutesWithSpecificTimeouts)

	// Very final registrations
	s.Given(`^I have a reverse proxy with per-route timeout overrides configured$`, ctx.iHaveAReverseProxyWithPerRouteTimeoutOverridesConfigured)
	s.When(`^a backend becomes healthy$`, ctx.aBackendBecomesHealthy)
	s.When(`^a new backend is added to the configuration$`, ctx.aNewBackendIsAddedToTheConfiguration)
	s.When(`^load balancing decisions are made$`, ctx.loadBalancingDecisionsAreMade)

	// Absolute final registrations
	s.Given(`^I have backends with health checking enabled$`, ctx.iHaveBackendsWithHealthCheckingEnabled)
	s.Given(`^I have multiple backends configured$`, ctx.iHaveMultipleBackendsConfigured)

	// Circuit breaker event steps (from bdd_events_test.go) - these were missing registrations
	s.Given(`^I have circuit breaker enabled for backends$`, ctx.iHaveCircuitBreakerEnabledForBackends)
	s.When(`^a circuit breaker opens due to failures$`, ctx.aCircuitBreakerOpensDueToFailures)
	s.Then(`^a circuit breaker open event should be emitted$`, ctx.aCircuitBreakerOpenEventShouldBeEmitted)

	// Round-Robin with Circuit Breaker Scenarios (from bdd_roundrobin_circuit_test.go)
	s.Given(`^I have a round-robin backend group with circuit breakers$`, ctx.iHaveARoundRobinBackendGroupWithCircuitBreakers)
	s.When(`^I force one backend to trip its circuit breaker$`, ctx.iForceOneBackendToTripItsCircuitBreaker)
	s.Then(`^subsequent requests should rotate to healthy backends$`, ctx.subsequentRequestsShouldRotateToHealthyBackends)
	s.Then(`^load balance round robin events should fire$`, ctx.loadBalanceRoundRobinEventsShouldFire)
	s.Then(`^circuit breaker open events should fire$`, ctx.circuitBreakerOpenEventsShouldFire)
	s.Then(`^handler should return 503 when all backends down$`, ctx.handlerShouldReturn503WhenAllBackendsDown)

	// Feature Flag Dry-Run Scenario Steps (from bdd_feature_flag_dryrun_test.go)
	s.Given(`^I have a composite route guarded by feature flag$`, ctx.iHaveACompositeRouteGuardedByFeatureFlag)
	s.When(`^I enable module-level dry run mode$`, ctx.iEnableModuleLevelDryRunMode)
	s.When(`^I disable the feature flag for composite route$`, ctx.iDisableTheFeatureFlagForCompositeRoute)
	s.Then(`^dry-run handler should compare alternative with primary$`, ctx.dryRunHandlerShouldCompareAlternativeWithPrimary)
	s.Then(`^log output should include comparison diffs$`, ctx.logOutputShouldIncludeComparisonDiffs)
	s.Then(`^CloudEvents should show request.received and request.failed when backends diverge$`, ctx.cloudEventsShouldShowRequestReceivedAndFailed)
	s.When(`^I make a request to the composite route$`, ctx.iMakeARequestToTheCompositeRoute)
	s.Then(`^the response should come from the alternative backend$`, ctx.theResponseShouldComeFromTheAlternativeBackend)

	// Timeout-related scenario steps (removing duplicate to avoid ambiguity)
	s.Then(`^appropriate timeout error responses should be returned$`, ctx.appropriateTimeoutErrorResponsesShouldBeReturned)

	// Pipeline and Fan-Out-Merge Composite Strategy Steps (from bdd_composite_pipeline_test.go)
	s.Given(`^I have a pipeline composite route with two backends$`, ctx.iHaveAPipelineCompositeRouteWithTwoBackends)
	s.When(`^I send a request to the pipeline route$`, ctx.iSendARequestToThePipelineRoute)
	s.Then(`^the first backend should be called with the original request$`, ctx.theFirstBackendShouldBeCalledWithTheOriginalRequest)
	s.Then(`^the second backend should receive data derived from the first response$`, ctx.theSecondBackendShouldReceiveDataDerivedFromTheFirstResponse)
	s.Then(`^the final response should contain merged data from all stages$`, ctx.theFinalResponseShouldContainMergedDataFromAllStages)

	s.Given(`^I have a fan-out-merge composite route with two backends$`, ctx.iHaveAFanOutMergeCompositeRouteWithTwoBackends)
	s.When(`^I send a request to the fan-out-merge route$`, ctx.iSendARequestToTheFanOutMergeRoute)
	s.Then(`^both backends should be called in parallel$`, ctx.bothBackendsShouldBeCalledInParallel)
	s.Then(`^the responses should be merged by matching IDs$`, ctx.theResponsesShouldBeMergedByMatchingIDs)
	s.Then(`^items with matching ancillary data should be enriched$`, ctx.itemsWithMatchingAncillaryDataShouldBeEnriched)

	s.Given(`^I have a pipeline route with skip-empty policy$`, ctx.iHaveAPipelineRouteWithSkipEmptyPolicy)
	s.When(`^I send a request and a backend returns an empty response$`, ctx.iSendARequestAndABackendReturnsAnEmptyResponse)
	s.Then(`^the empty response should be excluded from the result$`, ctx.theEmptyResponseShouldBeExcludedFromTheResult)
	s.Then(`^the non-empty responses should still be present$`, ctx.theNonEmptyResponsesShouldStillBePresent)

	s.Given(`^I have a fan-out-merge route with fail-on-empty policy$`, ctx.iHaveAFanOutMergeRouteWithFailOnEmptyPolicy)
	s.Then(`^the request should fail with a bad gateway error$`, ctx.theRequestShouldFailWithABadGatewayError)

	s.Given(`^I have a pipeline route that filters by ancillary backend data$`, ctx.iHaveAPipelineRouteThatFiltersByAncillaryBackendData)
	s.When(`^I send a request to fetch filtered results$`, ctx.iSendARequestToFetchFilteredResults)
	s.Then(`^only items matching the ancillary criteria should be returned$`, ctx.onlyItemsMatchingTheAncillaryCriteriaShouldBeReturned)

	// Note: Most comprehensive step implementations are already in existing BDD files
	// Only add new steps here for scenarios that are completely missing implementations
}
