// Package dev contains development and tooling utilities for the modular framework
package dev

// TasksContext records the feature identifier and version for development tooling
type TasksContext struct {
	// FeatureID identifies the specific feature being implemented
	FeatureID string

	// Version tracks the specification version
	Version string

	// Directory points to the feature specification directory
	Directory string
}

// GetCurrentTasksContext returns the context for the baseline specification implementation
func GetCurrentTasksContext() TasksContext {
	return TasksContext{
		FeatureID: "001-baseline-specification-for",
		Version:   "1.0.0",
		Directory: "specs/001-baseline-specification-for",
	}
}
