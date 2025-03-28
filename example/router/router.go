package router

import (
	"context"
	"fmt"
	"github.com/GoCodeAlone/modular"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"net/http"
)

const configSection = "router"

type Router = chi.Router

type Module struct {
	app    *modular.Application
	config *Config
	router Router
}

func NewRouter() *Module {
	return &Module{
		router: chi.NewRouter(),
	}
}

type Config struct {
	RoutePrefix string
}

func (m *Module) RegisterConfig(app *modular.Application) {
	app.RegisterConfigSection(configSection, modular.NewStdConfigProvider(&Config{
		RoutePrefix: "/",
	}))
}

func (m *Module) Init(app *modular.Application) error {
	m.router.Use(middleware.RequestID)
	m.router.Use(middleware.RealIP)
	m.router.Use(middleware.Logger)

	cp, err := app.GetConfigSection(configSection)
	if err != nil {
		return err
	}

	m.app = app
	m.config = cp.GetConfig().(*Config)

	app.Logger().Info("RouterModule initialized", "cfg", *m.config)
	app.Logger().Info("Router initialized", "router", m.router)

	return nil
}

func (m *Module) Start(ctx context.Context) error {
	// print routes
	err := chi.Walk(m.router, func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		m.app.Logger().Info("Route", "method", method, "route", route)
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	return nil
}

func (m *Module) Name() string {
	return "router"
}

func (m *Module) Dependencies() []string {
	return nil
}

func (m *Module) ProvidesServices() []modular.ServiceProvider {
	fmt.Printf("router: %v\n", m.router)
	return []modular.ServiceProvider{
		{Name: "router", Description: "HTTP RouterModule", Instance: m.router},
		{Name: "routerService", Description: "Router Service Interface", Instance: m.router},
	}
}

func (m *Module) RequiresServices() []modular.ServiceDependency {
	return nil
}
