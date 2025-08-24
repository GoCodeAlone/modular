package auth

// Event type constants for auth module events.
// Following CloudEvents specification reverse domain notation.
const (
	// Token events
	EventTypeTokenGenerated = "com.modular.auth.token.generated" // #nosec G101 - not a credential
	EventTypeTokenValidated = "com.modular.auth.token.validated" // #nosec G101 - not a credential
	EventTypeTokenExpired   = "com.modular.auth.token.expired"   // #nosec G101 - not a credential
	EventTypeTokenRefreshed = "com.modular.auth.token.refreshed" // #nosec G101 - not a credential

	// Session events
	EventTypeSessionCreated   = "com.modular.auth.session.created"
	EventTypeSessionAccessed  = "com.modular.auth.session.accessed"
	EventTypeSessionExpired   = "com.modular.auth.session.expired"
	EventTypeSessionDestroyed = "com.modular.auth.session.destroyed"

	// OAuth2 events
	EventTypeOAuth2AuthURL  = "com.modular.auth.oauth2.auth_url"
	EventTypeOAuth2Exchange = "com.modular.auth.oauth2.exchange"
)
