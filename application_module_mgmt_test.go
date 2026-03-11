package modular

import (
	"testing"
)

// Test_ResolveDependencies tests module dependency resolution
func Test_ResolveDependencies(t *testing.T) {
	tests := []struct {
		name       string
		modules    []Module
		wantErr    bool
		errCheck   func(error) bool
		checkOrder func([]string) bool
	}{
		{
			name: "Simple dependency chain",
			modules: []Module{
				&testModule{name: "module-c", dependencies: []string{"module-b"}},
				&testModule{name: "module-b", dependencies: []string{"module-a"}},
				&testModule{name: "module-a", dependencies: []string{}},
			},
			wantErr: false,
			checkOrder: func(order []string) bool {
				// Ensure module-a comes before module-b and module-b before module-c
				aIdx := -1
				bIdx := -1
				cIdx := -1
				for i, name := range order {
					switch name {
					case "module-a":
						aIdx = i
					case "module-b":
						bIdx = i
					case "module-c":
						cIdx = i
					}
				}
				return aIdx < bIdx && bIdx < cIdx
			},
		},
		{
			name: "Circular dependency",
			modules: []Module{
				&testModule{name: "module-a", dependencies: []string{"module-b"}},
				&testModule{name: "module-b", dependencies: []string{"module-a"}},
			},
			wantErr:  true,
			errCheck: IsCircularDependencyError,
		},
		{
			name: "Missing dependency",
			modules: []Module{
				&testModule{name: "module-a", dependencies: []string{"non-existent"}},
			},
			wantErr:  true,
			errCheck: IsModuleDependencyMissingError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &StdApplication{
				cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
				cfgSections:    make(map[string]ConfigProvider),
				svcRegistry:    make(ServiceRegistry),
				moduleRegistry: make(ModuleRegistry),
				logger:         &logger{t},
			}

			// Register modules
			for _, module := range tt.modules {
				app.RegisterModule(module)
			}

			// Resolve dependencies
			order, _, err := app.resolveDependencies()

			if (err != nil) != tt.wantErr {
				t.Errorf("resolveDependencies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errCheck != nil && !tt.errCheck(err) {
				t.Errorf("resolveDependencies() expected specific error, got %v", err)
			}

			if !tt.wantErr && tt.checkOrder != nil && !tt.checkOrder(order) {
				t.Errorf("resolveDependencies() returned incorrect order: %v", order)
			}
		})
	}
}
