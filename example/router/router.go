package router

import (
	"github.com/GoCodeAlone/modular"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	//"github.com/go-chi/chi/v5/middleware"
)

const configSection = "router"

type Module struct {
	router chi.Router
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
	m.router = chi.NewRouter()
	m.router.Use(middleware.RequestID)
	m.router.Use(middleware.RealIP)
	m.router.Use(middleware.Logger)

	cp, err := app.GetConfigSection(configSection)
	if err != nil {
		return err
	}

	cfg := cp.GetConfig().(*Config)

	app.Logger().Info("RouterModule initialized", "cfg", *cfg)

	return nil
}

func (m *Module) Name() string {
	return "router"
}

func (m *Module) Dependencies() []string {
	return nil
}

func (m *Module) ProvidesServices() []modular.Service {
	return []modular.Service{
		{Name: "router", Description: "HTTP RouterModule", Instance: m.router},
	}
}

func (m *Module) RequiresServices() []modular.ServiceDependency {
	return nil
}
