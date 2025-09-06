package modular

import (
	"context"
	"sync"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

// TestObserverRegistrationOrdering ensures that events emitted before registration are not
// delivered retroactively, while events after registration are, and concurrent registrations
// do not introduce data races or missed notifications.
func TestObserverRegistrationOrdering(t *testing.T) {
	t.Parallel()

	logger := &TestObserverLogger{}
	app := NewObservableApplication(NewStdConfigProvider(&struct{}{}), logger)

	baseCtx := context.Background()
	syncCtx := WithSynchronousNotification(baseCtx)

	var mu sync.Mutex
	received := make([]cloudevents.Event, 0, 8)

	obs := NewFunctionalObserver("ordering-observer", func(ctx context.Context, evt cloudevents.Event) error {
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		return nil
	})

	// Emit an event BEFORE registering the observer – should not be seen later.
	early := NewCloudEvent("test.early", "test", nil, nil)
	if err := app.NotifyObservers(syncCtx, early); err != nil {
		t.Fatalf("unexpected error emitting early event: %v", err)
	}

	// Register observer.
	if err := app.RegisterObserver(obs); err != nil {
		t.Fatalf("register observer: %v", err)
	}

	// Emit events after registration synchronously so we can assert without sleeps.
	post := []cloudevents.Event{
		NewCloudEvent("test.one", "test", nil, nil),
		NewCloudEvent("test.two", "test", nil, nil),
	}
	for _, e := range post {
		if err := app.NotifyObservers(syncCtx, e); err != nil {
			t.Fatalf("notify post event %s: %v", e.Type(), err)
		}
	}

	// Emit another event concurrently with a late registration of a second observer.
	// The snapshot semantics mean the second observer may or may not get this event; we only
	// assert first observer always does and no early events leak.
	lateObsReceived := make([]cloudevents.Event, 0, 1)
	var lateMu sync.Mutex
	lateObs := NewFunctionalObserver("late-observer", func(ctx context.Context, evt cloudevents.Event) error {
		if evt.Type() == "test.concurrent" {
			lateMu.Lock()
			lateObsReceived = append(lateObsReceived, evt)
			lateMu.Unlock()
		}
		return nil
	})

	done := make(chan struct{})
	go func() {
		_ = app.RegisterObserver(lateObs)
		close(done)
	}()

	conc := NewCloudEvent("test.concurrent", "test", nil, nil)
	if err := app.NotifyObservers(syncCtx, conc); err != nil {
		t.Fatalf("notify concurrent event: %v", err)
	}
	<-done // ensure registration attempt complete (ordering not guaranteed)

	mu.Lock()
	got := append([]cloudevents.Event(nil), received...)
	mu.Unlock()

	// Assertions: early event must not appear
	for _, e := range got {
		if e.Type() == "test.early" {
			t.Fatalf("received early event emitted before registration: %s", e.Type())
		}
	}

	// Ensure mandatory post-registration events were received by first observer.
	required := map[string]bool{"test.one": false, "test.two": false, "test.concurrent": false}
	for _, e := range got {
		if _, ok := required[e.Type()]; ok {
			required[e.Type()] = true
		}
	}
	for typ, seen := range required {
		if !seen {
			t.Fatalf("missing expected event %s in first observer", typ)
		}
	}

	// Late observer integrity: at most one concurrent event.
	lateMu.Lock()
	if len(lateObsReceived) > 1 {
		t.Fatalf("late observer received unexpected duplicate events: %d", len(lateObsReceived))
	}
	lateMu.Unlock()
}
