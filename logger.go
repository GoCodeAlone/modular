package modular

// Logger defines the interface for application logging.
// The modular framework uses structured logging with key-value pairs
// to provide consistent, parseable log output across all modules.
//
// All framework operations (module initialization, service registration,
// dependency resolution, etc.) are logged using this interface, so
// implementing applications can control how framework logs appear.
//
// The Logger interface uses variadic arguments in key-value pairs:
//   logger.Info("message", "key1", "value1", "key2", "value2")
//
// This approach is compatible with popular structured logging libraries
// like slog, logrus, zap, and others.
//
// Example implementation using Go's standard log/slog:
//   type SlogLogger struct {
//       logger *slog.Logger
//   }
//   
//   func (l *SlogLogger) Info(msg string, args ...any) {
//       l.logger.Info(msg, args...)
//   }
//   
//   func (l *SlogLogger) Error(msg string, args ...any) {
//       l.logger.Error(msg, args...)
//   }
//   
//   func (l *SlogLogger) Warn(msg string, args ...any) {
//       l.logger.Warn(msg, args...)
//   }
//   
//   func (l *SlogLogger) Debug(msg string, args ...any) {
//       l.logger.Debug(msg, args...)
//   }
type Logger interface {
	// Info logs an informational message with optional key-value pairs.
	// Used for normal application events like module startup, service registration, etc.
	//
	// Example:
	//   logger.Info("Module initialized", "module", "database", "version", "1.2.3")
	Info(msg string, args ...any)

	// Error logs an error message with optional key-value pairs.
	// Used for errors that don't prevent application startup but should be noted.
	//
	// Example:
	//   logger.Error("Failed to connect to service", "service", "cache", "error", err)
	Error(msg string, args ...any)

	// Warn logs a warning message with optional key-value pairs.
	// Used for conditions that are unusual but don't prevent normal operation.
	//
	// Example:
	//   logger.Warn("Service unavailable, using fallback", "service", "external-api")
	Warn(msg string, args ...any)

	// Debug logs a debug message with optional key-value pairs.
	// Used for detailed diagnostic information, typically disabled in production.
	//
	// Example:
	//   logger.Debug("Dependency resolved", "from", "module1", "to", "module2")
	Debug(msg string, args ...any)
}
