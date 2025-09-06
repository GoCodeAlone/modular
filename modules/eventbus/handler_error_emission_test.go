package eventbus

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/GoCodeAlone/modular"
	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// simpleSubject captures emitted events for inspection.
type simpleSubject struct {
	mu     sync.Mutex
	events []cloudevents.Event
	regs   []observerReg
}
type observerReg struct {
	o     modular.Observer
	types []string
	at    time.Time
}

func (s *simpleSubject) RegisterObserver(o modular.Observer, eventTypes ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.regs = append(s.regs, observerReg{o: o, types: eventTypes, at: time.Now()})
	return nil
}
func (s *simpleSubject) UnregisterObserver(o modular.Observer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.regs {
		if r.o.ObserverID() == o.ObserverID() {
			s.regs = append(s.regs[:i], s.regs[i+1:]...)
			break
		}
	}
	return nil
}
func (s *simpleSubject) NotifyObservers(ctx context.Context, e cloudevents.Event) error {
	s.mu.Lock()
	regs := append([]observerReg(nil), s.regs...)
	s.events = append(s.events, e)
	s.mu.Unlock()
	for _, r := range regs {
		if len(r.types) == 0 {
			_ = r.o.OnEvent(ctx, e)
			continue
		}
		for _, t := range r.types {
			if t == e.Type() {
				_ = r.o.OnEvent(ctx, e)
				break
			}
		}
	}
	return nil
}
func (s *simpleSubject) GetObservers() []modular.ObserverInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]modular.ObserverInfo, 0, len(s.regs))
	for _, r := range s.regs {
		out = append(out, modular.ObserverInfo{ID: r.o.ObserverID(), EventTypes: r.types, RegisteredAt: r.at})
	}
	return out
}

// TestHandlerErrorEmitsFailed verifies that a failing handler triggers MessageFailed event.
func TestHandlerErrorEmitsFailed(t *testing.T) {
	cfg := &EventBusConfig{Engine: "memory", MaxEventQueueSize: 10, DefaultEventBufferSize: 1, WorkerCount: 1}
	_ = cfg.ValidateConfig()
	mod := NewModule().(*EventBusModule)
	mod.config = cfg
	// Build router and set without calling Init (avoids logger usage before set)
	router, err := NewEngineRouter(cfg)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	mod.router = router
	// Provide a no-op logger and module reference for memory engine event emission
	mod.logger = noopLogger{}
	router.SetModuleReference(mod)
	subj := &simpleSubject{}
	// Directly set subject since RegisterObservers just stores it
	_ = mod.RegisterObservers(subj)
	if err := mod.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer mod.Stop(context.Background())
	topic := "err.topic"
	_, err = mod.Subscribe(context.Background(), topic, func(ctx context.Context, event Event) error { return errors.New("boom") })
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	_ = mod.Publish(context.Background(), topic, "payload")
	time.Sleep(50 * time.Millisecond)
	subj.mu.Lock()
	defer subj.mu.Unlock()
	found := false
	for _, e := range subj.events {
		if e.Type() == EventTypeMessageFailed {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected EventTypeMessageFailed emission")
	}
}
