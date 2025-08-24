package modular

import "context"

// internal key type to avoid collisions
type syncNotifyCtxKey struct{}

var syncKey = syncNotifyCtxKey{}

// WithSynchronousNotification marks the context to request synchronous observer delivery.
// Subjects may honor this hint to deliver events inline instead of spawning goroutines.
func WithSynchronousNotification(ctx context.Context) context.Context {
	return context.WithValue(ctx, syncKey, true)
}

// IsSynchronousNotification returns true if the context requests synchronous delivery.
func IsSynchronousNotification(ctx context.Context) bool {
	v, _ := ctx.Value(syncKey).(bool)
	return v
}
