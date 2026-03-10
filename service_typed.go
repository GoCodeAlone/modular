package modular

import "fmt"

// RegisterTypedService registers a service with compile-time type safety.
func RegisterTypedService[T any](app Application, name string, svc T) error {
	return app.RegisterService(name, svc)
}

// GetTypedService retrieves a service with compile-time type safety.
func GetTypedService[T any](app Application, name string) (T, error) {
	var zero T
	svcRegistry := app.SvcRegistry()
	raw, exists := svcRegistry[name]
	if !exists {
		return zero, fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}
	typed, ok := raw.(T)
	if !ok {
		return zero, fmt.Errorf("%w: service %q is %T, want %T", ErrServiceWrongType, name, raw, zero)
	}
	return typed, nil
}
