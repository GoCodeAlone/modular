package modular

import (
	"reflect"
	"testing"
)

func TestNewApplication(t *testing.T) {
	type args struct {
		cfgProvider ConfigProvider
		logger      Logger
	}
	cp := NewStdConfigProvider(testCfg{Str: "test"})
	logger := &logger{}
	tests := []struct {
		name string
		args args
		want AppRegistry
	}{
		{
			name: "TestNewApplication",
			args: args{
				cfgProvider: nil,
				logger:      nil,
			},
			want: &Application{
				cfgProvider:    nil,
				cfgSections:    make(map[string]ConfigProvider),
				svcRegistry:    make(ServiceRegistry),
				moduleRegistry: make(ModuleRegistry),
				logger:         nil,
			},
		},
		{
			name: "TestNewApplicationWithConfigProviderAndLogger",
			args: args{
				cfgProvider: cp,
				logger:      logger,
			},
			want: &Application{
				cfgProvider:    cp,
				cfgSections:    make(map[string]ConfigProvider),
				svcRegistry:    make(ServiceRegistry),
				moduleRegistry: make(ModuleRegistry),
				logger:         logger,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewApplication(tt.args.cfgProvider, tt.args.logger); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewApplication() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_application_Init(t *testing.T) {
	type fields struct {
		cfgProvider    ConfigProvider
		svcRegistry    ServiceRegistry
		moduleRegistry ModuleRegistry
		logger         Logger
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &Application{
				cfgProvider:    tt.fields.cfgProvider,
				svcRegistry:    tt.fields.svcRegistry,
				moduleRegistry: tt.fields.moduleRegistry,
				logger:         tt.fields.logger,
			}
			if err := app.Init(); (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_application_Logger(t *testing.T) {
	type fields struct {
		cfgProvider    ConfigProvider
		svcRegistry    ServiceRegistry
		moduleRegistry ModuleRegistry
		logger         Logger
	}
	tests := []struct {
		name   string
		fields fields
		want   Logger
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &Application{
				cfgProvider:    tt.fields.cfgProvider,
				svcRegistry:    tt.fields.svcRegistry,
				moduleRegistry: tt.fields.moduleRegistry,
				logger:         tt.fields.logger,
			}
			if got := app.Logger(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Logger() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_application_RegisterModule(t *testing.T) {
	type fields struct {
		cfgProvider    ConfigProvider
		svcRegistry    ServiceRegistry
		moduleRegistry ModuleRegistry
		logger         Logger
	}
	type args struct {
		module Module
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &Application{
				cfgProvider:    tt.fields.cfgProvider,
				svcRegistry:    tt.fields.svcRegistry,
				moduleRegistry: tt.fields.moduleRegistry,
				logger:         tt.fields.logger,
			}
			app.RegisterModule(tt.args.module)
		})
	}
}

func Test_application_SvcRegistry(t *testing.T) {
	type fields struct {
		cfgProvider    ConfigProvider
		svcRegistry    ServiceRegistry
		moduleRegistry ModuleRegistry
		logger         Logger
	}
	tests := []struct {
		name   string
		fields fields
		want   ServiceRegistry
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &Application{
				cfgProvider:    tt.fields.cfgProvider,
				svcRegistry:    tt.fields.svcRegistry,
				moduleRegistry: tt.fields.moduleRegistry,
				logger:         tt.fields.logger,
			}
			if got := app.SvcRegistry(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SvcRegistry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_application_resolveDependencies(t *testing.T) {
	type fields struct {
		cfgProvider    ConfigProvider
		svcRegistry    ServiceRegistry
		moduleRegistry ModuleRegistry
		logger         Logger
	}
	tests := []struct {
		name    string
		fields  fields
		want    []string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &Application{
				cfgProvider:    tt.fields.cfgProvider,
				svcRegistry:    tt.fields.svcRegistry,
				moduleRegistry: tt.fields.moduleRegistry,
				logger:         tt.fields.logger,
			}
			got, err := app.resolveDependencies()
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveDependencies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("resolveDependencies() got = %v, want %v", got, tt.want)
			}
		})
	}
}
