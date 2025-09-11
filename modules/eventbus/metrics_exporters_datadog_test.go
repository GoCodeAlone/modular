package eventbus

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/CrisisTextLine/modular"
)

// TestDatadogStatsdExporterBasic spins up an in-process UDP listener to capture
// DogStatsD packets and verifies delivered/dropped metrics plus aggregate are emitted.
func TestDatadogStatsdExporterBasic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start UDP listener
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve udp: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer conn.Close()

	// Channel to collect raw lines
	linesCh := make(chan string, 64)
	go func() {
		buf := make([]byte, 65535)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, _, rerr := conn.ReadFromUDP(buf)
			if rerr != nil {
				return
			}
			scanner := bufio.NewScanner(strings.NewReader(string(buf[:n])))
			for scanner.Scan() {
				linesCh <- scanner.Text()
			}
		}
	}()

	// Build minimal modular application and properly initialized eventbus module
	logger := &testLogger{}
	mainCfg := modular.NewStdConfigProvider(struct{}{})
	app := modular.NewObservableApplication(mainCfg, logger)

	modIface := NewModule()
	mod := modIface.(*EventBusModule)
	app.RegisterModule(mod)
	app.RegisterConfigSection("eventbus", modular.NewStdConfigProvider(&EventBusConfig{Engine: "memory"}))

	if err := mod.Init(app); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := mod.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = mod.Stop(context.Background()) }()

	// Create a subscriber to ensure delivery
	_, err = mod.Subscribe(ctx, "foo.bar", func(ctx context.Context, e Event) error { return nil })
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Publish a few events
	for i := 0; i < 5; i++ {
		if err := mod.Publish(ctx, "foo.bar", map[string]int{"i": i}); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	// Start exporter with short interval (100ms) to ensure at least one flush before capture deadline
	exporter, err := NewDatadogStatsdExporter(mod, "eventbus", conn.LocalAddr().String(), 100*time.Millisecond, []string{"env:test"})
	if err != nil {
		t.Fatalf("exporter create: %v", err)
	}
	defer func() { _ = exporter.Close() }()

	// Manually flush once (no ticker goroutine required)
	exporter.flush()
	if f, ok := interface{}(exporter.client).(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
	// Allow UDP packet arrival
	time.Sleep(200 * time.Millisecond)

	var captured []string
	deadline := time.After(800 * time.Millisecond)
forLoop:
	for {
		select {
		case l := <-linesCh:
			captured = append(captured, l)
			if len(captured) > 4 {
				break forLoop
			}
		case <-deadline:
			break forLoop
		}
	}
	if len(captured) == 0 {
		// Attempt second flush after more events
		for i := 0; i < 3; i++ {
			_ = mod.Publish(ctx, "foo.bar", map[string]int{"k": i})
		}
		exporter.flush()
		if f, ok := interface{}(exporter.client).(interface{ Flush() error }); ok {
			_ = f.Flush()
		}
		time.Sleep(200 * time.Millisecond)
		deadline2 := time.After(600 * time.Millisecond)
		for {
			select {
			case l := <-linesCh:
				captured = append(captured, l)
				if len(captured) > 4 {
					break
				}
			case <-deadline2:
				goto afterCollect
			}
		}
	}
afterCollect:
	if len(captured) == 0 {
		t.Fatalf("no statsd packets captured")
	}

	// Basic assertions: expect delivered_total & dropped_total and aggregate engine:_all tag
	var haveDelivered, haveDropped, haveAggregate bool
	for _, l := range captured {
		if strings.Contains(l, "delivered_total") && strings.Contains(l, "engine:") {
			haveDelivered = true
		}
		if strings.Contains(l, "dropped_total") && strings.Contains(l, "engine:") {
			haveDropped = true // may be zero but still emitted
		}
		if strings.Contains(l, "engine:_all") {
			haveAggregate = true
		}
	}
	if !haveDelivered {
		t.Errorf("expected delivered_total metric line, got: %+v", captured)
	}
	if !haveDropped {
		// permissible but we still expect at least one emission (could be zero but gauge sent)
		t.Errorf("expected dropped_total metric line, got: %+v", captured)
	}
	if !haveAggregate {
		t.Errorf("expected aggregate engine:_all metric line, got: %+v", captured)
	}
}
