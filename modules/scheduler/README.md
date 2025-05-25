# Scheduler Module

The Scheduler Module provides job scheduling capabilities for Modular applications. It supports one-time and recurring jobs using cron syntax with comprehensive job history tracking.

## Features

- Schedule one-time jobs to run at a specific time
- Schedule recurring jobs using cron expressions
- Configurable worker pool for job execution
- Job status tracking and history
- Memory-based job storage with optional persistence
- Graceful shutdown with configurable timeout

## Installation

```go
import (
    "github.com/GoCodeAlone/modular"
    "github.com/GoCodeAlone/modular/modules/scheduler"
)

// Register the scheduler module with your Modular application
app.RegisterModule(scheduler.NewModule())
```

## Configuration

The scheduler module can be configured using the following options:

```yaml
scheduler:
  workerCount: 5           # Number of worker goroutines to run jobs
  queueSize: 100           # Maximum size of the job queue
  shutdownTimeout: 30      # Time in seconds to wait for graceful shutdown
  storageType: memory      # Type of job storage (memory, file)
  checkInterval: 1         # How often to check for scheduled jobs (seconds)
  retentionDays: 7         # How many days to retain job history
  persistenceFile: "scheduler_jobs.json"  # File path for job persistence
  enablePersistence: false # Whether to persist jobs between restarts
```

## Usage

### Accessing the Scheduler Service

```go
// In your module's Init function
func (m *MyModule) Init(app modular.Application) error {
    var schedulerService *scheduler.SchedulerModule
    err := app.GetService("scheduler.provider", &schedulerService)
    if err != nil {
        return fmt.Errorf("failed to get scheduler service: %w", err)
    }
    
    // Now you can use the scheduler service
    m.scheduler = schedulerService
    return nil
}
```

### Using Interface-Based Service Matching

```go
// Define the service dependency
func (m *MyModule) RequiresServices() []modular.ServiceDependency {
    return []modular.ServiceDependency{
        {
            Name:               "scheduler",
            Required:           true,
            MatchByInterface:   true,
            SatisfiesInterface: reflect.TypeOf((*scheduler.SchedulerModule)(nil)).Elem(),
        },
    }
}

// Access the service in your constructor
func (m *MyModule) Constructor() modular.ModuleConstructor {
    return func(app modular.Application, services map[string]any) (modular.Module, error) {
        schedulerService := services["scheduler"].(*scheduler.SchedulerModule)
        return &MyModule{scheduler: schedulerService}, nil
    }
}
```

### Scheduling One-Time Jobs

```go
// Schedule a job to run once at a specific time
jobID, err := schedulerService.ScheduleJob(scheduler.Job{
    Name:    "data-cleanup",
    RunAt:   time.Now().Add(1 * time.Hour),
    JobFunc: func(ctx context.Context) error {
        // Your job logic here
        return nil
    },
})

if err != nil {
    // Handle error
}
```

### Scheduling Recurring Jobs

```go
// Schedule a job to run every minute
jobID, err := schedulerService.ScheduleRecurring(
    "log-metrics",           // Job name
    "0 * * * * *",           // Cron expression (every minute)
    func(ctx context.Context) error {
        // Your job logic here
        return nil
    },
)

if err != nil {
    // Handle error
}
```

### Managing Jobs

```go
// Cancel a job
err := schedulerService.CancelJob(jobID)

// Get job status
job, err := schedulerService.GetJob(jobID)
if err == nil {
    fmt.Printf("Job status: %s\n", job.Status)
    if job.LastRun != nil {
        fmt.Printf("Last run: %s\n", job.LastRun.Format(time.RFC3339))
    }
    if job.NextRun != nil {
        fmt.Printf("Next run: %s\n", job.NextRun.Format(time.RFC3339))
    }
}

// List all jobs
jobs, err := schedulerService.ListJobs()
for _, job := range jobs {
    fmt.Printf("Job: %s, Status: %s\n", job.Name, job.Status)
}

// Get job execution history
history, err := schedulerService.GetJobHistory(jobID)
for _, exec := range history {
    fmt.Printf("Execution: %s, Status: %s\n", 
        exec.StartTime.Format(time.RFC3339),
        exec.Status)
}
```

## Cron Expression Format

The scheduler uses standard cron expressions with seconds:

```
┌───────────── seconds (0-59)
│ ┌───────────── minute (0-59)
│ │ ┌───────────── hour (0-23)
│ │ │ ┌───────────── day of month (1-31)
│ │ │ │ ┌───────────── month (1-12)
│ │ │ │ │ ┌───────────── day of week (0-6) (Sunday to Saturday)
│ │ │ │ │ │
* * * * * *
```

Examples:
- `0 0 * * * *` - Every hour at 0 minutes 0 seconds
- `0 */5 * * * *` - Every 5 minutes
- `0 0 8 * * *` - Every day at 8:00 AM
- `0 0 12 * * 1-5` - Every weekday at noon

## Implementation Notes

- The scheduler uses a worker pool model to process jobs concurrently
- Each recurring job is registered with a cron scheduler
- Job executions are tracked for history and reporting
- The module supports graceful shutdown, completing in-progress jobs

## Testing

The scheduler module includes comprehensive tests for both module integration and job scheduling logic.