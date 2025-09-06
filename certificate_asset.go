package modular

import (
	"crypto/x509"
	"time"
)

// CertificateAsset represents managed TLS certificate material
type CertificateAsset struct {
	// ID is a unique identifier for this certificate asset
	ID string

	// Name is a human-readable name for this certificate
	Name string

	// Domains lists the domain names this certificate is valid for
	Domains []string

	// Certificate contains the PEM-encoded certificate data
	Certificate []byte

	// PrivateKey contains the PEM-encoded private key data
	PrivateKey []byte

	// CertificateChain contains the full certificate chain
	CertificateChain [][]byte

	// ParsedCertificate is the parsed X.509 certificate
	ParsedCertificate *x509.Certificate

	// IssuerName identifies the certificate issuer (e.g., "Let's Encrypt")
	IssuerName string

	// SerialNumber is the certificate serial number
	SerialNumber string

	// CreatedAt tracks when this certificate was first created
	CreatedAt time.Time

	// IssuedAt tracks when this certificate was issued
	IssuedAt time.Time

	// ExpiresAt tracks when this certificate expires
	ExpiresAt time.Time

	// RenewAt tracks when renewal should be attempted
	RenewAt time.Time

	// LastRenewalAttempt tracks the last renewal attempt
	LastRenewalAttempt *time.Time

	// NextRenewalAttempt tracks when the next renewal will be attempted
	NextRenewalAttempt *time.Time

	// RenewalCount tracks how many times this certificate has been renewed
	RenewalCount int

	// Status indicates the current status of this certificate
	Status CertificateStatus

	// RenewalPolicy defines when and how to renew this certificate
	RenewalPolicy *CertificateRenewalPolicy

	// Metadata contains additional certificate-specific metadata
	Metadata map[string]interface{}

	// ACMEAccount contains ACME account information if applicable
	ACMEAccount *ACMEAccountInfo

	// ValidationMethods lists the validation methods used for this certificate
	ValidationMethods []string

	// AutoRenew indicates if this certificate should be automatically renewed
	AutoRenew bool

	// InUse indicates if this certificate is currently being used
	InUse bool
}

// CertificateStatus represents the status of a certificate
type CertificateStatus string

const (
	// CertificateStatusValid indicates the certificate is valid and usable
	CertificateStatusValid CertificateStatus = "valid"

	// CertificateStatusExpiring indicates the certificate is approaching expiration
	CertificateStatusExpiring CertificateStatus = "expiring"

	// CertificateStatusExpired indicates the certificate has expired
	CertificateStatusExpired CertificateStatus = "expired"

	// CertificateStatusRenewing indicates the certificate is being renewed
	CertificateStatusRenewing CertificateStatus = "renewing"

	// CertificateStatusFailed indicates certificate operations have failed
	CertificateStatusFailed CertificateStatus = "failed"

	// CertificateStatusPending indicates the certificate is being issued
	CertificateStatusPending CertificateStatus = "pending"

	// CertificateStatusRevoked indicates the certificate has been revoked
	CertificateStatusRevoked CertificateStatus = "revoked"
)

// CertificateRenewalPolicy defines when and how to renew a certificate
type CertificateRenewalPolicy struct {
	// RenewBeforeExpiry specifies how long before expiry to start renewal
	RenewBeforeExpiry time.Duration

	// MaxRetries specifies maximum renewal attempts
	MaxRetries int

	// RetryDelay specifies delay between renewal attempts
	RetryDelay time.Duration

	// EscalationThreshold specifies when to escalate renewal failures
	EscalationThreshold time.Duration

	// NotificationEmails lists emails to notify of renewal events
	NotificationEmails []string

	// WebhookURL specifies a webhook to call for renewal events
	WebhookURL string

	// PreRenewalHooks lists functions to call before renewal
	PreRenewalHooks []CertificateHookFunc

	// PostRenewalHooks lists functions to call after renewal
	PostRenewalHooks []CertificateHookFunc
}

// CertificateHookFunc defines the signature for certificate lifecycle hooks
type CertificateHookFunc func(cert *CertificateAsset) error

// ACMEAccountInfo contains ACME account information
type ACMEAccountInfo struct {
	// AccountURL is the ACME account URL
	AccountURL string

	// Email is the account email address
	Email string

	// PrivateKey is the account private key
	PrivateKey []byte

	// TermsAgreed indicates if terms of service were agreed to
	TermsAgreed bool

	// DirectoryURL is the ACME directory URL
	DirectoryURL string

	// CreatedAt tracks when this account was created
	CreatedAt time.Time
}

// CertificateEvent represents events in the certificate lifecycle
type CertificateEvent struct {
	// CertificateID is the ID of the certificate this event relates to
	CertificateID string

	// EventType indicates what happened
	EventType CertificateEventType

	// Timestamp indicates when this event occurred
	Timestamp time.Time

	// Message provides details about the event
	Message string

	// Error contains error information if applicable
	Error string

	// Metadata contains event-specific metadata
	Metadata map[string]interface{}
}

// CertificateEventType represents types of certificate events
type CertificateEventType string

const (
	// CertificateEventTypeIssued indicates a certificate was issued
	CertificateEventTypeIssued CertificateEventType = "issued"

	// CertificateEventTypeRenewed indicates a certificate was renewed
	CertificateEventTypeRenewed CertificateEventType = "renewed"

	// CertificateEventTypeRenewalFailed indicates renewal failed
	CertificateEventTypeRenewalFailed CertificateEventType = "renewal_failed"

	// CertificateEventTypeExpiring indicates a certificate is expiring soon
	CertificateEventTypeExpiring CertificateEventType = "expiring"

	// CertificateEventTypeExpired indicates a certificate has expired
	CertificateEventTypeExpired CertificateEventType = "expired"

	// CertificateEventTypeRevoked indicates a certificate was revoked
	CertificateEventTypeRevoked CertificateEventType = "revoked"
)
