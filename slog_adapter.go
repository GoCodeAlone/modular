package modular

import "log/slog"

// SlogAdapter wraps a *slog.Logger to implement the Logger interface.
type SlogAdapter struct {
	logger *slog.Logger
}

// NewSlogAdapter creates a new SlogAdapter wrapping the given slog.Logger.
func NewSlogAdapter(l *slog.Logger) *SlogAdapter {
	return &SlogAdapter{logger: l}
}

func (a *SlogAdapter) Info(msg string, args ...any)  { a.logger.Info(msg, args...) }
func (a *SlogAdapter) Error(msg string, args ...any) { a.logger.Error(msg, args...) }
func (a *SlogAdapter) Warn(msg string, args ...any)  { a.logger.Warn(msg, args...) }
func (a *SlogAdapter) Debug(msg string, args ...any) { a.logger.Debug(msg, args...) }

// With returns a new SlogAdapter with the given key-value pairs added to the context.
func (a *SlogAdapter) With(args ...any) *SlogAdapter {
	return &SlogAdapter{logger: a.logger.With(args...)}
}

// WithGroup returns a new SlogAdapter with the given group name.
func (a *SlogAdapter) WithGroup(name string) *SlogAdapter {
	return &SlogAdapter{logger: a.logger.WithGroup(name)}
}
