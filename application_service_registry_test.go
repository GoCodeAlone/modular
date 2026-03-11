package modular

import (
	"errors"
	"testing"
)

// Test_RegisterService tests service registration scenarios
func Test_RegisterService(t *testing.T) {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &logger{t},
	}

	// Test successful registration
	err := app.RegisterService("storage", &MockStorage{data: map[string]string{"key": "value"}})
	if err != nil {
		t.Errorf("RegisterService() error = %v, expected no error", err)
	}

	// Test duplicate registration
	err = app.RegisterService("storage", &MockStorage{data: map[string]string{}})
	if err == nil {
		t.Error("RegisterService() expected error for duplicate service, got nil")
	} else if !IsServiceAlreadyRegisteredError(err) {
		t.Errorf("RegisterService() expected ErrServiceAlreadyRegistered, got %v", err)
	}
}

// Test_GetService tests service retrieval scenarios
func Test_GetService(t *testing.T) {
	app := &StdApplication{
		cfgProvider:    NewStdConfigProvider(testCfg{Str: "test"}),
		cfgSections:    make(map[string]ConfigProvider),
		svcRegistry:    make(ServiceRegistry),
		moduleRegistry: make(ModuleRegistry),
		logger:         &logger{t},
	}

	// Register test services
	mockStorage := &MockStorage{data: map[string]string{"key": "value"}}
	if err := app.RegisterService("storage", mockStorage); err != nil {
		t.Fatalf("Failed to register storage service: %v", err)
	}

	// Test retrieving existing service
	tests := []struct {
		name        string
		serviceName string
		target      any
		wantErr     bool
		errCheck    func(error) bool
	}{
		{
			name:        "Get existing service with interface target",
			serviceName: "storage",
			target:      new(StorageService),
			wantErr:     false,
		},
		{
			name:        "Get existing service with concrete type target",
			serviceName: "storage",
			target:      new(MockStorage),
			wantErr:     false,
		},
		{
			name:        "Get non-existent service",
			serviceName: "unknown",
			target:      new(StorageService),
			wantErr:     true,
			errCheck:    IsServiceNotFoundError,
		},
		{
			name:        "Target not a pointer",
			serviceName: "storage",
			target:      StorageService(nil),
			wantErr:     true,
			errCheck:    func(err error) bool { return errors.Is(err, ErrTargetNotPointer) },
		},
		{
			name:        "Incompatible target type",
			serviceName: "storage",
			target:      new(string),
			wantErr:     true,
			errCheck:    IsServiceIncompatibleError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := app.GetService(tt.serviceName, tt.target)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errCheck != nil && !tt.errCheck(err) {
				t.Errorf("GetService() expected specific error, got %v", err)
			}

			if !tt.wantErr {
				if ptr, ok := tt.target.(*StorageService); ok && *ptr == nil {
					t.Error("GetService() service was nil after successful retrieval")
				}
			}
		})
	}
}
