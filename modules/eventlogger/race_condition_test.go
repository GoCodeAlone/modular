package eventlogger

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// TestRaceCondition_StartStopConcurrency reproduces the race condition
// between emitStartupOperationalEvents goroutine and Stop() method.
// This test will fail with -race flag before the fix.
//
// The race occurs when:
// 1. Start() launches emitStartupOperationalEvents in a goroutine
// 2. Stop() is called concurrently and modifies m.started under write lock
// 3. emitStartupOperationalEvents reads m.started without synchronization
func TestRaceCondition_StartStopConcurrency(t *testing.T) {
	// Run multiple iterations to increase chance of detecting race
	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			// Create mock application
			app := &MockApplication{
				configSections: make(map[string]modular.ConfigProvider),
				logger:         &MockLogger{},
			}

			// Create and initialize module
			module := NewModule().(*EventLoggerModule)
			err := module.RegisterConfig(app)
			if err != nil {
				t.Fatalf("Failed to register config: %v", err)
			}

			err = module.Init(app)
			if err != nil {
				t.Fatalf("Failed to initialize module: %v", err)
			}

			// Create a wait group to synchronize concurrent operations
			var wg sync.WaitGroup
			wg.Add(2)

			ctx := context.Background()

			// Channel to signal when Start has begun
			startedCh := make(chan struct{})

			// Start the module in one goroutine
			go func() {
				defer wg.Done()
				// Signal that Start is about to be called
				close(startedCh)
				if err := module.Start(ctx); err != nil {
					t.Errorf("Start failed: %v", err)
				}
			}()

			// Stop the module immediately in another goroutine
			// This creates the race condition with emitStartupOperationalEvents
			go func() {
				defer wg.Done()
				// Wait for Start to begin before calling Stop
				<-startedCh
				if err := module.Stop(ctx); err != nil {
					t.Errorf("Stop failed: %v", err)
				}
			}()

			wg.Wait()
		})
	}
}

// TestRaceCondition_StartStopWithSubject tests the race condition
// when the module has a subject registered (more realistic scenario)
func TestRaceCondition_StartStopWithSubject(t *testing.T) {
	for i := 0; i < 50; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			app := &MockApplication{
				configSections: make(map[string]modular.ConfigProvider),
				logger:         &MockLogger{},
			}

			module := NewModule().(*EventLoggerModule)
			err := module.RegisterConfig(app)
			if err != nil {
				t.Fatalf("Failed to register config: %v", err)
			}

			err = module.Init(app)
			if err != nil {
				t.Fatalf("Failed to initialize module: %v", err)
			}

			// Simulate RegisterObservers being called
			mockSubject := &MockSubject{}
			if err := module.RegisterObservers(mockSubject); err != nil {
				t.Fatalf("Failed to register observers: %v", err)
			}

			var wg sync.WaitGroup
			wg.Add(2)

			ctx := context.Background()

			// Channel to signal when Start has begun
			startedCh := make(chan struct{})

			// Concurrent Start and Stop
			go func() {
				defer wg.Done()
				// Signal that Start is about to be called
				close(startedCh)
				if err := module.Start(ctx); err != nil {
					t.Errorf("Start failed: %v", err)
				}
			}()

			go func() {
				defer wg.Done()
				// Wait for Start to begin before calling Stop
				<-startedCh
				if err := module.Stop(ctx); err != nil {
					t.Errorf("Stop failed: %v", err)
				}
			}()

			wg.Wait()
		})
	}
}

// MockSubject for testing observer registration
type MockSubject struct {
	mu        sync.RWMutex
	observers []modular.Observer
}

func (s *MockSubject) RegisterObserver(observer modular.Observer, eventTypes ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observers = append(s.observers, observer)
	return nil
}

func (s *MockSubject) UnregisterObserver(observer modular.Observer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, obs := range s.observers {
		if obs.ObserverID() == observer.ObserverID() {
			s.observers = append(s.observers[:i], s.observers[i+1:]...)
			return nil
		}
	}
	return nil
}

func (s *MockSubject) NotifyObservers(ctx context.Context, event cloudevents.Event) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return nil
}

func (s *MockSubject) GetObservers() []modular.ObserverInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	infos := make([]modular.ObserverInfo, 0, len(s.observers))
	for _, obs := range s.observers {
		infos = append(infos, modular.ObserverInfo{
			ID:           obs.ObserverID(),
			EventTypes:   []string{},
			RegisteredAt: time.Now(),
		})
	}
	return infos
}
