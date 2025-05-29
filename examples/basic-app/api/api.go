package api

import (
	"basic-app/router"
	"net/http"
	"reflect"

	"github.com/GoCodeAlone/modular"
	"github.com/go-chi/chi/v5"
)

type Module struct {
	app           modular.Application
	routerService router.Router
}

func NewAPIModule() *Module {
	return &Module{}
}

func (m *Module) Init(app modular.Application) error {
	m.registerRoutes()
	return nil
}

func (m *Module) Name() string {
	return "api"
}

func (m *Module) Dependencies() []string {
	return []string{"router"}
}

func (m *Module) ProvidesServices() []modular.ServiceProvider {
	return nil
}

func (m *Module) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{
			Name:               "routerService",
			Required:           true,
			SatisfiesInterface: reflect.TypeOf((*router.Router)(nil)).Elem(),
		},
	}
}

// Constructor implements ModuleConstructor interface for dependency injection
func (m *Module) Constructor() modular.ModuleConstructor {
	return func(app modular.Application, services map[string]any) (modular.Module, error) {
		m.app = app
		m.routerService = services["routerService"].(router.Router)
		return m, nil
	}
}

// registerRoutes registers all API routes
func (m *Module) registerRoutes() {
	// Register a base API route
	m.routerService.Route("/api/v1", func(r chi.Router) {
		// Example resources
		r.Route("/users", func(r chi.Router) {
			r.Get("/", m.handleGetUsers)
			r.Post("/", m.handleCreateUser)
			r.Get("/{id}", m.handleGetUser)
		})
	})
}

// Route handlers
func (m *Module) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"users":[]}`))
}

func (m *Module) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"id":"new-user-id"}`))
}

func (m *Module) handleGetUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"id":"user-id","name":"Example User"}`))
}
