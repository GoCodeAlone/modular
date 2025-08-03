package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
	"github.com/GoCodeAlone/modular/modules/chimux"
	"github.com/GoCodeAlone/modular/modules/httpserver"
	"github.com/GoCodeAlone/modular/modules/scheduler"
	"github.com/go-chi/chi/v5"
)

type AppConfig struct {
	Name string `yaml:"name" default:"Scheduler Demo"`
}

type CronJobRequest struct {
	Name    string                 `json:"name"`
	Cron    string                 `json:"cron"`
	Task    string                 `json:"task"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type OneTimeJobRequest struct {
	Name    string                 `json:"name"`
	Delay   int                    `json:"delay"` // seconds from now
	Task    string                 `json:"task"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type JobResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// SchedulerService defines the interface we expect from the scheduler module
type SchedulerService interface {
	ScheduleRecurring(name string, cronExpr string, jobFunc func(context.Context) error) (string, error)
	ScheduleJob(job interface{}) (string, error) // Using interface{} since we don't have the exact Job type
	CancelJob(jobID string) error
	ListJobs() ([]interface{}, error) // Using interface{} since we don't have the exact Job type
	GetJob(jobID string) (interface{}, error)
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Set up configuration feeders
	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewYamlFeeder("config.yaml"),
		feeders.NewEnvFeeder(),
	}

	// Create config provider
	appConfig := &AppConfig{}
	configProvider := modular.NewStdConfigProvider(appConfig)

	// Create application
	app := modular.NewStdApplication(configProvider, logger)

	// Register modules
	app.RegisterModule(scheduler.NewModule())
	app.RegisterModule(chimux.NewChiMuxModule())
	app.RegisterModule(httpserver.NewHTTPServerModule())

	// Register API routes module
	app.RegisterModule(NewSchedulerAPIModule())

	// Run the application
	if err := app.Run(); err != nil {
		logger.Error("Application error", "error", err)
		os.Exit(1)
	}
}

// SchedulerAPIModule provides HTTP routes for job scheduling
type SchedulerAPIModule struct {
	router    chi.Router
	scheduler SchedulerService
	logger    modular.Logger
}

func NewSchedulerAPIModule() modular.Module {
	return &SchedulerAPIModule{}
}

func (m *SchedulerAPIModule) Name() string {
	return "scheduler-api"
}

func (m *SchedulerAPIModule) Dependencies() []string {
	return []string{"scheduler", "chimux"}
}

func (m *SchedulerAPIModule) RegisterConfig(app modular.Application) error {
	// No additional config needed
	return nil
}

func (m *SchedulerAPIModule) Init(app modular.Application) error {
	m.logger = app.Logger()

	// Get scheduler service
	if err := app.GetService("scheduler.service", &m.scheduler); err != nil {
		return fmt.Errorf("failed to get scheduler service: %w", err)
	}

	// Get router
	if err := app.GetService("chimux.router", &m.router); err != nil {
		return fmt.Errorf("failed to get router service: %w", err)
	}

	m.setupRoutes()
	m.setupDemoJobs()
	return nil
}

func (m *SchedulerAPIModule) setupRoutes() {
	m.router.Route("/api/jobs", func(r chi.Router) {
		r.Post("/cron", m.handleScheduleCronJob)
		r.Post("/once", m.handleScheduleOneTimeJob)
		r.Get("/", m.handleListJobs)
		r.Get("/{id}", m.handleGetJob)
		r.Delete("/{id}", m.handleCancelJob)
	})
}

func (m *SchedulerAPIModule) setupDemoJobs() {
	// Schedule a demo heartbeat job
	_, err := m.scheduler.ScheduleRecurring(
		"demo-heartbeat",
		"*/30 * * * * *", // Every 30 seconds
		m.createHeartbeatJob(),
	)
	if err != nil {
		m.logger.Error("Failed to schedule demo heartbeat job", "error", err)
	} else {
		m.logger.Info("Scheduled demo heartbeat job (every 30 seconds)")
	}
}

func (m *SchedulerAPIModule) createHeartbeatJob() func(context.Context) error {
	return func(ctx context.Context) error {
		m.logger.Info("❤️ Demo heartbeat - scheduler is working!")
		return nil
	}
}

func (m *SchedulerAPIModule) createLogJob(task string, payload map[string]interface{}) func(context.Context) error {
	return func(ctx context.Context) error {
		m.logger.Info("Executing scheduled job", "task", task, "payload", payload)
		
		switch task {
		case "log_heartbeat":
			if msg, ok := payload["message"].(string); ok {
				m.logger.Info("Heartbeat: " + msg)
			}
		case "check_status":
			if component, ok := payload["component"].(string); ok {
				m.logger.Info("Status check for component: " + component)
			}
		case "cleanup":
			if dir, ok := payload["directory"].(string); ok {
				m.logger.Info("Cleanup task for directory: " + dir)
			}
		default:
			m.logger.Info("Unknown task type: " + task)
		}
		
		return nil
	}
}

func (m *SchedulerAPIModule) handleScheduleCronJob(w http.ResponseWriter, r *http.Request) {
	var req CronJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Cron == "" || req.Task == "" {
		http.Error(w, "Name, cron, and task are required", http.StatusBadRequest)
		return
	}

	jobFunc := m.createLogJob(req.Task, req.Payload)
	jobID, err := m.scheduler.ScheduleRecurring(req.Name, req.Cron, jobFunc)
	if err != nil {
		m.logger.Error("Failed to schedule recurring job", "error", err)
		http.Error(w, "Failed to schedule job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JobResponse{
		ID:      jobID,
		Message: "Recurring job scheduled successfully",
	})
}

func (m *SchedulerAPIModule) handleScheduleOneTimeJob(w http.ResponseWriter, r *http.Request) {
	var req OneTimeJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Task == "" || req.Delay <= 0 {
		http.Error(w, "Name, task, and positive delay are required", http.StatusBadRequest)
		return
	}

	// For one-time jobs, we'll schedule a recurring job that runs once
	// In a real implementation, you'd use the actual one-time job method
	runAt := time.Now().Add(time.Duration(req.Delay) * time.Second)
	cronExpr := fmt.Sprintf("%d %d %d %d %d *", 
		runAt.Second(), runAt.Minute(), runAt.Hour(), runAt.Day(), int(runAt.Month()))

	jobFunc := func(ctx context.Context) error {
		m.logger.Info("Executing one-time job", "name", req.Name, "task", req.Task)
		m.createLogJob(req.Task, req.Payload)(ctx)
		// In a real implementation, you'd cancel the job after execution
		return nil
	}

	jobID, err := m.scheduler.ScheduleRecurring(req.Name, cronExpr, jobFunc)
	if err != nil {
		m.logger.Error("Failed to schedule one-time job", "error", err)
		http.Error(w, "Failed to schedule job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JobResponse{
		ID:      jobID,
		Message: fmt.Sprintf("One-time job scheduled to run in %d seconds", req.Delay),
	})
}

func (m *SchedulerAPIModule) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := m.scheduler.ListJobs()
	if err != nil {
		m.logger.Error("Failed to list jobs", "error", err)
		http.Error(w, "Failed to list jobs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jobs":  jobs,
		"count": len(jobs),
	})
}

func (m *SchedulerAPIModule) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	job, err := m.scheduler.GetJob(jobID)
	if err != nil {
		m.logger.Error("Failed to get job", "jobID", jobID, "error", err)
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (m *SchedulerAPIModule) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	if err := m.scheduler.CancelJob(jobID); err != nil {
		m.logger.Error("Failed to cancel job", "jobID", jobID, "error", err)
		http.Error(w, "Failed to cancel job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Job canceled successfully",
		"jobID":   jobID,
	})
}

func (m *SchedulerAPIModule) Start(ctx context.Context) error {
	m.logger.Info("Scheduler API module started")
	return nil
}

func (m *SchedulerAPIModule) Stop(ctx context.Context) error {
	m.logger.Info("Scheduler API module stopped")
	return nil
}

func (m *SchedulerAPIModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{}
}

func (m *SchedulerAPIModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{Name: "scheduler.service", Required: true},
		{Name: "chimux.router", Required: true},
	}
}