// Package letsencrypt provides Let's Encrypt certificate management
package letsencrypt

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
)

// Static errors for certificate management
var (
	ErrCertificateNotFound       = errors.New("certificate not found")
	ErrCertificateExpired        = errors.New("certificate has expired")
	ErrRenewalInProgress         = errors.New("renewal already in progress")
	ErrRenewalFailed             = errors.New("certificate renewal failed")
	ErrInvalidCertificate        = errors.New("invalid certificate")
	ErrACMEProviderNotConfigured = errors.New("ACME provider not configured")
	ErrDomainValidationFailed    = errors.New("domain validation failed")
	ErrRenewalHookFailed         = errors.New("renewal hook execution failed")
)

// CertificateManager manages the lifecycle of SSL/TLS certificates
type CertificateManager struct {
	mu               sync.RWMutex
	certificates     map[string]*CertificateInfo
	renewalScheduler CertificateScheduler
	acmeClient       ACMEClient
	storage          CertificateStorage
	config           *ManagerConfig
	logger           modular.Logger
	
	// Renewal tracking
	renewalInProgress map[string]bool
	renewalMutex      sync.RWMutex
}

// CertificateInfo represents information about a managed certificate
type CertificateInfo struct {
	Domain          string            `json:"domain"`
	Certificate     *tls.Certificate  `json:"-"`                // The actual certificate
	PEMCertificate  []byte            `json:"pem_certificate"`  // PEM-encoded certificate
	PEMPrivateKey   []byte            `json:"pem_private_key"`  // PEM-encoded private key
	ExpiresAt       time.Time         `json:"expires_at"`
	IssuedAt        time.Time         `json:"issued_at"`
	LastRenewed     *time.Time        `json:"last_renewed,omitempty"`
	RenewalAttempts int               `json:"renewal_attempts"`
	Status          CertificateStatus `json:"status"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	
	// Renewal configuration
	PreRenewalDays    int `json:"pre_renewal_days"`    // T048: Days before expiry to start renewal
	EscalationDays    int `json:"escalation_days"`     // T048: Days before expiry for escalation
	MaxRenewalAttempts int `json:"max_renewal_attempts"`
}

// CertificateStatus represents the status of a certificate
type CertificateStatus string

const (
	CertificateStatusActive   CertificateStatus = "active"
	CertificateStatusExpiring CertificateStatus = "expiring"
	CertificateStatusExpired  CertificateStatus = "expired"
	CertificateStatusRenewing CertificateStatus = "renewing"
	CertificateStatusFailed   CertificateStatus = "failed"
)

// ManagerConfig configures the certificate manager
type ManagerConfig struct {
	ACMEProviderConfig *ACMEProviderConfig      `json:"acme_provider,omitempty"`
	StorageConfig      *CertificateStorageConfig `json:"storage,omitempty"`
	DefaultPreRenewal  int                      `json:"default_pre_renewal_days"`  // T048: Default 30 days
	DefaultEscalation  int                      `json:"default_escalation_days"`   // T048: Default 7 days
	CheckInterval      time.Duration            `json:"check_interval"`            // How often to check for renewals
	RenewalTimeout     time.Duration            `json:"renewal_timeout"`           // Timeout for renewal operations
	EnableAutoRenewal  bool                     `json:"enable_auto_renewal"`       // Whether to automatically renew
	NotificationHooks  []string                 `json:"notification_hooks,omitempty"` // Hooks for notifications
}

// NewCertificateManager creates a new certificate manager
func NewCertificateManager(config *ManagerConfig, logger modular.Logger) (*CertificateManager, error) {
	if config == nil {
		config = &ManagerConfig{
			DefaultPreRenewal: 30, // T048: 30-day pre-renewal default
			DefaultEscalation: 7,  // T048: 7-day escalation default
			CheckInterval:     24 * time.Hour,
			RenewalTimeout:    10 * time.Minute,
			EnableAutoRenewal: true,
		}
	}

	// Set defaults if not provided
	if config.DefaultPreRenewal == 0 {
		config.DefaultPreRenewal = 30
	}
	if config.DefaultEscalation == 0 {
		config.DefaultEscalation = 7
	}
	if config.CheckInterval == 0 {
		config.CheckInterval = 24 * time.Hour
	}
	if config.RenewalTimeout == 0 {
		config.RenewalTimeout = 10 * time.Minute
	}

	return &CertificateManager{
		certificates:      make(map[string]*CertificateInfo),
		renewalInProgress: make(map[string]bool),
		config:            config,
		logger:            logger,
	}, nil
}

// RegisterCertificate registers a domain for certificate management
func (m *CertificateManager) RegisterCertificate(domain string, config *CertificateConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already registered
	if _, exists := m.certificates[domain]; exists {
		return fmt.Errorf("certificate for domain %s already registered", domain)
	}

	// Create certificate info with T048 defaults
	certInfo := &CertificateInfo{
		Domain:             domain,
		Status:             CertificateStatusActive,
		PreRenewalDays:     m.config.DefaultPreRenewal,
		EscalationDays:     m.config.DefaultEscalation,
		MaxRenewalAttempts: 3, // Default max attempts
		Metadata:           make(map[string]string),
	}

	// Apply custom configuration if provided
	if config != nil {
		if config.PreRenewalDays > 0 {
			certInfo.PreRenewalDays = config.PreRenewalDays
		}
		if config.EscalationDays > 0 {
			certInfo.EscalationDays = config.EscalationDays
		}
		if config.MaxRenewalAttempts > 0 {
			certInfo.MaxRenewalAttempts = config.MaxRenewalAttempts
		}
		for k, v := range config.Metadata {
			certInfo.Metadata[k] = v
		}
	}

	m.certificates[domain] = certInfo

	if m.logger != nil {
		m.logger.Info("Registered certificate for management", 
			"domain", domain, 
			"preRenewalDays", certInfo.PreRenewalDays,
			"escalationDays", certInfo.EscalationDays)
	}

	return nil
}

// T047: CheckRenewalNeeded determines if a certificate needs renewal
func (m *CertificateManager) CheckRenewalNeeded(domain string) (bool, CertificateStatus, error) {
	m.mu.RLock()
	certInfo, exists := m.certificates[domain]
	m.mu.RUnlock()

	if !exists {
		return false, "", ErrCertificateNotFound
	}

	now := time.Now()
	
	// Check if certificate has expired
	if now.After(certInfo.ExpiresAt) {
		certInfo.Status = CertificateStatusExpired
		return true, CertificateStatusExpired, nil
	}

	// T048: Check if within escalation period (urgent renewal needed)
	escalationThreshold := certInfo.ExpiresAt.AddDate(0, 0, -certInfo.EscalationDays)
	if now.After(escalationThreshold) {
		certInfo.Status = CertificateStatusExpiring
		return true, CertificateStatusExpiring, nil
	}

	// T048: Check if within pre-renewal period (normal renewal window)
	preRenewalThreshold := certInfo.ExpiresAt.AddDate(0, 0, -certInfo.PreRenewalDays)
	if now.After(preRenewalThreshold) {
		certInfo.Status = CertificateStatusExpiring
		return true, CertificateStatusExpiring, nil
	}

	return false, CertificateStatusActive, nil
}

// T047: RenewCertificate initiates certificate renewal for a domain
func (m *CertificateManager) RenewCertificate(ctx context.Context, domain string) error {
	// Check if renewal is already in progress
	m.renewalMutex.Lock()
	if m.renewalInProgress[domain] {
		m.renewalMutex.Unlock()
		return ErrRenewalInProgress
	}
	m.renewalInProgress[domain] = true
	m.renewalMutex.Unlock()

	// Ensure we clean up the renewal flag
	defer func() {
		m.renewalMutex.Lock()
		delete(m.renewalInProgress, domain)
		m.renewalMutex.Unlock()
	}()

	m.mu.RLock()
	certInfo, exists := m.certificates[domain]
	m.mu.RUnlock()

	if !exists {
		return ErrCertificateNotFound
	}

	if m.logger != nil {
		m.logger.Info("Starting certificate renewal", "domain", domain)
	}

	// Update status to renewing
	m.mu.Lock()
	certInfo.Status = CertificateStatusRenewing
	certInfo.RenewalAttempts++
	m.mu.Unlock()

	// Create renewal context with timeout
	renewalCtx, cancel := context.WithTimeout(ctx, m.config.RenewalTimeout)
	defer cancel()

	// Perform the actual renewal (this would integrate with ACME client)
	err := m.performRenewal(renewalCtx, certInfo)
	
	m.mu.Lock()
	if err != nil {
		certInfo.Status = CertificateStatusFailed
		if m.logger != nil {
			m.logger.Error("Certificate renewal failed", "domain", domain, "error", err, "attempts", certInfo.RenewalAttempts)
		}
		
		// T048: Check if we need escalation
		if certInfo.RenewalAttempts >= certInfo.MaxRenewalAttempts {
			m.triggerEscalation(certInfo, err)
		}
	} else {
		now := time.Now()
		certInfo.Status = CertificateStatusActive
		certInfo.LastRenewed = &now
		certInfo.RenewalAttempts = 0 // Reset on success
		
		if m.logger != nil {
			m.logger.Info("Certificate renewal successful", "domain", domain)
		}
	}
	m.mu.Unlock()

	return err
}

// T047: performRenewal performs the actual certificate renewal
func (m *CertificateManager) performRenewal(ctx context.Context, certInfo *CertificateInfo) error {
	// TODO: This would integrate with the actual ACME client implementation
	// For now, this is a skeleton that demonstrates the renewal flow
	
	if m.acmeClient == nil {
		return ErrACMEProviderNotConfigured
	}

	// Step 1: Request new certificate from ACME provider
	newCert, newKey, err := m.acmeClient.ObtainCertificate(ctx, certInfo.Domain)
	if err != nil {
		return fmt.Errorf("failed to obtain new certificate: %w", err)
	}

	// Step 2: Validate the new certificate
	err = m.validateCertificate(newCert, newKey, certInfo.Domain)
	if err != nil {
		return fmt.Errorf("new certificate validation failed: %w", err)
	}

	// Step 3: Store the new certificate
	if m.storage != nil {
		err = m.storage.StoreCertificate(certInfo.Domain, newCert, newKey)
		if err != nil {
			return fmt.Errorf("failed to store new certificate: %w", err)
		}
	}

	// Step 4: Update certificate info
	certInfo.PEMCertificate = newCert
	certInfo.PEMPrivateKey = newKey
	
	// Parse expiration date from new certificate
	expiresAt, err := m.parseCertificateExpiry(newCert)
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("Failed to parse certificate expiry", "domain", certInfo.Domain, "error", err)
		}
		// Set a default expiry (90 days from now, typical for Let's Encrypt)
		expiresAt = time.Now().AddDate(0, 0, 90)
	}
	certInfo.ExpiresAt = expiresAt

	return nil
}

// T048: triggerEscalation handles escalation when renewal fails repeatedly
func (m *CertificateManager) triggerEscalation(certInfo *CertificateInfo, renewalErr error) {
	if m.logger != nil {
		m.logger.Error("Certificate renewal escalation triggered", 
			"domain", certInfo.Domain, 
			"attempts", certInfo.RenewalAttempts,
			"expiresAt", certInfo.ExpiresAt,
			"renewalError", renewalErr)
	}

	// Execute notification hooks for escalation
	for _, hookName := range m.config.NotificationHooks {
		err := m.executeNotificationHook(hookName, certInfo, renewalErr)
		if err != nil && m.logger != nil {
			m.logger.Error("Notification hook execution failed", 
				"hook", hookName, 
				"domain", certInfo.Domain, 
				"error", err)
		}
	}

	// Update metadata to track escalation
	certInfo.Metadata["escalation_triggered"] = time.Now().Format(time.RFC3339)
	certInfo.Metadata["escalation_reason"] = renewalErr.Error()
}

// StartAutoRenewalCheck starts the automatic renewal checking process
func (m *CertificateManager) StartAutoRenewalCheck(ctx context.Context) {
	if !m.config.EnableAutoRenewal {
		return
	}

	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	if m.logger != nil {
		m.logger.Info("Starting automatic certificate renewal checks", "interval", m.config.CheckInterval)
	}

	for {
		select {
		case <-ctx.Done():
			if m.logger != nil {
				m.logger.Info("Stopping automatic certificate renewal checks")
			}
			return
		case <-ticker.C:
			m.checkAllCertificates(ctx)
		}
	}
}

// checkAllCertificates checks all registered certificates for renewal needs
func (m *CertificateManager) checkAllCertificates(ctx context.Context) {
	m.mu.RLock()
	domains := make([]string, 0, len(m.certificates))
	for domain := range m.certificates {
		domains = append(domains, domain)
	}
	m.mu.RUnlock()

	for _, domain := range domains {
		needsRenewal, status, err := m.CheckRenewalNeeded(domain)
		if err != nil {
			if m.logger != nil {
				m.logger.Error("Failed to check renewal status", "domain", domain, "error", err)
			}
			continue
		}

		if needsRenewal {
			if m.logger != nil {
				m.logger.Info("Certificate needs renewal", "domain", domain, "status", status)
			}
			
			// Perform renewal in background
			go func(d string) {
				renewalCtx, cancel := context.WithTimeout(context.Background(), m.config.RenewalTimeout)
				defer cancel()
				
				err := m.RenewCertificate(renewalCtx, d)
				if err != nil && m.logger != nil {
					m.logger.Error("Automatic renewal failed", "domain", d, "error", err)
				}
			}(domain)
		}
	}
}

// Helper methods (placeholders for actual implementation)

func (m *CertificateManager) validateCertificate(cert, key []byte, domain string) error {
	// TODO: Implement certificate validation logic
	return nil
}

func (m *CertificateManager) parseCertificateExpiry(cert []byte) (time.Time, error) {
	// TODO: Implement certificate parsing to extract expiry date
	return time.Now().AddDate(0, 0, 90), nil
}

func (m *CertificateManager) executeNotificationHook(hookName string, certInfo *CertificateInfo, err error) error {
	// TODO: Implement notification hook execution
	return nil
}

// Interfaces that would be implemented by other components

// ACMEClient defines the interface for ACME operations
type ACMEClient interface {
	ObtainCertificate(ctx context.Context, domain string) (cert, key []byte, err error)
	RevokeCertificate(ctx context.Context, cert []byte) error
}

// CertificateStorage defines the interface for certificate storage
type CertificateStorage interface {
	StoreCertificate(domain string, cert, key []byte) error
	LoadCertificate(domain string) (cert, key []byte, err error)
	DeleteCertificate(domain string) error
}

// CertificateScheduler defines the interface for renewal scheduling
type CertificateScheduler interface {
	ScheduleRenewal(domain string, renewAt time.Time) error
	CancelRenewal(domain string) error
}

// Configuration structures

// CertificateConfig provides per-certificate configuration
type CertificateConfig struct {
	PreRenewalDays     int               `json:"pre_renewal_days,omitempty"`
	EscalationDays     int               `json:"escalation_days,omitempty"`
	MaxRenewalAttempts int               `json:"max_renewal_attempts,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

// ACMEProviderConfig configures the ACME provider
type ACMEProviderConfig struct {
	DirectoryURL string `json:"directory_url"`
	Email        string `json:"email"`
	KeyType      string `json:"key_type"`
}

// CertificateStorageConfig configures certificate storage
type CertificateStorageConfig struct {
	Type   string            `json:"type"`   // "file", "database", etc.
	Config map[string]string `json:"config"` // Type-specific configuration
}