package reverseproxy

// Event type constants for reverseproxy module events.
// Following CloudEvents specification reverse domain notation.
const (
	// Configuration events
	EventTypeConfigLoaded    = "com.modular.reverseproxy.config.loaded"
	EventTypeConfigValidated = "com.modular.reverseproxy.config.validated"

	// Proxy events
	EventTypeProxyCreated = "com.modular.reverseproxy.proxy.created"
	EventTypeProxyStarted = "com.modular.reverseproxy.proxy.started"
	EventTypeProxyStopped = "com.modular.reverseproxy.proxy.stopped"

	// Request events
	EventTypeRequestReceived = "com.modular.reverseproxy.request.received"
	EventTypeRequestProxied  = "com.modular.reverseproxy.request.proxied"
	EventTypeRequestFailed   = "com.modular.reverseproxy.request.failed"

	// Backend events
	EventTypeBackendHealthy   = "com.modular.reverseproxy.backend.healthy"
	EventTypeBackendUnhealthy = "com.modular.reverseproxy.backend.unhealthy"
	EventTypeBackendAdded     = "com.modular.reverseproxy.backend.added"
	EventTypeBackendRemoved   = "com.modular.reverseproxy.backend.removed"

	// Load balancing events
	EventTypeLoadBalanceDecision   = "com.modular.reverseproxy.loadbalance.decision"
	EventTypeLoadBalanceRoundRobin = "com.modular.reverseproxy.loadbalance.roundrobin"

	// Circuit breaker events
	EventTypeCircuitBreakerOpen     = "com.modular.reverseproxy.circuitbreaker.open"
	EventTypeCircuitBreakerClosed   = "com.modular.reverseproxy.circuitbreaker.closed"
	EventTypeCircuitBreakerHalfOpen = "com.modular.reverseproxy.circuitbreaker.halfopen"

	// Module lifecycle events
	EventTypeModuleStarted = "com.modular.reverseproxy.module.started"
	EventTypeModuleStopped = "com.modular.reverseproxy.module.stopped"

	// Error events
	EventTypeError = "com.modular.reverseproxy.error"
)
