package letsencrypt

import (
	"crypto/tls"
)

// CertificateService defines the interface for a service that can provide TLS certificates
type CertificateService interface {
	// GetCertificate returns a certificate for the given ClientHello
	GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error)

	// GetCertificateForDomain returns a certificate for the specified domain
	GetCertificateForDomain(domain string) (*tls.Certificate, error)

	// Domains returns a list of domains this service can provide certificates for
	Domains() []string
}

// ChallengeHandler defines the interface for handlers that can handle ACME challenges
type ChallengeHandler interface {
	// PresentChallenge is called when a challenge token needs to be made available
	PresentChallenge(domain, token, keyAuth string) error

	// CleanupChallenge is called when a challenge token needs to be removed
	CleanupChallenge(domain, token, keyAuth string) error
}
