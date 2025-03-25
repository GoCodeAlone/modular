package modular

// ServiceRegistry allows registration and retrieval of services
type ServiceRegistry map[string]any

// RegisterService adds a service to the svcRegistry
func RegisterService[T any](app AppRegistry, name string, service *T) {
	app.SvcRegistry()[name] = service
}

// GetService retrieves a service by name
func GetService[T any](app AppRegistry, name string) (*T, bool) {
	registry := app.SvcRegistry()
	if registry == nil {
		return nil, false
	}

	svc, exists := registry[name].(*T)
	if !exists {
		return nil, exists
	}
	return svc, exists
}
