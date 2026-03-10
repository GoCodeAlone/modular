package modular

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type parallelInitModule struct {
	name      string
	deps      []string
	initDelay time.Duration
	initCount *atomic.Int32
	maxPar    *atomic.Int32
	curPar    *atomic.Int32
}

func (m *parallelInitModule) Name() string           { return m.name }
func (m *parallelInitModule) Dependencies() []string { return m.deps }
func (m *parallelInitModule) Init(app Application) error {
	cur := m.curPar.Add(1)
	defer m.curPar.Add(-1)
	for {
		old := m.maxPar.Load()
		if cur <= old || m.maxPar.CompareAndSwap(old, cur) {
			break
		}
	}
	m.initCount.Add(1)
	time.Sleep(m.initDelay)
	return nil
}

func TestWithParallelInit_ConcurrentSameDepth(t *testing.T) {
	var initCount, maxPar, curPar atomic.Int32
	modA := &parallelInitModule{name: "a", initDelay: 50 * time.Millisecond, initCount: &initCount, maxPar: &maxPar, curPar: &curPar}
	modB := &parallelInitModule{name: "b", initDelay: 50 * time.Millisecond, initCount: &initCount, maxPar: &maxPar, curPar: &curPar}
	modC := &parallelInitModule{name: "c", initDelay: 50 * time.Millisecond, initCount: &initCount, maxPar: &maxPar, curPar: &curPar}

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithModules(modA, modB, modC),
		WithParallelInit(),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}

	start := time.Now()
	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	elapsed := time.Since(start)

	if initCount.Load() != 3 {
		t.Errorf("expected 3 inits, got %d", initCount.Load())
	}
	if maxPar.Load() < 2 {
		t.Errorf("expected at least 2 concurrent inits, got max %d", maxPar.Load())
	}
	if elapsed > 120*time.Millisecond {
		t.Errorf("expected parallel init to be faster, took %v", elapsed)
	}
}

func TestWithParallelInit_RespectsDepOrder(t *testing.T) {
	var mu sync.Mutex
	order := make([]string, 0)

	makeModule := func(name string, deps []string) *simpleOrderModule {
		return &simpleOrderModule{name: name, deps: deps, order: &order, mu: &mu}
	}

	modDep := makeModule("dep", nil)
	modA := makeModule("a", []string{"dep"})
	modB := makeModule("b", []string{"dep"})

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithModules(modDep, modA, modB),
		WithParallelInit(),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 || order[0] != "dep" {
		t.Errorf("expected dep first, got order %v", order)
	}
}

type simpleOrderModule struct {
	name  string
	deps  []string
	order *[]string
	mu    *sync.Mutex
}

func (m *simpleOrderModule) Name() string           { return m.name }
func (m *simpleOrderModule) Dependencies() []string { return m.deps }
func (m *simpleOrderModule) Init(app Application) error {
	m.mu.Lock()
	*m.order = append(*m.order, m.name)
	m.mu.Unlock()
	return nil
}
