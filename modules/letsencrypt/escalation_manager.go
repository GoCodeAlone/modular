package letsencrypt

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrNilCertInfo = errors.New("letsencrypt: nil certInfo")
)

// EscalationConfig controls when escalation events are emitted.
// Tags follow configuration documentation conventions.
type EscalationConfig struct {
	FailureThreshold     int           `yaml:"failure_threshold" json:"failure_threshold" default:"3" desc:"Consecutive failures within window required to escalate"`
	Window               time.Duration `yaml:"window" json:"window" default:"5m" desc:"Time window for counting consecutive failures"`
	ExpiringSoonDays     int           `yaml:"expiring_soon_days" json:"expiring_soon_days" default:"7" desc:"Days before expiry that trigger expiring soon escalation"`
	RateLimitSubstring   string        `yaml:"rate_limit_substring" json:"rate_limit_substring" default:"rateLimited" desc:"Substring indicating ACME rate limit in error"`
	NotificationCooldown time.Duration `yaml:"notification_cooldown" json:"notification_cooldown" default:"15m" desc:"Minimum time between notifications for the same domain escalation"`
}

// setDefaults applies defaults where zero-values are present (when not populated via struct tags loader yet).
func (c *EscalationConfig) setDefaults() {
	if c.FailureThreshold == 0 {
		c.FailureThreshold = 3
	}
	if c.Window == 0 {
		c.Window = 5 * time.Minute
	}
	if c.ExpiringSoonDays == 0 {
		c.ExpiringSoonDays = 7
	}
	if c.RateLimitSubstring == "" {
		c.RateLimitSubstring = "rateLimited"
	}
	if c.NotificationCooldown == 0 {
		c.NotificationCooldown = 15 * time.Minute
	}
}

// NotificationChannel represents an outbound notification integration (email, webhook, etc.).
// We intentionally keep the contract narrow; richer templating can evolve additively.
type NotificationChannel interface {
	Notify(ctx context.Context, event *CertificateRenewalEscalatedEvent) error
}

// EscalationStats captures metrics-style counters for observability.
type EscalationStats struct {
	TotalEscalations int
	Reasons          map[EscalationType]int
	Resolutions      int
	LastResolution   time.Time
}

// escalationState tracks per-domain transient data.
type escalationState struct {
	failures         int
	firstFailureAt   time.Time
	lastFailureAt    time.Time
	lastNotification time.Time
	active           bool
	escalationType   EscalationType
	escalationID     string
	acknowledged     bool
}

// EscalationManager evaluates conditions and emits escalation & recovery events.
type EscalationManager struct {
	cfg EscalationConfig

	mu      sync.Mutex
	domains map[string]*escalationState

	channels []NotificationChannel
	now      func() time.Time

	// eventEmitter is injected to surface events to the broader modular observer system.
	eventEmitter func(ctx context.Context, event interface{}) error

	stats EscalationStats
}

// NewEscalationManager creates a manager with config and optional functional options.
func NewEscalationManager(cfg EscalationConfig, emitter func(ctx context.Context, event interface{}) error, opts ...func(*EscalationManager)) *EscalationManager {
	cfg.setDefaults()
	m := &EscalationManager{
		cfg:          cfg,
		domains:      make(map[string]*escalationState),
		eventEmitter: emitter,
		now:          time.Now,
		stats:        EscalationStats{Reasons: make(map[EscalationType]int)},
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// WithNotificationChannels registers outbound notification channels.
func WithNotificationChannels(ch ...NotificationChannel) func(*EscalationManager) {
	return func(m *EscalationManager) { m.channels = append(m.channels, ch...) }
}

// WithNow substitutes the time source (tests).
func WithNow(fn func() time.Time) func(*EscalationManager) {
	return func(m *EscalationManager) { m.now = fn }
}

// snapshotState returns (and creates) a domain state under lock.
func (m *EscalationManager) snapshotState(domain string) *escalationState {
	st, ok := m.domains[domain]
	if !ok {
		st = &escalationState{}
		m.domains[domain] = st
	}
	return st
}

// RecordFailure registers a renewal failure and triggers escalation when threshold criteria met.
func (m *EscalationManager) RecordFailure(ctx context.Context, domain string, errMsg string) (*CertificateRenewalEscalatedEvent, error) {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.snapshotState(domain)
	if st.failures == 0 || now.Sub(st.firstFailureAt) > m.cfg.Window {
		st.firstFailureAt = now
		st.failures = 0
	}
	st.failures++
	st.lastFailureAt = now

	if st.failures >= m.cfg.FailureThreshold {
		if !st.active {
			return m.escalateLocked(ctx, domain, EscalationTypeRetryExhausted, errMsg)
		}
		// already active escalation for this domain; maybe re-notify after cooldown
		if evt := m.maybeRenotifyLocked(ctx, domain, st, errMsg); evt != nil {
			return evt, nil
		}
	}
	return nil, nil
}

// RecordTimeout escalates immediately for timeouts (treated as validation failure high severity).
func (m *EscalationManager) RecordTimeout(ctx context.Context, domain string, errMsg string) (*CertificateRenewalEscalatedEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.escalateLocked(ctx, domain, EscalationTypeValidationFailed, errMsg)
}

// HandleACMEError classifies ACME errors (rate limit vs generic ACME error) and escalates.
func (m *EscalationManager) HandleACMEError(ctx context.Context, domain, acmeErr string) (*CertificateRenewalEscalatedEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	et := EscalationTypeACMEError
	if m.cfg.RateLimitSubstring != "" && contains(acmeErr, m.cfg.RateLimitSubstring) {
		et = EscalationTypeRateLimited
	}
	return m.escalateLocked(ctx, domain, et, acmeErr)
}

// CheckExpiration escalates if certificate is expiring soon.
func (m *EscalationManager) CheckExpiration(ctx context.Context, domain string, certInfo *CertificateInfo) (*CertificateRenewalEscalatedEvent, error) {
	if certInfo == nil {
		return nil, ErrNilCertInfo
	}
	if !certInfo.IsExpiringSoon(m.cfg.ExpiringSoonDays) {
		return nil, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.escalateLocked(ctx, domain, EscalationTypeExpiringSoon, "certificate expiring soon")
}

// Acknowledge marks an active escalation as acknowledged.
func (m *EscalationManager) Acknowledge(domain string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, ok := m.domains[domain]; ok {
		st.acknowledged = true
	}
}

// Clear resets state & emits recovery event if escalation was active.
func (m *EscalationManager) Clear(ctx context.Context, domain string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.domains[domain]
	if !ok || !st.active {
		return
	}
	// Emit recovery event
	rec := &CertificateRenewalEscalationRecoveredEvent{
		Domain:       domain,
		EscalationID: st.escalationID,
		ResolvedAt:   m.now(),
	}
	m.stats.Resolutions++
	m.stats.LastResolution = rec.ResolvedAt
	st.active = false
	st.failures = 0
	if m.eventEmitter != nil {
		_ = m.eventEmitter(ctx, rec)
	}
}

// Stats returns a copy of current counters.
func (m *EscalationManager) Stats() EscalationStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	// shallow copy
	reasons := make(map[EscalationType]int, len(m.stats.Reasons))
	for k, v := range m.stats.Reasons {
		reasons[k] = v
	}
	return EscalationStats{
		TotalEscalations: m.stats.TotalEscalations,
		Reasons:          reasons,
		Resolutions:      m.stats.Resolutions,
		LastResolution:   m.stats.LastResolution,
	}
}

// escalateLocked assumes mutex held.
func (m *EscalationManager) escalateLocked(ctx context.Context, domain string, et EscalationType, errMsg string) (*CertificateRenewalEscalatedEvent, error) {
	st := m.snapshotState(domain)
	now := m.now()
	if st.active && st.escalationType == et {
		// Possibly send notification if cooldown passed & not acknowledged
		if !st.acknowledged && now.Sub(st.lastNotification) >= m.cfg.NotificationCooldown {
			st.lastNotification = now
			// re-notify channels with existing event surface (idempotent-ish)
			evt := &CertificateRenewalEscalatedEvent{Domain: domain, EscalationID: st.escalationID, EscalationType: et, FailureCount: st.failures, LastError: errMsg, Timestamp: now}
			m.notify(ctx, evt)
		}
		return nil, nil
	}
	st.active = true
	st.escalationType = et
	st.escalationID = fmt.Sprintf("%s-%d", et, now.UnixNano())
	st.lastNotification = now
	evt := &CertificateRenewalEscalatedEvent{
		Domain:          domain,
		EscalationID:    st.escalationID,
		Timestamp:       now,
		FailureCount:    st.failures,
		LastFailureTime: st.lastFailureAt,
		EscalationType:  et,
		LastError:       errMsg,
	}
	m.stats.TotalEscalations++
	m.stats.Reasons[et]++

	m.notify(ctx, evt)
	if m.eventEmitter != nil {
		_ = m.eventEmitter(ctx, evt)
	}
	return evt, nil
}

func (m *EscalationManager) notify(ctx context.Context, evt *CertificateRenewalEscalatedEvent) {
	for _, ch := range m.channels {
		_ = ch.Notify(ctx, evt)
	}
}

// maybeRenotifyLocked sends a follow-up notification (without incrementing stats) if cooldown elapsed.
func (m *EscalationManager) maybeRenotifyLocked(ctx context.Context, domain string, st *escalationState, errMsg string) *CertificateRenewalEscalatedEvent {
	if !st.active || st.acknowledged {
		return nil
	}
	now := m.now()
	if now.Sub(st.lastNotification) < m.cfg.NotificationCooldown {
		return nil
	}
	st.lastNotification = now
	evt := &CertificateRenewalEscalatedEvent{Domain: domain, EscalationID: st.escalationID, EscalationType: st.escalationType, FailureCount: st.failures, LastError: errMsg, Timestamp: now}
	m.notify(ctx, evt)
	if m.eventEmitter != nil {
		_ = m.eventEmitter(ctx, evt)
	}
	return evt
}

// CertificateRenewalEscalationRecoveredEvent signifies an escalation resolved.
type CertificateRenewalEscalationRecoveredEvent struct {
	Domain       string
	EscalationID string
	ResolvedAt   time.Time
}

func (e *CertificateRenewalEscalationRecoveredEvent) EventType() string {
	return "certificate.renewal.escalation.recovered"
}
func (e *CertificateRenewalEscalationRecoveredEvent) EventSource() string {
	return "modular.letsencrypt"
}
func (e *CertificateRenewalEscalationRecoveredEvent) StructuredFields() map[string]interface{} {
	return map[string]interface{}{"module": "letsencrypt", "event": e.EventType(), "domain": e.Domain, "escalation_id": e.EscalationID}
}

// contains reports whether substr is within s (simple helper; avoids pulling in strings package repeatedly)
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
