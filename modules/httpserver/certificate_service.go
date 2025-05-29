// Package httpserver provides an HTTP server module for the modular framework.
package httpserver

import (
	"crypto/tls"
)

// CertificateService defines the interface for a service that can provide TLS certificates
type CertificateService interface {
	// GetCertificate returns a certificate for the given ClientHello
	GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error)
}
