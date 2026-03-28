package scheduler

import (
	"errors"
	"time"
)

// Persistence errors
var (
	ErrNoPersistenceHandler      = errors.New("no persistence handler configured")
	ErrUnknownPersistenceBackend = errors.New("unknown persistence backend")
)

// PersistenceBackend defines the type of persistence backend to use
type PersistenceBackend string

const (
	// PersistenceBackendNone disables persistence - jobs are lost on restart
	PersistenceBackendNone PersistenceBackend = "none"
	// PersistenceBackendMemory uses memory-based storage (for testing)
	PersistenceBackendMemory PersistenceBackend = "memory"
	// PersistenceBackendCustom allows injection of custom persistence handlers
	PersistenceBackendCustom PersistenceBackend = "custom"
)

// PersistenceHandler defines the interface for custom persistence backends
type PersistenceHandler interface {
	// Save persists jobs to the configured backend
	Save(jobs []Job) error
	// Load retrieves jobs from the configured backend
	Load() ([]Job, error)
}

// SchedulerConfig defines the configuration for the scheduler module
type SchedulerConfig struct {
	// WorkerCount is the number of worker goroutines to run
	WorkerCount int `json:"workerCount" yaml:"workerCount" validate:"min=1" env:"WORKER_COUNT"`

	// QueueSize is the maximum number of jobs to queue
	QueueSize int `json:"queueSize" yaml:"queueSize" validate:"min=1" env:"QUEUE_SIZE"`

	// ShutdownTimeout is the time to wait for graceful shutdown
	ShutdownTimeout time.Duration `json:"shutdownTimeout" yaml:"shutdownTimeout" env:"SHUTDOWN_TIMEOUT"`

	// StorageType is the type of job storage to use (memory only)
	StorageType string `json:"storageType" yaml:"storageType" validate:"oneof=memory" env:"STORAGE_TYPE" default:"memory"`

	// CheckInterval is how often to check for scheduled jobs
	CheckInterval time.Duration `json:"checkInterval" yaml:"checkInterval" env:"CHECK_INTERVAL"`

	// RetentionDays is how many days to retain job history
	RetentionDays int `json:"retentionDays" yaml:"retentionDays" validate:"min=1" env:"RETENTION_DAYS"`

	// PersistenceBackend determines the type of persistence to use
	PersistenceBackend PersistenceBackend `json:"persistenceBackend" yaml:"persistenceBackend" env:"PERSISTENCE_BACKEND" default:"none"`

	// PersistenceHandler allows injection of custom persistence logic
	// This field is not serializable and must be set programmatically
	PersistenceHandler PersistenceHandler `json:"-" yaml:"-"`
}
