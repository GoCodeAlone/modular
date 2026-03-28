package scheduler

// Event type constants for scheduler module events.
// Following CloudEvents specification reverse domain notation.
const (
	// Configuration events
	EventTypeConfigLoaded    = "com.modular.scheduler.config.loaded"
	EventTypeConfigValidated = "com.modular.scheduler.config.validated"

	// Job lifecycle events
	EventTypeJobScheduled = "com.modular.scheduler.job.scheduled"
	EventTypeJobStarted   = "com.modular.scheduler.job.started"
	EventTypeJobCompleted = "com.modular.scheduler.job.completed"
	EventTypeJobFailed    = "com.modular.scheduler.job.failed"
	EventTypeJobCancelled = "com.modular.scheduler.job.cancelled"
	EventTypeJobRemoved   = "com.modular.scheduler.job.removed"

	// Scheduler events
	EventTypeSchedulerStarted = "com.modular.scheduler.scheduler.started"
	EventTypeSchedulerStopped = "com.modular.scheduler.scheduler.stopped"
	EventTypeSchedulerPaused  = "com.modular.scheduler.scheduler.paused"
	EventTypeSchedulerResumed = "com.modular.scheduler.scheduler.resumed"

	// Worker pool events
	EventTypeWorkerStarted = "com.modular.scheduler.worker.started"
	EventTypeWorkerStopped = "com.modular.scheduler.worker.stopped"
	EventTypeWorkerBusy    = "com.modular.scheduler.worker.busy"
	EventTypeWorkerIdle    = "com.modular.scheduler.worker.idle"

	// Module lifecycle events
	EventTypeModuleStarted = "com.modular.scheduler.module.started"
	EventTypeModuleStopped = "com.modular.scheduler.module.stopped"

	// Error events
	EventTypeError   = "com.modular.scheduler.error"
	EventTypeWarning = "com.modular.scheduler.warning"
)
