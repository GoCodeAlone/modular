package letsencrypt

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockChannel records notifications for assertions.
type mockChannel struct {
	events []*CertificateRenewalEscalatedEvent
}

func (m *mockChannel) Notify(ctx context.Context, evt *CertificateRenewalEscalatedEvent) error {
	m.events = append(m.events, evt)
	return nil
}

// mockEmitter captures emitted events (both escalation & recovery) without coupling to framework observer.
type mockEmitter struct{ events []interface{} }

func (m *mockEmitter) emit(ctx context.Context, evt interface{}) error {
	m.events = append(m.events, evt)
	return nil
}

func TestEscalation_OnRepeatedFailures(t *testing.T) {
	em := &mockEmitter{}
	ch := &mockChannel{}
	cfg := EscalationConfig{FailureThreshold: 3, Window: time.Minute}
	mgr := NewEscalationManager(cfg, em.emit, WithNotificationChannels(ch))
	ctx := context.Background()

	// two failures below threshold
	evt, err := mgr.RecordFailure(ctx, "example.com", "validation failed")
	require.NoError(t, err)
	assert.Nil(t, evt)
	evt, err = mgr.RecordFailure(ctx, "example.com", "validation failed")
	require.NoError(t, err)
	assert.Nil(t, evt)
	// third triggers escalation
	evt, err = mgr.RecordFailure(ctx, "example.com", "validation failed")
	require.NoError(t, err)
	require.NotNil(t, evt, "expected escalation event")
	assert.Equal(t, EscalationTypeRetryExhausted, evt.EscalationType)
	assert.Equal(t, 3, evt.FailureCount)
	assert.Len(t, ch.events, 1, "notification channel should receive event once")
	stats := mgr.Stats()
	assert.Equal(t, 1, stats.TotalEscalations)
	assert.Equal(t, 1, stats.Reasons[EscalationTypeRetryExhausted])
}

func TestEscalation_RateLimitedACME(t *testing.T) {
	em := &mockEmitter{}
	cfg := EscalationConfig{RateLimitSubstring: "rateLimited"}
	mgr := NewEscalationManager(cfg, em.emit)
	ctx := context.Background()
	evt, err := mgr.HandleACMEError(ctx, "rl.example", "urn:ietf:params:acme:error:rateLimited: too many requests")
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, EscalationTypeRateLimited, evt.EscalationType)
}

func TestEscalation_ExpiringSoon(t *testing.T) {
	em := &mockEmitter{}
	cfg := EscalationConfig{ExpiringSoonDays: 10}
	mgr := NewEscalationManager(cfg, em.emit)
	ctx := context.Background()
	certInfo := &CertificateInfo{Domain: "expiring.example", DaysRemaining: 5}
	evt, err := mgr.CheckExpiration(ctx, certInfo.Domain, certInfo)
	require.NoError(t, err)
	require.NotNil(t, evt)
	assert.Equal(t, EscalationTypeExpiringSoon, evt.EscalationType)
}

func TestEscalation_NotificationCooldownAndAck(t *testing.T) {
	em := &mockEmitter{}
	ch := &mockChannel{}
	now := time.Now()
	fakeNow := now
	mgr := NewEscalationManager(EscalationConfig{FailureThreshold: 1, NotificationCooldown: 10 * time.Minute}, em.emit, WithNotificationChannels(ch), WithNow(func() time.Time { return fakeNow }))
	ctx := context.Background()
	// trigger first escalation
	evt, _ := mgr.RecordFailure(ctx, "cool.example", "error")
	require.NotNil(t, evt)
	require.Len(t, ch.events, 1)
	// attempt re-escalation inside cooldown -> no new notification
	fakeNow = fakeNow.Add(5 * time.Minute)
	_, _ = mgr.RecordFailure(ctx, "cool.example", "error again")
	assert.Len(t, ch.events, 1, "should not notify again inside cooldown")
	// advance past cooldown without ack -> new notification
	fakeNow = fakeNow.Add(6 * time.Minute)
	_, _ = mgr.RecordFailure(ctx, "cool.example", "error again2")
	assert.Len(t, ch.events, 2, "should notify again after cooldown")
	// acknowledge and advance beyond cooldown -> no notification
	mgr.Acknowledge("cool.example")
	fakeNow = fakeNow.Add(20 * time.Minute)
	_, _ = mgr.RecordFailure(ctx, "cool.example", "error again3")
	assert.Len(t, ch.events, 2, "acknowledged escalation should suppress further notifications")
}

func TestEscalation_Recovery(t *testing.T) {
	em := &mockEmitter{}
	cfg := EscalationConfig{FailureThreshold: 1}
	mgr := NewEscalationManager(cfg, em.emit)
	ctx := context.Background()
	evt, _ := mgr.RecordFailure(ctx, "recover.example", "boom")
	require.NotNil(t, evt)
	mgr.Clear(ctx, "recover.example")
	// Expect a recovery event emitted after escalation
	var foundRecovery bool
	for _, e := range em.events {
		if _, ok := e.(*CertificateRenewalEscalationRecoveredEvent); ok {
			foundRecovery = true
			break
		}
	}
	assert.True(t, foundRecovery, "expected recovery event")
	stats := mgr.Stats()
	assert.Equal(t, 1, stats.Resolutions)
}

func TestEscalation_MetricsReasonTracking(t *testing.T) {
	em := &mockEmitter{}
	mgr := NewEscalationManager(EscalationConfig{FailureThreshold: 1}, em.emit)
	ctx := context.Background()
	mgr.RecordFailure(ctx, "r1.example", "a")
	mgr.HandleACMEError(ctx, "r2.example", "acme error")
	mgr.HandleACMEError(ctx, "r3.example", "rateLimited hit")
	mgr.RecordTimeout(ctx, "r4.example", "timeout")
	stats := mgr.Stats()
	assert.GreaterOrEqual(t, stats.TotalEscalations, 4)
	assert.True(t, stats.Reasons[EscalationTypeRetryExhausted] >= 1)
	assert.True(t, stats.Reasons[EscalationTypeACMEError] >= 1)
	assert.True(t, stats.Reasons[EscalationTypeRateLimited] >= 1)
	assert.True(t, stats.Reasons[EscalationTypeValidationFailed] >= 1)
}
