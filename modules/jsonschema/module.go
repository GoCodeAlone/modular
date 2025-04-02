package jsonschema

import (
	"context"
	"github.com/GoCodeAlone/modular"
)

type Module struct {
	app           modular.Application
	schemaService JSONSchemaService
}

func NewModule(application modular.Application) *Module {
	return &Module{
		app:           application,
		schemaService: NewJSONSchemaService(),
	}
}

func (m *Module) RegisterConfig(app modular.Application) {
	// nothing to do
}

func (m *Module) Init(app modular.Application) error {
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	// Nothing special needed for startup
	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	// Nothing special needed for shutdown
	return nil
}

func (m *Module) Name() string {
	return "jsonschema"
}

func (m *Module) Dependencies() []string {
	return nil
}

func (m *Module) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{
		{
			Name:     "jsonschema.service",
			Instance: m.schemaService,
		},
	}
}

func (m *Module) RequiresServices() []modular.ServiceDependency {
	return nil
}
