# Scheduler Module Demo

This example demonstrates how to use the scheduler module for job scheduling with cron expressions, one-time jobs, and job management.

## Overview

The example sets up:
- Cron-based recurring jobs with configurable schedules
- One-time scheduled jobs with specific execution times
- Job management: create, cancel, list, and monitor jobs
- HTTP API endpoints for job control
- Job history and status tracking

## Features Demonstrated

1. **Cron Jobs**: Schedule recurring tasks with cron expressions
2. **One-time Jobs**: Schedule tasks for specific future times
3. **Job Management**: Create, cancel, and monitor job execution
4. **HTTP Integration**: RESTful API for job scheduling
5. **Job History**: Track job execution history and results

## API Endpoints

- `POST /api/jobs/cron` - Schedule a recurring job with cron expression
- `POST /api/jobs/once` - Schedule a one-time job
- `GET /api/jobs` - List all scheduled jobs
- `GET /api/jobs/:id` - Get job details and history
- `DELETE /api/jobs/:id` - Cancel a scheduled job

## Running the Example

1. Start the application:
   ```bash
   go run main.go
   ```

2. The application will start on port 8080

## Testing Job Scheduling

### Schedule a recurring job (every minute)
```bash
curl -X POST http://localhost:8080/api/jobs/cron \
  -H "Content-Type: application/json" \
  -d '{
    "name": "heartbeat",
    "cron": "0 * * * * *",
    "task": "log_heartbeat",
    "payload": {"message": "System heartbeat"}
  }'
```

### Schedule a recurring job (every 30 seconds)
```bash
curl -X POST http://localhost:8080/api/jobs/cron \
  -H "Content-Type: application/json" \
  -d '{
    "name": "status_check",
    "cron": "*/30 * * * * *",
    "task": "check_status",
    "payload": {"component": "database"}
  }'
```

### Schedule a one-time job (5 minutes from now)
```bash
curl -X POST http://localhost:8080/api/jobs/once \
  -H "Content-Type: application/json" \
  -d '{
    "name": "cleanup_task",
    "delay": 300,
    "task": "cleanup",
    "payload": {"directory": "/tmp/cache"}
  }'
```

### List all jobs
```bash
curl http://localhost:8080/api/jobs
```

### Get job details
```bash
curl http://localhost:8080/api/jobs/{job-id}
```

### Cancel a job
```bash
curl -X DELETE http://localhost:8080/api/jobs/{job-id}
```

## Configuration

The scheduler module is configured in `config.yaml`:

```yaml
scheduler:
  worker_pool_size: 5
  max_concurrent_jobs: 10
  job_timeout: 300  # 5 minutes
  enable_persistence: false
  history_retention: 168  # 7 days in hours
```

## Job Types

The example includes several predefined job types:

### Log Heartbeat
- Simple logging job that outputs a heartbeat message
- Useful for monitoring application health

### Status Check
- Performs system status checks
- Can be configured to check different components

### Cleanup Task
- File system cleanup operations
- Configurable directories and retention policies

### Custom Jobs
- Extensible job system for adding new task types
- JSON payload support for job parameters

## Cron Expression Examples

- `0 * * * * *` - Every minute at second 0
- `*/30 * * * * *` - Every 30 seconds
- `0 0 * * * *` - Every hour at minute 0
- `0 0 6 * * *` - Every day at 6:00 AM
- `0 0 0 * * 1` - Every Monday at midnight

## Error Handling

The example includes proper error handling for:
- Invalid cron expressions
- Job scheduling conflicts
- Worker pool exhaustion
- Job execution timeouts
- Persistence failures

This demonstrates how to integrate job scheduling capabilities into modular applications for automated task execution.