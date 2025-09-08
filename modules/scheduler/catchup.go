package scheduler

import "time"

// CatchUpConfig defines configuration for scheduler catch-up behavior
type CatchUpConfig struct {
	Enabled         bool
	MaxCatchUpTasks int
	CatchUpWindow   time.Duration
}

// WithSchedulerCatchUp creates a scheduler option for configuring catch-up behavior
func WithSchedulerCatchUp(config CatchUpConfig) SchedulerOption {
	return func(s *Scheduler) {
		if s.catchUpConfig == nil {
			s.catchUpConfig = &CatchUpConfig{}
		}
		*s.catchUpConfig = config
	}
}