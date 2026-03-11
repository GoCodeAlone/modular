package configwatcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/GoCodeAlone/modular"
	"github.com/fsnotify/fsnotify"
)

// ConfigWatcher watches config files and calls OnChange when modifications are detected.
type ConfigWatcher struct {
	paths    []string
	debounce time.Duration
	onChange func(paths []string)
	watcher  *fsnotify.Watcher
	stopCh   chan struct{}
	stopOnce sync.Once
	logger   modular.Logger
}

// Option configures a ConfigWatcher.
type Option func(*ConfigWatcher)

func WithPaths(paths ...string) Option {
	return func(w *ConfigWatcher) { w.paths = append(w.paths, paths...) }
}

func WithDebounce(d time.Duration) Option {
	return func(w *ConfigWatcher) { w.debounce = d }
}

func WithOnChange(fn func(paths []string)) Option {
	return func(w *ConfigWatcher) { w.onChange = fn }
}

func WithLogger(l modular.Logger) Option {
	return func(w *ConfigWatcher) { w.logger = l }
}

func New(opts ...Option) *ConfigWatcher {
	w := &ConfigWatcher{
		debounce: 500 * time.Millisecond,
		stopCh:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *ConfigWatcher) Name() string { return "configwatcher" }

// Init satisfies the modular.Module interface. Captures the application logger
// if one was not provided via WithLogger.
func (w *ConfigWatcher) Init(app modular.Application) error {
	if w.logger == nil {
		w.logger = app.Logger()
	}
	return nil
}

func (w *ConfigWatcher) Start(ctx context.Context) error {
	if err := w.startWatching(); err != nil {
		return err
	}
	go func() {
		select {
		case <-ctx.Done():
			_ = w.stopWatching()
		case <-w.stopCh:
		}
	}()
	return nil
}

func (w *ConfigWatcher) Stop(_ context.Context) error {
	return w.stopWatching()
}

func (w *ConfigWatcher) startWatching() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating file watcher: %w", err)
	}
	w.watcher = watcher
	for _, path := range w.paths {
		if err := watcher.Add(path); err != nil {
			watcher.Close()
			return fmt.Errorf("watching path %q: %w", path, err)
		}
	}
	go w.eventLoop()
	return nil
}

func (w *ConfigWatcher) stopWatching() error {
	var closeErr error
	w.stopOnce.Do(func() {
		close(w.stopCh)
		if w.watcher != nil {
			if err := w.watcher.Close(); err != nil {
				closeErr = fmt.Errorf("closing file watcher: %w", err)
			}
		}
	})
	return closeErr
}

func (w *ConfigWatcher) eventLoop() {
	var timer *time.Timer
	changedPaths := make(map[string]struct{})
	var mu sync.Mutex

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				mu.Lock()
				changedPaths[event.Name] = struct{}{}
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(w.debounce, func() {
					// Check stopCh before invoking callback to avoid
					// firing onChange after shutdown.
					select {
					case <-w.stopCh:
						return
					default:
					}
					if w.onChange != nil {
						mu.Lock()
						paths := make([]string, 0, len(changedPaths))
						for p := range changedPaths {
							paths = append(paths, p)
						}
						clear(changedPaths)
						mu.Unlock()
						w.onChange(paths)
					}
				})
				mu.Unlock()
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			if w.logger != nil {
				w.logger.Error("file watcher error", "error", err)
			}
		case <-w.stopCh:
			if timer != nil {
				timer.Stop()
			}
			return
		}
	}
}
