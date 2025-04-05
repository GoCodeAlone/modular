package jsonschema

import (
	"github.com/GoCodeAlone/modular"
)

type Module struct {
	schemaService JSONSchemaService
}

func NewModule() *Module {
	return &Module{
		schemaService: NewJSONSchemaService(),
	}
}

func (m *Module) Name() string {
	return "jsonschema"
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
