package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithSchedulerCatchUpOption(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "should_define_with_scheduler_catchup_option",
			testFunc: func(t *testing.T) {
				config := CatchUpConfig{
					Enabled:         true,
					MaxCatchUpTasks: 100,
					CatchUpWindow:   24 * time.Hour,
				}

				option := WithSchedulerCatchUp(config)
				assert.NotNil(t, option, "WithSchedulerCatchUp should return option")
			},
		},
		{
			name: "should_configure_catchup_behavior",
			testFunc: func(t *testing.T) {
				config := CatchUpConfig{
					Enabled:         true,
					MaxCatchUpTasks: 50,
					CatchUpWindow:   12 * time.Hour,
				}

				jobStore := NewMemoryJobStore(24 * time.Hour)
				scheduler := NewScheduler(jobStore)
				err := scheduler.ApplyOption(WithSchedulerCatchUp(config))
				assert.NoError(t, err, "Should apply catchup option")

				catchUpEnabled := scheduler.IsCatchUpEnabled()
				assert.True(t, catchUpEnabled, "Catchup should be enabled")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}
