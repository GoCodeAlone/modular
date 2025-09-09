package letsencrypt

import (
	"time"
)

// EscalationType represents the type of escalation for certificate renewal issues
type EscalationType string

const (
	EscalationTypeRetryExhausted   EscalationType = "retry_exhausted"
	EscalationTypeExpiringSoon     EscalationType = "expiring_soon"
	EscalationTypeValidationFailed EscalationType = "validation_failed"
	EscalationTypeRateLimited      EscalationType = "rate_limited"
	EscalationTypeACMEError        EscalationType = "acme_error"
)

// String returns the string representation of EscalationType
func (et EscalationType) String() string {
	return string(et)
}

// EscalationSeverity represents the severity level of an escalation
type EscalationSeverity string

const (
	EscalationSeverityLow      EscalationSeverity = "low"
	EscalationSeverityMedium   EscalationSeverity = "medium"
	EscalationSeverityHigh     EscalationSeverity = "high"
	EscalationSeverityCritical EscalationSeverity = "critical"
	EscalationSeverityWarning  EscalationSeverity = "warning"
)

// Severity returns the severity level associated with an escalation type
func (et EscalationType) Severity() EscalationSeverity {
	switch et {
	case EscalationTypeRetryExhausted:
		return EscalationSeverityCritical
	case EscalationTypeExpiringSoon:
		return EscalationSeverityWarning
	case EscalationTypeValidationFailed:
		return EscalationSeverityHigh
	case EscalationTypeRateLimited:
		return EscalationSeverityMedium
	case EscalationTypeACMEError:
		return EscalationSeverityHigh
	default:
		return EscalationSeverityMedium
	}
}

// CertificateInfo contains information about a certificate
type CertificateInfo struct {
	Domain         string
	SerialNumber   string
	Issuer         string
	ExpirationTime time.Time
	DaysRemaining  int
	IsValid        bool
	Fingerprint    string
}

// IsExpiringSoon checks if the certificate is expiring within the specified threshold
func (ci *CertificateInfo) IsExpiringSoon(thresholdDays int) bool {
	return ci.DaysRemaining <= thresholdDays
}

// CertificateRenewalEscalatedEvent represents an escalated certificate renewal event
type CertificateRenewalEscalatedEvent struct {
	Domain          string
	EscalationID    string
	Timestamp       time.Time
	FailureCount    int
	LastFailureTime time.Time
	NextRetryTime   time.Time
	EscalationType  EscalationType
	CurrentCertInfo *CertificateInfo
	LastError       string
}

// EventType returns the event type
func (e *CertificateRenewalEscalatedEvent) EventType() string {
	return "certificate.renewal.escalated"
}

// EventSource returns the event source
func (e *CertificateRenewalEscalatedEvent) EventSource() string {
	return "modular.letsencrypt"
}

// StructuredFields returns structured logging fields for the event
func (e *CertificateRenewalEscalatedEvent) StructuredFields() map[string]interface{} {
	fields := map[string]interface{}{
		"module":          "letsencrypt",
		"phase":           "renewal.escalation",
		"event":           e.EventType(),
		"domain":          e.Domain,
		"escalation_id":   e.EscalationID,
		"escalation_type": string(e.EscalationType),
		"failure_count":   e.FailureCount,
		"severity":        string(e.EscalationType.Severity()),
	}

	if e.CurrentCertInfo != nil {
		fields["days_remaining"] = e.CurrentCertInfo.DaysRemaining
	}

	return fields
}

// X509CertificateInterface defines the interface for extracting certificate info
type X509CertificateInterface interface {
	Subject() string
	Issuer() string
	SerialNumber() string
	NotAfter() time.Time
}

// NewCertificateInfoFromX509 creates CertificateInfo from an x509 certificate
func NewCertificateInfoFromX509(cert X509CertificateInterface, domain string) (*CertificateInfo, error) {
	daysRemaining := int(time.Until(cert.NotAfter()).Hours() / 24)

	return &CertificateInfo{
		Domain:         domain,
		SerialNumber:   cert.SerialNumber(),
		Issuer:         cert.Issuer(),
		ExpirationTime: cert.NotAfter(),
		DaysRemaining:  daysRemaining,
		IsValid:        time.Now().Before(cert.NotAfter()),
		Fingerprint:    "", // Would need actual cert bytes to compute
	}, nil
}

// OrderSeveritiesByPriority sorts escalation severities by priority
func OrderSeveritiesByPriority(severities []EscalationSeverity) []EscalationSeverity {
	// Simple implementation - in real scenario would use proper sorting
	ordered := make([]EscalationSeverity, 0, len(severities))

	// Add in priority order
	for _, s := range severities {
		if s == EscalationSeverityCritical {
			ordered = append(ordered, s)
		}
	}
	for _, s := range severities {
		if s == EscalationSeverityHigh {
			ordered = append(ordered, s)
		}
	}
	for _, s := range severities {
		if s == EscalationSeverityMedium {
			ordered = append(ordered, s)
		}
	}
	for _, s := range severities {
		if s == EscalationSeverityWarning {
			ordered = append(ordered, s)
		}
	}
	for _, s := range severities {
		if s == EscalationSeverityLow {
			ordered = append(ordered, s)
		}
	}

	return ordered
}
