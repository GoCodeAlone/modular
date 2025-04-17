package jsonschema

import (
	"github.com/GoCodeAlone/modular"
)

const Name = "modular.jsonschema"

type Module struct {
	schemaService JSONSchemaService
}

func NewModule() *Module {
	return &Module{
		schemaService: NewJSONSchemaService(),
	}
}

func (m *Module) Name() string {
	return Name
}

func (m *Module) Init(app modular.Application) error {
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
