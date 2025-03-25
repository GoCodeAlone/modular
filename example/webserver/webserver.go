package webserver

import (
	"fmt"
	"github.com/GoCodeAlone/modular"
	"net/http"
	"reflect"
)

const configSection = "webserver"

type Router = http.Handler

type Module struct {
	router Router // Dependency
	server *http.Server
	config *WebConfig
}

func NewWebServer() *Module {
	return &Module{}
}

type WebConfig struct {
	Port string
}

func (m *Module) RegisterConfig(app *modular.Application) {
	app.RegisterConfigSection(configSection, modular.NewStdConfigProvider(&WebConfig{
		Port: "8080",
	}))
}

func (m *Module) Init(app *modular.Application) error {
	app.Logger().Info("web server initialized", "cfg", *m.config)
	// Only do startup operations here, not construction

	return nil
}

func (m *Module) Name() string {
	return "webserver"
}

func (m *Module) Dependencies() []string {
	return []string{"router"}
}

func (m *Module) ProvidesServices() []modular.Service {
	return []modular.Service{
		{Name: "webserver", Description: "HTTP server", Instance: m.server},
	}
}

func (m *Module) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{Name: "router", Required: true, SatisfiesInterface: reflect.TypeOf((*Router)(nil)).Elem()},
		//{Name: "logger", Required: true, Type: reflect.TypeOf((*Logger)(nil)).Elem()},
		//{Name: "auth", Required: false, Type: reflect.TypeOf(&AuthService{})},
	}
}

func (m *Module) Constructor() modular.ModuleConstructor {
	return func(app *modular.Application, services map[string]any) (modular.Module, error) {
		// Get router dependency
		router, ok := services["router"].(Router)
		if !ok {
			return nil, fmt.Errorf("service 'router' is not of type Router")
		}

		// Get config early
		cp, err := app.GetConfigSection(configSection)
		if err != nil {
			return nil, err
		}
		config := cp.GetConfig().(*WebConfig)

		// Create a complete new module with all dependencies
		return &Module{
			router: router,
			config: config,
			server: &http.Server{
				Addr:    ":" + config.Port,
				Handler: router,
			},
		}, nil
	}
}
