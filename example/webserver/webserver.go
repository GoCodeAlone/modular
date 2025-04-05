package webserver

import (
	"context"
	"errors"
	"example/router"
	"fmt"
	"github.com/GoCodeAlone/modular"
	"net/http"
	"reflect"
	"time"
)

const configSection = "webserver"

type Module struct {
	router        http.Handler // Dependency
	server        *http.Server
	config        *WebConfig
	app           modular.Application
	routerService router.Router
}

func NewWebServer() *Module {
	return &Module{}
}

type WebConfig struct {
	Port string
}

func (m *Module) RegisterConfig(app modular.Application) error {
	app.RegisterConfigSection(configSection, modular.NewStdConfigProvider(&WebConfig{
		Port: "8080",
	}))
	return nil
}

func (m *Module) Init(app modular.Application) error {
	app.Logger().Info("web server initialized", "cfg", *m.config)
	// Only do startup operations here, not construction

	m.registerRoutes()

	return nil
}

func (m *Module) Start(ctx context.Context) error {
	go func() {
		m.app.Logger().Info("web server starting", "port", m.config.Port)
		if err := m.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			m.app.Logger().Error("web server error", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		m.app.Logger().Info("web server stopping")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := m.server.Shutdown(shutdownCtx)
		if err != nil {
			m.app.Logger().Error("web server shutdown error", "error", err)
			return
		}
	}()

	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	if m.server != nil {
		return m.server.Shutdown(ctx)
	}
	return nil
}

func (m *Module) Name() string {
	return "webserver"
}

func (m *Module) Dependencies() []string {
	return []string{"router"}
}

func (m *Module) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{Name: "webserver", Description: "HTTP server", Instance: m.server},
	}
}

func (m *Module) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "router",
			Required:           true,
			SatisfiesInterface: reflect.TypeOf((*http.Handler)(nil)).Elem(),
		},
		{
			Name:               "routerService",
			Required:           true,
			SatisfiesInterface: reflect.TypeOf((*router.Router)(nil)).Elem(),
		},
	}
}

func (m *Module) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		// Get router dependency
		rtr, ok := services["router"].(http.Handler)
		if !ok {
			return nil, fmt.Errorf("service 'router' is not of type http.Handler or is nil. Detected type: %T", services["router"])
		}
		rtrSvc, ok := services["routerService"].(router.Router)
		if !ok {
			return nil, fmt.Errorf("service 'routerService' is not of type router.Router or is nil. Detected type: %T", services["routerService"])
		}

		// Get config early
		cp, err := app.GetConfigSection(configSection)
		if err != nil {
			return nil, err
		}
		config := cp.GetConfig().(*WebConfig)

		// Create a complete new module with all dependencies
		return &Module{
			app:           app,
			router:        rtr,
			routerService: rtrSvc,
			config:        config,
			server: &http.Server{
				Addr:    ":" + config.Port,
				Handler: rtr,
			},
		}, nil
	}
}

// registerRoutes registers default routes
func (m *Module) registerRoutes() {
	m.routerService.Get("/health", m.handleHealth)
}

func (m *Module) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
