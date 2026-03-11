package modular

import "testing"

type testPlugin struct {
	modules  []Module
	services []ServiceDefinition
	hookRan  bool
}

func (p *testPlugin) Name() string                    { return "test-plugin" }
func (p *testPlugin) Modules() []Module               { return p.modules }
func (p *testPlugin) Services() []ServiceDefinition   { return p.services }
func (p *testPlugin) InitHooks() []func(Application) error {
	return []func(Application) error{
		func(app Application) error {
			p.hookRan = true
			return nil
		},
	}
}

type pluginTestModule struct {
	name        string
	initialized bool
}

func (m *pluginTestModule) Name() string               { return m.name }
func (m *pluginTestModule) Init(app Application) error { m.initialized = true; return nil }

func TestWithPlugins_RegistersModulesAndServices(t *testing.T) {
	mod := &pluginTestModule{name: "plugin-mod"}
	plugin := &testPlugin{
		modules:  []Module{mod},
		services: []ServiceDefinition{{Name: "plugin.svc", Service: "hello"}},
	}

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithPlugins(plugin),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !mod.initialized {
		t.Error("plugin module should have been initialized")
	}
	if !plugin.hookRan {
		t.Error("plugin hook should have run")
	}

	svc, err := GetTypedService[string](app, "plugin.svc")
	if err != nil {
		t.Fatalf("GetTypedService: %v", err)
	}
	if svc != "hello" {
		t.Errorf("expected hello, got %s", svc)
	}
}

type simpleTestPlugin struct {
	modules []Module
}

func (p *simpleTestPlugin) Name() string      { return "simple" }
func (p *simpleTestPlugin) Modules() []Module { return p.modules }

func TestWithPlugins_SimplePlugin(t *testing.T) {
	mod := &pluginTestModule{name: "simple-mod"}
	plugin := &simpleTestPlugin{modules: []Module{mod}}

	app, err := NewApplication(
		WithLogger(nopLogger{}),
		WithPlugins(plugin),
	)
	if err != nil {
		t.Fatalf("NewApplication: %v", err)
	}
	if err := app.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !mod.initialized {
		t.Error("plugin module should have been initialized")
	}
}
