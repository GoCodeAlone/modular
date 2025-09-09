package letsencrypt

// Event type constants for letsencrypt module events.
// Following CloudEvents specification reverse domain notation.
const (
	// Configuration events
	EventTypeConfigLoaded    = "com.modular.letsencrypt.config.loaded"
	EventTypeConfigValidated = "com.modular.letsencrypt.config.validated"

	// Certificate lifecycle events
	EventTypeCertificateRequested = "com.modular.letsencrypt.certificate.requested"
	EventTypeCertificateIssued    = "com.modular.letsencrypt.certificate.issued"
	EventTypeCertificateRenewed   = "com.modular.letsencrypt.certificate.renewed"
	EventTypeCertificateRevoked   = "com.modular.letsencrypt.certificate.revoked"
	EventTypeCertificateExpiring  = "com.modular.letsencrypt.certificate.expiring"
	EventTypeCertificateExpired   = "com.modular.letsencrypt.certificate.expired"

	// ACME protocol events
	EventTypeAcmeChallenge     = "com.modular.letsencrypt.acme.challenge"
	EventTypeAcmeAuthorization = "com.modular.letsencrypt.acme.authorization"
	EventTypeAcmeOrder         = "com.modular.letsencrypt.acme.order"

	// Service events
	EventTypeServiceStarted = "com.modular.letsencrypt.service.started"
	EventTypeServiceStopped = "com.modular.letsencrypt.service.stopped"

	// Storage events
	EventTypeStorageRead  = "com.modular.letsencrypt.storage.read"
	EventTypeStorageWrite = "com.modular.letsencrypt.storage.write"
	EventTypeStorageError = "com.modular.letsencrypt.storage.error"

	// Module lifecycle events
	EventTypeModuleStarted = "com.modular.letsencrypt.module.started"
	EventTypeModuleStopped = "com.modular.letsencrypt.module.stopped"

	// Error events
	EventTypeError   = "com.modular.letsencrypt.error"
	EventTypeWarning = "com.modular.letsencrypt.warning"

	// Escalation events
	EventTypeCertificateRenewalEscalated           = "com.modular.letsencrypt.certificate.renewal.escalated"
	EventTypeCertificateRenewalEscalationRecovered = "com.modular.letsencrypt.certificate.renewal.escalation.recovered"
)
