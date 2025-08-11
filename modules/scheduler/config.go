package scheduler

import (
	"time"
)

// SchedulerConfig defines the configuration for the scheduler module
type SchedulerConfig struct {
	// WorkerCount is the number of worker goroutines to run
	WorkerCount int `json:"workerCount" yaml:"workerCount" validate:"min=1" env:"WORKER_COUNT"`

	// QueueSize is the maximum number of jobs to queue
	QueueSize int `json:"queueSize" yaml:"queueSize" validate:"min=1" env:"QUEUE_SIZE"`

	// ShutdownTimeout is the time to wait for graceful shutdown
	ShutdownTimeout time.Duration `json:"shutdownTimeout" yaml:"shutdownTimeout" env:"SHUTDOWN_TIMEOUT"`

	// StorageType is the type of job storage to use (memory, file, etc.)
	StorageType string `json:"storageType" yaml:"storageType" validate:"oneof=memory file" env:"STORAGE_TYPE"`

	// CheckInterval is how often to check for scheduled jobs
	CheckInterval time.Duration `json:"checkInterval" yaml:"checkInterval" env:"CHECK_INTERVAL"`

	// RetentionDays is how many days to retain job history
	RetentionDays int `json:"retentionDays" yaml:"retentionDays" validate:"min=1" env:"RETENTION_DAYS"`

	// PersistenceFile is the file path for job persistence
	PersistenceFile string `json:"persistenceFile" yaml:"persistenceFile" env:"PERSISTENCE_FILE"`

	// EnablePersistence determines if jobs should be persisted between restarts
	EnablePersistence bool `json:"enablePersistence" yaml:"enablePersistence" env:"ENABLE_PERSISTENCE"`
}
