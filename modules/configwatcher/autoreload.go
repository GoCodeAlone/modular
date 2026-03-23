package configwatcher

import (
	"context"

	"github.com/GoCodeAlone/modular"
)

// WithAutoReload returns an Option that wires the ConfigWatcher to the given
// modular.ReloadableApp so that every detected file-change triggers a
// configuration reload via app.RequestReload.
//
// The reload is fired with a ReloadFileChange trigger and an empty ConfigDiff.
// The orchestrator will still call CanReload / Reload on all registered
// Reloadable modules so they can re-read their configuration.
//
// Example:
//
//	w := configwatcher.New(
//	    configwatcher.WithPaths("config.yaml"),
//	    configwatcher.WithAutoReload(app),
//	)
func WithAutoReload(app modular.ReloadableApp) Option {
	return WithOnChange(func(_ []string) {
		_ = app.RequestReload(
			context.Background(),
			modular.ReloadFileChange,
			modular.ConfigDiff{},
		)
	})
}

// ConnectAutoReload wires an already-constructed ConfigWatcher to the given
// modular.ReloadableApp. It replaces any previously registered OnChange callback.
// Prefer WithAutoReload when constructing the watcher; use this function when
// the watcher and the application are wired together after construction.
//
// Example:
//
//	w := configwatcher.New(configwatcher.WithPaths("config.yaml"))
//	configwatcher.ConnectAutoReload(w, app)
func ConnectAutoReload(w *ConfigWatcher, app modular.ReloadableApp) {
	w.onChange = func(_ []string) {
		_ = app.RequestReload(
			context.Background(),
			modular.ReloadFileChange,
			modular.ConfigDiff{},
		)
	}
}
