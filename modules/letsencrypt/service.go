// Package letsencrypt provides a module for automatic SSL certificate generation
// via Let's Encrypt for the modular framework.
package letsencrypt

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/certificate"
)

// certificateStorage handles the persistence of certificates on disk
type certificateStorage struct {
	basePath string
}

// newCertificateStorage creates a new certificate storage handler
func newCertificateStorage(basePath string) (*certificateStorage, error) {
	// Ensure storage directory exists
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create certificate storage directory: %w", err)
	}

	return &certificateStorage{
		basePath: basePath,
	}, nil
}

// SaveCertificate saves a certificate to disk
func (s *certificateStorage) SaveCertificate(domain string, cert *certificate.Resource) error {
	domainDir := filepath.Join(s.basePath, sanitizeDomain(domain))

	// Create domain directory
	if err := os.MkdirAll(domainDir, 0700); err != nil {
		return fmt.Errorf("failed to create domain directory: %w", err)
	}

	// Save certificate
	if err := ioutil.WriteFile(filepath.Join(domainDir, "cert.pem"), cert.Certificate, 0600); err != nil {
		return fmt.Errorf("failed to save certificate: %w", err)
	}

	// Save private key
	if err := ioutil.WriteFile(filepath.Join(domainDir, "key.pem"), cert.PrivateKey, 0600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	// Save certificate chain if available
	if len(cert.IssuerCertificate) > 0 {
		if err := ioutil.WriteFile(filepath.Join(domainDir, "chain.pem"), cert.IssuerCertificate, 0600); err != nil {
			return fmt.Errorf("failed to save certificate chain: %w", err)
		}
	}

	// Save metadata
	metaData := fmt.Sprintf("Domain: %s\nObtained: %s\n", domain, time.Now().Format(time.RFC3339))
	if err := ioutil.WriteFile(filepath.Join(domainDir, "metadata.txt"), []byte(metaData), 0600); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}

// LoadCertificate loads a certificate from disk
func (s *certificateStorage) LoadCertificate(domain string) (*tls.Certificate, error) {
	domainDir := filepath.Join(s.basePath, sanitizeDomain(domain))

	// Check if domain directory exists
	if _, err := os.Stat(domainDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("no certificate found for domain: %s", domain)
	}

	// Load certificate and key files
	certFile := filepath.Join(domainDir, "cert.pem")
	keyFile := filepath.Join(domainDir, "key.pem")

	// Check if files exist
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("certificate file not found: %s", certFile)
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("key file not found: %s", keyFile)
	}

	// Load and parse certificate
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	return &cert, nil
}

// ListCertificates returns a list of domains with stored certificates
func (s *certificateStorage) ListCertificates() ([]string, error) {
	var domains []string

	// Get all subdirectories
	files, err := ioutil.ReadDir(s.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list certificate directories: %w", err)
	}

	// Filter directories that have cert.pem files
	for _, file := range files {
		if file.IsDir() {
			certPath := filepath.Join(s.basePath, file.Name(), "cert.pem")
			if _, err := os.Stat(certPath); err == nil {
				domains = append(domains, desanitizeDomain(file.Name()))
			}
		}
	}

	return domains, nil
}

// IsCertificateExpiringSoon checks if a certificate is expiring within the given days
func (s *certificateStorage) IsCertificateExpiringSoon(domain string, days int) (bool, error) {
	domainDir := filepath.Join(s.basePath, sanitizeDomain(domain))
	certPath := filepath.Join(domainDir, "cert.pem")

	// Read certificate
	certPEMBlock, err := ioutil.ReadFile(certPath)
	if err != nil {
		return false, fmt.Errorf("failed to read certificate: %w", err)
	}

	// Parse PEM block
	block, _ := pem.Decode(certPEMBlock)
	if block == nil || block.Type != "CERTIFICATE" {
		return false, fmt.Errorf("failed to decode PEM block containing certificate")
	}

	// Parse certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Check expiry
	expiryTime := cert.NotAfter
	renewalThreshold := time.Now().AddDate(0, 0, days)

	return expiryTime.Before(renewalThreshold), nil
}

// Helper functions for sanitizing domain names for use in filesystem paths
func sanitizeDomain(domain string) string {
	return strings.ReplaceAll(domain, ".", "_")
}

func desanitizeDomain(sanitized string) string {
	return strings.ReplaceAll(sanitized, "_", ".")
}
