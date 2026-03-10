package modular

import (
	"reflect"
)

// Mock modules for different cycle scenarios

// CycleModuleA - provides TestInterfaceA and requires TestInterfaceB
type CycleModuleA struct {
	name string
}

func (m *CycleModuleA) Name() string               { return m.name }
func (m *CycleModuleA) Init(app Application) error { return nil }

func (m *CycleModuleA) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "serviceA",
		Instance: &TestInterfaceAImpl{name: "A"},
	}}
}

func (m *CycleModuleA) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "serviceB",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeFor[TestInterfaceB](),
	}}
}

// CycleModuleB - provides TestInterfaceB and requires TestInterfaceA
type CycleModuleB struct {
	name string
}

func (m *CycleModuleB) Name() string               { return m.name }
func (m *CycleModuleB) Init(app Application) error { return nil }

func (m *CycleModuleB) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "serviceB",
		Instance: &TestInterfaceBImpl{name: "B"},
	}}
}

func (m *CycleModuleB) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "serviceA",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeFor[TestInterfaceA](),
	}}
}

// LinearModuleA - only provides services, no dependencies
type LinearModuleA struct {
	name string
}

func (m *LinearModuleA) Name() string               { return m.name }
func (m *LinearModuleA) Init(app Application) error { return nil }

func (m *LinearModuleA) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "linearServiceA",
		Instance: &TestInterfaceAImpl{name: "LinearA"},
	}}
}

// LinearModuleB - depends on LinearModuleA
type LinearModuleB struct {
	name string
}

func (m *LinearModuleB) Name() string               { return m.name }
func (m *LinearModuleB) Init(app Application) error { return nil }

func (m *LinearModuleB) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "linearServiceA",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeFor[TestInterfaceA](),
	}}
}

// SelfDependentModule - depends on a service it provides
type SelfDependentModule struct {
	name string
}

func (m *SelfDependentModule) Name() string               { return m.name }
func (m *SelfDependentModule) Init(app Application) error { return nil }

func (m *SelfDependentModule) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "selfService",
		Instance: &TestInterfaceAImpl{name: "self"},
	}}
}

func (m *SelfDependentModule) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "selfService",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeFor[TestInterfaceA](),
	}}
}

// MixedDependencyModuleA - has both named and interface dependencies
type MixedDependencyModuleA struct {
	name string
}

func (m *MixedDependencyModuleA) Name() string               { return m.name }
func (m *MixedDependencyModuleA) Init(app Application) error { return nil }

func (m *MixedDependencyModuleA) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "mixedServiceA",
		Instance: &TestInterfaceAImpl{name: "MixedA"},
	}}
}

func (m *MixedDependencyModuleA) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:             "namedServiceB", // Named dependency
		Required:         true,
		MatchByInterface: false,
	}}
}

// MixedDependencyModuleB - provides named service and requires interface
type MixedDependencyModuleB struct {
	name string
}

func (m *MixedDependencyModuleB) Name() string               { return m.name }
func (m *MixedDependencyModuleB) Init(app Application) error { return nil }

func (m *MixedDependencyModuleB) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "namedServiceB",
		Instance: &TestInterfaceBImpl{name: "MixedB"},
	}}
}

func (m *MixedDependencyModuleB) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "mixedServiceA",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeFor[TestInterfaceA](),
	}}
}

// ComplexCycleModuleA - part of 3-module cycle A->B->C->A
type ComplexCycleModuleA struct {
	name string
}

func (m *ComplexCycleModuleA) Name() string               { return m.name }
func (m *ComplexCycleModuleA) Init(app Application) error { return nil }

func (m *ComplexCycleModuleA) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "complexServiceA",
		Instance: &TestInterfaceAImpl{name: "ComplexA"},
	}}
}

func (m *ComplexCycleModuleA) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "complexServiceB",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeFor[TestInterfaceB](),
	}}
}

// ComplexCycleModuleB - part of 3-module cycle A->B->C->A
type ComplexCycleModuleB struct {
	name string
}

func (m *ComplexCycleModuleB) Name() string               { return m.name }
func (m *ComplexCycleModuleB) Init(app Application) error { return nil }

func (m *ComplexCycleModuleB) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "complexServiceB",
		Instance: &TestInterfaceBImpl{name: "ComplexB"},
	}}
}

func (m *ComplexCycleModuleB) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "complexServiceC",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeFor[TestInterfaceC](),
	}}
}

// ComplexCycleModuleC - part of 3-module cycle A->B->C->A
type ComplexCycleModuleC struct {
	name string
}

func (m *ComplexCycleModuleC) Name() string               { return m.name }
func (m *ComplexCycleModuleC) Init(app Application) error { return nil }

func (m *ComplexCycleModuleC) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "complexServiceC",
		Instance: &TestInterfaceCImpl{name: "ComplexC"},
	}}
}

func (m *ComplexCycleModuleC) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "complexServiceA",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeFor[TestInterfaceA](),
	}}
}

// DisambiguationModuleA - for interface name disambiguation testing
type DisambiguationModuleA struct {
	name string
}

func (m *DisambiguationModuleA) Name() string               { return m.name }
func (m *DisambiguationModuleA) Init(app Application) error { return nil }

func (m *DisambiguationModuleA) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "disambiguationServiceA",
		Instance: &EnhancedTestInterfaceImpl{name: "DisambigA"},
	}}
}

func (m *DisambiguationModuleA) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "disambiguationServiceB",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeFor[AnotherEnhancedTestInterface](),
	}}
}

// DisambiguationModuleB - for interface name disambiguation testing
type DisambiguationModuleB struct {
	name string
}

func (m *DisambiguationModuleB) Name() string               { return m.name }
func (m *DisambiguationModuleB) Init(app Application) error { return nil }

func (m *DisambiguationModuleB) ProvidesServices() []ServiceProvider {
	return []ServiceProvider{{
		Name:     "disambiguationServiceB",
		Instance: &AnotherEnhancedTestInterfaceImpl{name: "DisambigB"},
	}}
}

func (m *DisambiguationModuleB) RequiresServices() []ServiceDependency {
	return []ServiceDependency{{
		Name:               "disambiguationServiceA",
		Required:           true,
		MatchByInterface:   true,
		SatisfiesInterface: reflect.TypeFor[EnhancedTestInterface](), // Note: different interface
	}}
}
