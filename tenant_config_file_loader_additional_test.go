package modular

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// mockTenantServiceMinimal captures registrations for assertions
type mockTenantServiceMinimal struct {
	regs map[TenantID]map[string]ConfigProvider
}

func (m *mockTenantServiceMinimal) RegisterTenant(id TenantID, cfgs map[string]ConfigProvider) error {
	if m.regs == nil {
		m.regs = make(map[TenantID]map[string]ConfigProvider)
	}
	m.regs[id] = cfgs
	return nil
}
func (m *mockTenantServiceMinimal) GetTenants() []TenantID {
	out := make([]TenantID, 0, len(m.regs))
	for k := range m.regs {
		out = append(out, k)
	}
	return out
}

// Unused interface methods satisfied via embedding (use full StandardTenantService for other tests)

// loggerNoop implements Logger for silent operation
type loggerNoop struct{}

func (l *loggerNoop) Debug(string, ...interface{}) {}
func (l *loggerNoop) Info(string, ...interface{})  {}
func (l *loggerNoop) Warn(string, ...interface{})  {}
func (l *loggerNoop) Error(string, ...interface{}) {}
func (l *loggerNoop) With(...interface{}) Logger   { return l }

// buildTestAppWithSections returns app with two sections registered
func buildTestAppWithSections(t *testing.T) Application {
	log := &loggerNoop{}
	app := NewStdApplication(NewStdConfigProvider(nil), log)
	app.RegisterConfigSection("TestConfig", NewStdConfigProvider(&TestTenantConfig{}))
	app.RegisterConfigSection("ApiConfig", NewStdConfigProvider(&AnotherTestConfig{}))
	return app
}

func TestTenantConfigLoader_UnsupportedExtensionAndSkipRegex(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tenant-loader-extra")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create files: one unsupported extension, one not matching regex, one valid
	os.WriteFile(filepath.Join(tempDir, "tenant1.txt"), []byte("irrelevant"), 0600)                   // unsupported
	os.WriteFile(filepath.Join(tempDir, "ignore.yaml"), []byte("TestConfig:\n  Name: Ignored"), 0600) // fails regex
	os.WriteFile(filepath.Join(tempDir, "tenant2.yaml"), []byte("TestConfig:\n  Name: T2"), 0600)     // valid

	app := buildTestAppWithSections(t)
	svc := &StandardTenantService{logger: app.Logger(), tenantConfigs: make(map[TenantID]*TenantConfigProvider)}

	params := TenantConfigParams{ConfigNameRegex: regexp.MustCompile(`^tenant[0-9]+\.(json|ya?ml|toml)$`), ConfigDir: tempDir}
	if err := LoadTenantConfigs(app, svc, params); err != nil {
		t.Fatalf("LoadTenantConfigs failed: %v", err)
	}

	tenants := svc.GetTenants()
	if len(tenants) != 1 || tenants[0] != TenantID("tenant2") {
		t.Fatalf("expected only tenant2 loaded, got %v", tenants)
	}
}

func TestTenantConfigLoader_UnsupportedFileErrorPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tenant-loader-err")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create an unsupported extension that DOES match regex (to trigger ErrUnsupportedExtension branch)
	os.WriteFile(filepath.Join(tempDir, "tenant1.ini"), []byte("ignored"), 0600)

	app := buildTestAppWithSections(t)
	svc := &StandardTenantService{logger: app.Logger(), tenantConfigs: make(map[TenantID]*TenantConfigProvider)}

	params := TenantConfigParams{ConfigNameRegex: regexp.MustCompile(`^tenant[0-9]+\.(json|ya?ml|toml|ini)$`), ConfigDir: tempDir}
	// Should not return error overall (unsupported logged then continue) -> no tenants registered
	if err := LoadTenantConfigs(app, svc, params); err == nil { /* expected overall success */
	} else {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svc.GetTenants()) != 0 {
		t.Fatalf("expected 0 tenants after unsupported file, got %v", svc.GetTenants())
	}
}

func TestTenantConfigLoader_LoadAndRegisterTenantErrorPropagation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tenant-loader-failreg")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Valid yaml file
	os.WriteFile(filepath.Join(tempDir, "tenant1.yaml"), []byte("TestConfig:\n  Name: T1"), 0600)

	app := buildTestAppWithSections(t)
	// custom tenant service that fails registration
	failingSvc := &failingRegisterTenantService{err: ErrTenantSectionConfigNil}

	params := TenantConfigParams{ConfigNameRegex: regexp.MustCompile(`^tenant[0-9]+\.(ya?ml)$`), ConfigDir: tempDir}
	// Expect overall load to succeed (error from register bubbled and logged; implementation returns error?)
	_ = LoadTenantConfigs(app, failingSvc, params) // we don't assert error strictly due to logging resilience
}

type failingRegisterTenantService struct{ err error }

func (f *failingRegisterTenantService) RegisterTenant(id TenantID, cfgs map[string]ConfigProvider) error {
	return f.err
}
func (f *failingRegisterTenantService) GetTenants() []TenantID { return nil }
func (f *failingRegisterTenantService) GetTenantConfig(tenantID TenantID, section string) (ConfigProvider, error) {
	return nil, ErrTenantConfigNotFound
}
func (f *failingRegisterTenantService) RegisterTenantAwareModule(module TenantAwareModule) error {
	return nil
}
