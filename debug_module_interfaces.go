package modular

import (
	"fmt"
	"reflect"
)

// DebugModuleInterfaces helps debug interface implementation issues during module lifecycle
func DebugModuleInterfaces(app Application, moduleName string) {
	// Ensure the application is of type StdApplication
	stdApp, ok := app.(*StdApplication)
	if !ok {
		fmt.Println("❌ Application is not a StdApplication, cannot debug modules")
		return
	}
	module, exists := stdApp.moduleRegistry[moduleName]
	if !exists {
		fmt.Printf("❌ Module '%s' not found in registry\n", moduleName)
		return
	}

	fmt.Printf("🔍 Debugging module '%s' (type: %T)\n", moduleName, module)
	fmt.Printf("   Memory address: %p\n", module)

	// Check all the interfaces
	interfaces := map[string]interface{}{
		"Module":          (*Module)(nil),
		"Configurable":    (*Configurable)(nil),
		"DependencyAware": (*DependencyAware)(nil),
		"ServiceAware":    (*ServiceAware)(nil),
		"Startable":       (*Startable)(nil),
		"Stoppable":       (*Stoppable)(nil),
		"Constructable":   (*Constructable)(nil),
	}

	moduleType := reflect.TypeOf(module)
	for interfaceName, interfacePtr := range interfaces {
		interfaceType := reflect.TypeOf(interfacePtr).Elem()
		implements := moduleType.Implements(interfaceType)
		status := "❌"
		if implements {
			status = "✅"
		}
		fmt.Printf("   %s %s\n", status, interfaceName)
	}

	// Check if it's a ServiceAware module
	if svcAware, ok := module.(ServiceAware); ok {
		provides := svcAware.ProvidesServices()
		requires := svcAware.RequiresServices()
		fmt.Printf("   📦 Provides %d services, Requires %d services\n", len(provides), len(requires))

		if constructable, isConstructable := module.(Constructable); isConstructable {
			fmt.Printf("   🏗️  Has constructor - this module may be replaced during injection!\n")
			_ = constructable // avoid unused variable warning
		}
	}
}

// DebugAllModuleInterfaces debugs all registered modules
func DebugAllModuleInterfaces(app Application) {
	// Ensure the application is of type StdApplication
	stdApp, ok := app.(*StdApplication)
	if !ok {
		fmt.Println("❌ Application is not a StdApplication, cannot debug modules")
		return
	}

	fmt.Printf("\n🔍 ==> DEBUG: All Module Interface Implementations <==\n")
	for name := range stdApp.moduleRegistry {
		DebugModuleInterfaces(app, name)
		fmt.Println()
	}
}

// CompareModuleInstances compares two module instances to see if they're the same
func CompareModuleInstances(original, current Module, moduleName string) {
	fmt.Printf("🔍 Comparing module instances for '%s':\n", moduleName)
	fmt.Printf("   Original: %T at %p\n", original, original)
	fmt.Printf("   Current:  %T at %p\n", current, current)

	if original == current {
		fmt.Printf("   ✅ Same instance\n")
	} else {
		fmt.Printf("   ❌ Different instances - module was replaced!\n")
	}

	// Check if both implement Startable
	_, originalStartable := original.(Startable)
	_, currentStartable := current.(Startable)

	fmt.Printf("   Original implements Startable: %v\n", originalStartable)
	fmt.Printf("   Current implements Startable:  %v\n", currentStartable)

	if originalStartable && !currentStartable {
		fmt.Printf("   🚨 PROBLEM: Original was Startable but replacement is not!\n")
	}
}
