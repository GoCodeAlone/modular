package scheduler

// SchedulerConfig defines the configuration for the scheduler module
type SchedulerConfig struct {
	// WorkerCount is the number of worker goroutines to run
	WorkerCount int `json:"workerCount" yaml:"workerCount" validate:"min=1"`

	// QueueSize is the maximum number of jobs to queue
	QueueSize int `json:"queueSize" yaml:"queueSize" validate:"min=1"`

	// ShutdownTimeout is the time in seconds to wait for graceful shutdown
	ShutdownTimeout int `json:"shutdownTimeout" yaml:"shutdownTimeout" validate:"min=1"`

	// StorageType is the type of job storage to use (memory, file, etc.)
	StorageType string `json:"storageType" yaml:"storageType" validate:"oneof=memory file"`

	// CheckInterval is how often to check for scheduled jobs (in seconds)
	CheckInterval int `json:"checkInterval" yaml:"checkInterval" validate:"min=1"`

	// RetentionDays is how many days to retain job history
	RetentionDays int `json:"retentionDays" yaml:"retentionDays" validate:"min=1"`

	// PersistenceFile is the file path for job persistence
	PersistenceFile string `json:"persistenceFile" yaml:"persistenceFile"`

	// EnablePersistence determines if jobs should be persisted between restarts
	EnablePersistence bool `json:"enablePersistence" yaml:"enablePersistence"`
}
