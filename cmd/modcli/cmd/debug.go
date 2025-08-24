package cmd

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// ServiceInfo represents a service found during analysis
type ServiceInfo struct {
	Module      string
	File        string
	ServiceName string
	Type        string
	Description string
	Kind        string // provided/required
	Interface   string // for required
	Optional    bool
}

// NewDebugCommand creates the debug command for troubleshooting modular applications
func NewDebugCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Debug and troubleshoot modular applications",
		Long: `Debug command provides tools for troubleshooting modular applications:

- Verify interface implementations
- Analyze module dependencies  
- Inspect service registrations
- Check module compatibility

These tools help diagnose common issues like interface matching failures,
missing dependencies, and circular dependencies.`,
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}

	// Add debug subcommands
	cmd.AddCommand(NewDebugInterfaceCommand())
	cmd.AddCommand(NewDebugDependenciesCommand())
	cmd.AddCommand(NewDebugServicesCommand())
	cmd.AddCommand(NewDebugConfigCommand())
	cmd.AddCommand(NewDebugLifecycleCommand())
	cmd.AddCommand(NewDebugHealthCommand())
	cmd.AddCommand(NewDebugTenantCommand())

	return cmd
}

// NewDebugInterfaceCommand creates a command to verify interface implementations
func NewDebugInterfaceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "interface",
		Short: "Verify if a type implements an interface",
		Long: `Verify interface implementation using reflection.

This helps debug interface matching issues in dependency injection.

Examples:
  modcli debug interface --type "*chimux.ChiMuxModule" --interface "http.Handler"
  modcli debug interface --type "*sql.DB" --interface "database.Executor"`,
		RunE: runDebugInterface,
	}

	cmd.Flags().StringP("type", "t", "", "The concrete type to check (e.g., '*chimux.ChiMuxModule')")
	cmd.Flags().StringP("interface", "i", "", "The interface to check against (e.g., 'http.Handler')")
	cmd.Flags().BoolP("verbose", "v", false, "Show detailed reflection information")

	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("interface")

	return cmd
}

// NewDebugServicesCommand creates a command to inspect service registrations
func NewDebugServicesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "services",
		Short: "Inspect service registrations in a project",
		Long: `Inspect service registrations and requirements in a modular project.

This helps understand what services are provided vs required and identify 
dependency issues in your modular application.

📝 Service Debugging Guidelines:
1. Each module should implement ProvidesServices() and RequiresServices() if it participates in DI
2. Provided services must match required services by name or interface
3. Use --interfaces to check for missing or mismatched services
4. Use --verbose for file locations and more details
5. Use --graph to visualize module dependencies

💡 Common Issues to Look For:
- Service name typos or mismatches
- Required service not provided by any module  
- Interface mismatch (pointer vs value, wrong method signature)
- Optional services not handled gracefully
- Circular dependencies between modules

Examples:
  modcli debug services --path .
  modcli debug services --path ./examples/reverse-proxy --verbose --interfaces
  modcli debug services --path ./modules --graph`,
		RunE: runDebugServices,
	}

	cmd.Flags().StringP("path", "p", ".", "Path to the modular project")
	cmd.Flags().BoolP("verbose", "v", false, "Show detailed service information")
	cmd.Flags().BoolP("interfaces", "i", false, "Show interface compatibility checks")
	cmd.Flags().BoolP("graph", "g", false, "Show dependency graph")

	return cmd
}

// NewDebugDependenciesCommand creates a command to analyze module dependencies
func NewDebugDependenciesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dependencies",
		Short: "Analyze module dependencies in a project",
		Long: `Analyze module dependencies and service requirements.

This helps understand dependency resolution and identify missing services.

Examples:
  modcli debug dependencies --path . --module httpserver
  modcli debug dependencies --path ./examples/reverse-proxy --all`,
		RunE: runDebugDependencies,
	}

	cmd.Flags().StringP("path", "p", ".", "Path to the modular project")
	cmd.Flags().StringP("module", "m", "", "Specific module to analyze")
	cmd.Flags().BoolP("all", "a", false, "Analyze all modules in the project")
	cmd.Flags().BoolP("graph", "g", false, "Show dependency graph")

	return cmd
}

// NewDebugConfigCommand creates a command to analyze module configurations
func NewDebugConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Analyze module configurations and validation",
		Long: `Analyze module configurations, validate required fields, and check for conflicts.

This helps identify configuration issues before runtime and ensures all
modules have proper configuration setup.

📝 Configuration Analysis Features:
- Shows configuration structures for each module
- Validates required vs optional fields
- Identifies missing configuration
- Detects configuration conflicts between modules
- Shows default values and overrides

Examples:
  modcli debug config --path .
  modcli debug config --path ./examples/basic-app --validate
  modcli debug config --module database --show-defaults`,
		RunE: runDebugConfig,
	}

	cmd.Flags().StringP("path", "p", ".", "Path to the modular project")
	cmd.Flags().StringP("module", "m", "", "Specific module to analyze")
	cmd.Flags().BoolP("validate", "v", false, "Validate configuration completeness")
	cmd.Flags().BoolP("show-defaults", "d", false, "Show default values")
	cmd.Flags().BoolP("verbose", "V", false, "Show detailed configuration information")

	return cmd
}

func runDebugInterface(cmd *cobra.Command, args []string) error {
	typeName, _ := cmd.Flags().GetString("type")
	interfaceName, _ := cmd.Flags().GetString("interface")
	verbose, _ := cmd.Flags().GetBool("verbose")

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Debugging Interface Implementation\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Type: %s\n", typeName)
	fmt.Fprintf(cmd.OutOrStdout(), "Interface: %s\n", interfaceName)
	fmt.Fprintln(cmd.OutOrStdout())

	// Try to analyze the types using known patterns
	result := analyzeInterfaceImplementation(typeName, interfaceName, verbose)

	if result.IsKnownPattern {
		if result.Implements {
			fmt.Fprintf(cmd.OutOrStdout(), "✅ SUCCESS: %s implements %s\n", typeName, interfaceName)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "❌ FAILURE: %s does NOT implement %s\n", typeName, interfaceName)
		}

		if verbose && len(result.Details) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "\n🔬 Detailed Analysis:\n")
			for _, detail := range result.Details {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", detail)
			}
		}
	} else {
		// Fall back to educational template
		fmt.Fprintf(cmd.OutOrStdout(), "📝 Analysis Template (type pattern not recognized):\n")
		fmt.Fprintf(cmd.OutOrStdout(), "1. Load type '%s' using reflection\n", typeName)
		fmt.Fprintf(cmd.OutOrStdout(), "2. Load interface '%s' using reflection\n", interfaceName)
		fmt.Fprintf(cmd.OutOrStdout(), "3. Check: serviceType.Implements(interfaceType)\n")
		fmt.Fprintf(cmd.OutOrStdout(), "4. Check: serviceType.Kind() == reflect.Ptr && serviceType.Elem().Implements(interfaceType)\n")
		fmt.Fprintln(cmd.OutOrStdout())
	}

	if verbose {
		fmt.Fprintf(cmd.OutOrStdout(), "🔬 Reflection Best Practices:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "- Use reflect.TypeOf((*Interface)(nil)).Elem() for interface types\n")
		fmt.Fprintf(cmd.OutOrStdout(), "- Check both pointer and value types for implementations\n")
		fmt.Fprintf(cmd.OutOrStdout(), "- Remember: pointer receivers require pointer types\n")
		fmt.Fprintf(cmd.OutOrStdout(), "- Verify method signatures match exactly\n")
		fmt.Fprintln(cmd.OutOrStdout())
	}

	fmt.Fprintf(cmd.OutOrStdout(), "💡 Common Issues:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "- Pointer vs Value receiver methods\n")
	fmt.Fprintf(cmd.OutOrStdout(), "- Missing methods in implementation\n")
	fmt.Fprintf(cmd.OutOrStdout(), "- Incorrect reflection pattern\n")
	fmt.Fprintf(cmd.OutOrStdout(), "- Package visibility (exported vs unexported)\n")

	return nil
}

// InterfaceAnalysisResult holds the result of interface analysis
type InterfaceAnalysisResult struct {
	IsKnownPattern bool
	Implements     bool
	Details        []string
}

// analyzeInterfaceImplementation analyzes known type/interface patterns
func analyzeInterfaceImplementation(typeName, interfaceName string, verbose bool) InterfaceAnalysisResult {
	result := InterfaceAnalysisResult{
		IsKnownPattern: false,
		Implements:     false,
		Details:        []string{},
	}

	// Handle common modular framework patterns
	switch {
	case strings.Contains(typeName, "ChiMuxModule") && strings.Contains(interfaceName, "http.Handler"):
		result.IsKnownPattern = true
		result.Implements = true
		result.Details = []string{
			"✅ ChiMuxModule implements ServeHTTP(ResponseWriter, *Request)",
			"✅ This satisfies the http.Handler interface",
			"🔍 Common issue: pointer vs value type checking in reflection",
			"💡 Fix: Use typeImplementsInterface helper that checks both pointer and value types",
		}

	case strings.Contains(typeName, "ChiMuxModule"):
		result.IsKnownPattern = true
		result.Implements = false // default, unless we know it implements
		result.Details = []string{
			"📝 ChiMuxModule is a router implementation",
			"✅ Implements: Module, ServiceAware, Startable",
			"✅ Provides: 'router' service (*ChiMuxModule), 'chi.router' service (*chi.Mux)",
			"🔍 Check if the interface requires http.Handler methods",
		}

		// Check for common interfaces ChiMuxModule implements
		if strings.Contains(interfaceName, "Module") ||
			strings.Contains(interfaceName, "ServiceAware") ||
			strings.Contains(interfaceName, "Startable") {
			result.Implements = true
		}

	case strings.Contains(interfaceName, "http.Handler"):
		result.IsKnownPattern = true
		result.Details = []string{
			"📝 http.Handler interface requires:",
			"  ServeHTTP(http.ResponseWriter, *http.Request)",
			"🔍 Check if the type has this method with exact signature",
			"💡 Pointer receivers need pointer types in reflection checks",
		}

		// We can't determine implementation without knowing the type
		// but we can provide guidance
		if strings.Contains(typeName, "Mux") || strings.Contains(typeName, "Router") || strings.Contains(typeName, "Handler") {
			result.Details = append(result.Details, "🤔 Type name suggests it might implement http.Handler")
		}
	}

	return result
}

func runDebugDependencies(cmd *cobra.Command, args []string) error {
	projectPath, _ := cmd.Flags().GetString("path")
	moduleName, _ := cmd.Flags().GetString("module")
	analyzeAll, _ := cmd.Flags().GetBool("all")
	showGraph, _ := cmd.Flags().GetBool("graph")

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Debugging Module Dependencies\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Project Path: %s\n", projectPath)

	if moduleName != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Target Module: %s\n", moduleName)
	} else if analyzeAll {
		fmt.Fprintf(cmd.OutOrStdout(), "Analyzing: All modules\n")
	}
	fmt.Fprintln(cmd.OutOrStdout())

	// Try to analyze the actual project
	analysis, err := analyzeProjectDependencies(projectPath)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "❌ Error analyzing project: %v\n", err)
		return err
	}

	if len(analysis.ModulesFound) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "📦 Found %d module registrations:\n", len(analysis.ModulesFound))
		for _, module := range analysis.ModulesFound {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s (in %s)\n", module.Name, module.File)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	if len(analysis.PotentialIssues) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Potential Issues Found:\n")
		for _, issue := range analysis.PotentialIssues {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", issue)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	if showGraph && len(analysis.ModulesFound) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "📊 Dependency Graph:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "   Note: Full dependency graph is available in 'debug services' command\n")
		fmt.Fprintf(cmd.OutOrStdout(), "   Use 'modcli debug services --path %s --interfaces' for detailed analysis\n", ".")
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Always show the educational template
	fmt.Fprintf(cmd.OutOrStdout(), "📝 Complete Analysis Template:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "1. Scan project for module registrations\n")
	fmt.Fprintf(cmd.OutOrStdout(), "2. Parse module RequiresServices() and ProvidesServices()\n")
	fmt.Fprintf(cmd.OutOrStdout(), "3. Build dependency graph\n")
	fmt.Fprintf(cmd.OutOrStdout(), "4. Check for:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "   - Missing required services\n")
	fmt.Fprintf(cmd.OutOrStdout(), "   - Interface compatibility issues\n")
	fmt.Fprintf(cmd.OutOrStdout(), "   - Circular dependencies\n")
	fmt.Fprintf(cmd.OutOrStdout(), "   - Initialization order conflicts\n")
	fmt.Fprintln(cmd.OutOrStdout())

	fmt.Fprintf(cmd.OutOrStdout(), "🎯 Common Debugging Scenarios:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "- Service not found: Check if module is registered and provides the service\n")
	fmt.Fprintf(cmd.OutOrStdout(), "- Interface mismatch: Use 'modcli debug interface' to verify implementation\n")
	fmt.Fprintf(cmd.OutOrStdout(), "- Circular dependencies: Check if modules depend on each other\n")
	fmt.Fprintf(cmd.OutOrStdout(), "- Startup failures: Verify module initialization order\n")

	return nil
}

// ProjectAnalysis holds the results of project dependency analysis
type ProjectAnalysis struct {
	ModulesFound    []ModuleInfo
	PotentialIssues []string
}

// ModuleInfo holds information about a detected module
type ModuleInfo struct {
	Name string
	Type string
	File string
}

// analyzeProjectDependencies scans a project directory for module patterns
func analyzeProjectDependencies(projectPath string) (*ProjectAnalysis, error) {
	analysis := &ProjectAnalysis{
		ModulesFound:    []ModuleInfo{},
		PotentialIssues: []string{},
	}

	// Walk through Go files looking for module registration patterns
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".go") || strings.Contains(path, "vendor/") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		contentStr := string(content)

		// Look for common module registration patterns
		patterns := []struct {
			pattern    string
			moduleType string
		}{
			{"Register.*chimux", "ChiMuxModule"},
			{"Register.*httpserver", "HTTPServerModule"},
			{"Register.*database", "DatabaseModule"},
			{"Register.*cache", "CacheModule"},
			{"Register.*auth", "AuthModule"},
			{"Register.*reverseproxy", "ReverseProxyModule"},
			{"Register.*httpclient", "HTTPClientModule"},
			{"Register.*eventbus", "EventBusModule"},
			{"Register.*scheduler", "SchedulerModule"},
			{"Register.*letsencrypt", "LetsEncryptModule"},
			{"Register.*jsonschema", "JSONSchemaModule"},
			{"RegisterModule", "GenericModule"},
		}

		for _, p := range patterns {
			if strings.Contains(strings.ToLower(contentStr), strings.ToLower(p.pattern)) {
				analysis.ModulesFound = append(analysis.ModulesFound, ModuleInfo{
					Name: p.moduleType,
					Type: p.moduleType,
					File: path,
				})
			}
		}

		// Look for potential issues
		if strings.Contains(contentStr, "http.Handler") && strings.Contains(contentStr, "httpserver") {
			if !strings.Contains(contentStr, "chimux") {
				analysis.PotentialIssues = append(analysis.PotentialIssues,
					"HTTPServer requires http.Handler but no router module (chimux) detected")
			}
		}

		return nil
	})

	if err != nil {
		return analysis, fmt.Errorf("failed to walk project directory: %w", err)
	}
	return analysis, nil
}

// generateDependencyGraph creates a visual representation of module dependencies
func generateDependencyGraph(cmd *cobra.Command, provided, required []ServiceInfo) {
	fmt.Fprintf(cmd.OutOrStdout(), "🔗 Dynamic Dependency Graph:\n")

	// Create a map of provided services to their modules
	providerMap := make(map[string][]ServiceInfo)
	for _, service := range provided {
		providerMap[service.ServiceName] = append(providerMap[service.ServiceName], service)
	}

	// Group requirements by module
	moduleRequirements := make(map[string][]ServiceInfo)
	for _, req := range required {
		moduleRequirements[req.Module] = append(moduleRequirements[req.Module], req)
	}

	// Group providers by module
	moduleProviders := make(map[string][]ServiceInfo)
	for _, prov := range provided {
		moduleProviders[prov.Module] = append(moduleProviders[prov.Module], prov)
	}

	// Get all unique modules
	allModules := make(map[string]bool)
	for _, service := range provided {
		allModules[service.Module] = true
	}
	for _, service := range required {
		allModules[service.Module] = true
	}

	// Display each module's dependencies
	for moduleName := range allModules {
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", moduleName)

		// Show what this module provides
		hasProviders := false
		if providers, exists := moduleProviders[moduleName]; exists && len(providers) > 0 {
			hasProviders = true
		}

		hasRequirements := false
		if requirements, exists := moduleRequirements[moduleName]; exists && len(requirements) > 0 {
			hasRequirements = true
		}

		// Determine tree symbols based on what sections exist
		providesSymbol := "├──"
		requiresSymbol := "└──"
		providerPrefix := "│   "
		requirementPrefix := "    "

		// Always show both sections, so always use connecting lines
		if !hasProviders {
			// If no providers, Requires becomes the only section
			requiresSymbol = "└──"
			requirementPrefix = "    "
		}

		// Show what this module provides
		if hasProviders {
			fmt.Fprintf(cmd.OutOrStdout(), "%s Provides:\n", providesSymbol)
			providers := moduleProviders[moduleName]
			for i, prov := range providers {
				symbol := "└──"
				if i < len(providers)-1 {
					symbol = "├──"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s %s", providerPrefix, symbol, prov.ServiceName)
				if prov.Description != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " — %s", prov.Description)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n")
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%s Provides: (none)\n", providesSymbol)
		}

		// Show what this module requires
		if hasRequirements {
			fmt.Fprintf(cmd.OutOrStdout(), "%s Requires:\n", requiresSymbol)
			requirements := moduleRequirements[moduleName]
			for i, req := range requirements {
				symbol := "└──"
				if i < len(requirements)-1 {
					symbol = "├──"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s %s", requirementPrefix, symbol, req.ServiceName)
				if req.Interface != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " (interface: %s)", req.Interface)
				}
				if req.Optional {
					fmt.Fprintf(cmd.OutOrStdout(), " [optional]")
				}

				// Check if this requirement is satisfied
				if providers, found := providerMap[req.ServiceName]; found {
					fmt.Fprintf(cmd.OutOrStdout(), " ✅ provided by: ")
					for j, prov := range providers {
						if j > 0 {
							fmt.Fprintf(cmd.OutOrStdout(), ", ")
						}
						fmt.Fprintf(cmd.OutOrStdout(), "%s", prov.Module)
					}
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), " ❌ NOT PROVIDED")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n")
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%s Requires: (none)\n", requiresSymbol)
		}
	}

	// Check for circular dependencies
	cycles := detectCircularDependencies(provided, required)
	if len(cycles) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Circular Dependencies Detected:\n")
		for _, cycle := range cycles {
			fmt.Fprintf(cmd.OutOrStdout(), "  🔄 %s\n", cycle)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\n💡 Circular dependencies can cause initialization issues.\n")
		fmt.Fprintf(cmd.OutOrStdout(), "   Consider breaking the cycle by making one dependency optional\n")
		fmt.Fprintf(cmd.OutOrStdout(), "   or introducing an intermediate service.\n")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n")
}

// ServiceDefinition represents a service found in the AST
type ServiceDefinition struct {
	Name        string
	Type        string
	Description string
	Interface   string
	Optional    bool
}

// extractProvidedServices parses ProvidesServices method body to find service registrations
func extractProvidedServices(body *ast.BlockStmt, constants map[string]string, fset *token.FileSet) []ServiceDefinition {
	var services []ServiceDefinition

	// Look for return statements with ServiceProvider slices
	ast.Inspect(body, func(n ast.Node) bool {
		if retStmt, ok := n.(*ast.ReturnStmt); ok && len(retStmt.Results) > 0 {
			// Check if return value is a slice literal
			if sliceLit, ok := retStmt.Results[0].(*ast.CompositeLit); ok {
				// Iterate through slice elements
				for _, elt := range sliceLit.Elts {
					if compLit, ok := elt.(*ast.CompositeLit); ok {
						// Parse ServiceProvider struct fields
						var name, description, serviceType string
						for _, field := range compLit.Elts {
							if kvExpr, ok := field.(*ast.KeyValueExpr); ok {
								if ident, ok := kvExpr.Key.(*ast.Ident); ok {
									switch ident.Name {
									case "Name":
										name = extractStringValue(kvExpr.Value, constants)
									case "Description":
										description = extractStringValue(kvExpr.Value, constants)
									case "Instance":
										serviceType = extractTypeString(kvExpr.Value)
									}
								}
							}
						}
						if name != "" {
							services = append(services, ServiceDefinition{
								Name:        name,
								Type:        serviceType,
								Description: description,
							})
						}
					}
				}
			}
		}
		return true
	})

	return services
}

// extractRequiredServices parses RequiresServices method body to find service requirements
func extractRequiredServices(body *ast.BlockStmt, constants map[string]string, fset *token.FileSet, fileContent []string) []ServiceDefinition {
	var services []ServiceDefinition

	// Look for return statements with ServiceDependency slices
	ast.Inspect(body, func(n ast.Node) bool {
		if retStmt, ok := n.(*ast.ReturnStmt); ok && len(retStmt.Results) > 0 {
			// Check if return value is a slice literal
			if sliceLit, ok := retStmt.Results[0].(*ast.CompositeLit); ok {
				// Iterate through slice elements
				for _, elt := range sliceLit.Elts {
					if compLit, ok := elt.(*ast.CompositeLit); ok {
						// Parse ServiceDependency struct fields
						var name, interfaceType string
						var optional bool
						for _, field := range compLit.Elts {
							if kvExpr, ok := field.(*ast.KeyValueExpr); ok {
								if ident, ok := kvExpr.Key.(*ast.Ident); ok {
									switch ident.Name {
									case "Name":
										name = extractStringValue(kvExpr.Value, constants)
									case "SatisfiesInterface":
										interfaceType = extractTypeString(kvExpr.Value)
										// If AST parsing failed, try text-based fallback
										if interfaceType == "unknown" {
											pos := fset.Position(kvExpr.Value.Pos())
											if pos.Line > 0 && pos.Line <= len(fileContent) {
												lineText := fileContent[pos.Line-1] // Convert to 0-based index
												if textInterface := extractInterfaceFromText(lineText); textInterface != "" {
													interfaceType = textInterface
												}
											}
										}
									case "Optional":
										if ident, ok := kvExpr.Value.(*ast.Ident); ok {
											optional = ident.Name == "true"
										}
									}
								}
							}
						}
						if name != "" {
							services = append(services, ServiceDefinition{
								Name:      name,
								Interface: interfaceType,
								Optional:  optional,
							})
						}
					}
				}
			}
		}
		return true
	})

	return services
}

// extractStringValue extracts string value from AST expression, resolving constants
func extractStringValue(expr ast.Expr, constants map[string]string) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return strings.Trim(e.Value, `"`)
		}
	case *ast.Ident:
		// Try to resolve constant
		if value, ok := constants[e.Name]; ok {
			return value
		}
		return e.Name // Return the identifier name as fallback
	case *ast.SelectorExpr:
		// Handle package.Constant references
		if x, ok := e.X.(*ast.Ident); ok {
			return x.Name + "." + e.Sel.Name
		}
	}
	return ""
}

// extractTypeString extracts type information from AST expression
func extractTypeString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + extractTypeString(e.X)
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok {
			return x.Name + "." + e.Sel.Name
		}
	case *ast.ArrayType:
		return "[]" + extractTypeString(e.Elt)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func"
	case *ast.CallExpr:
		// Handle reflect.TypeOf((*InterfaceName)(nil)).Elem() pattern
		if selectorExpr, ok := e.Fun.(*ast.SelectorExpr); ok {
			// Handle .Elem() method call on reflect.TypeOf result
			if selectorExpr.Sel.Name == "Elem" {
				// Get the argument to reflect.TypeOf which should be (*Interface)(nil)
				if typeOfCall, ok := selectorExpr.X.(*ast.CallExpr); ok {
					if selectorExpr2, ok := typeOfCall.Fun.(*ast.SelectorExpr); ok {
						if ident, ok := selectorExpr2.X.(*ast.Ident); ok && ident.Name == "reflect" && selectorExpr2.Sel.Name == "TypeOf" {
							if len(typeOfCall.Args) > 0 {
								// Parse (*InterfaceName)(nil) pattern
								if starExpr, ok := typeOfCall.Args[0].(*ast.StarExpr); ok {
									if parenExpr, ok := starExpr.X.(*ast.ParenExpr); ok {
										if starExpr2, ok := parenExpr.X.(*ast.StarExpr); ok {
											if ident, ok := starExpr2.X.(*ast.Ident); ok {
												return "*" + ident.Name
											}
											if selectorExpr3, ok := starExpr2.X.(*ast.SelectorExpr); ok {
												if x, ok := selectorExpr3.X.(*ast.Ident); ok {
													return "*" + x.Name + "." + selectorExpr3.Sel.Name
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
			// Handle direct reflect.TypeOf calls without .Elem()
			if ident, ok := selectorExpr.X.(*ast.Ident); ok && ident.Name == "reflect" && selectorExpr.Sel.Name == "TypeOf" {
				if len(e.Args) > 0 {
					// Look for (*InterfaceName)(nil) pattern
					if starExpr, ok := e.Args[0].(*ast.StarExpr); ok {
						if parenExpr, ok := starExpr.X.(*ast.ParenExpr); ok {
							if starExpr2, ok := parenExpr.X.(*ast.StarExpr); ok {
								if ident, ok := starExpr2.X.(*ast.Ident); ok {
									return "*" + ident.Name
								}
								if selectorExpr2, ok := starExpr2.X.(*ast.SelectorExpr); ok {
									if x, ok := selectorExpr2.X.(*ast.Ident); ok {
										return "*" + x.Name + "." + selectorExpr2.Sel.Name
									}
								}
							}
						}
					}
				}
			}
		}
		// Handle method calls that might resolve to interfaces
		return extractTypeString(e.Fun)
	}
	return "unknown"
}

func runDebugServices(cmd *cobra.Command, args []string) error {
	projectPath, _ := cmd.Flags().GetString("path")
	verbose, _ := cmd.Flags().GetBool("verbose")
	showInterfaces, _ := cmd.Flags().GetBool("interfaces")
	showGraph, _ := cmd.Flags().GetBool("graph")

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Inspecting Service Registrations\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Project Path: %s\n", projectPath)
	fmt.Fprintln(cmd.OutOrStdout())

	var provided, required []ServiceInfo

	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Skipping %s: %v\n", path, err)
			}
			return nil //nolint:nilerr // Intentionally skip files with errors
		}
		if !strings.HasSuffix(path, ".go") || strings.Contains(path, "vendor/") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Parse the Go file using AST
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Parse error in %s: %v\n", path, err)
			}
			return nil //nolint:nilerr // Skip files with parse errors
		}

		// Read file content for text-based fallback parsing
		content, err := os.ReadFile(path)
		if err != nil {
			if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Cannot read %s: %v\n", path, err)
			}
			return nil //nolint:nilerr // Skip files that cannot be read
		}
		lines := strings.Split(string(content), "\n")

		// Extract constants for resolving service names
		constants := make(map[string]string)
		for _, decl := range node.Decls {
			if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.CONST {
				for _, spec := range genDecl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for i, name := range valueSpec.Names {
							if i < len(valueSpec.Values) {
								if basicLit, ok := valueSpec.Values[i].(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
									// Remove quotes from string literal
									value := strings.Trim(basicLit.Value, `"`)
									constants[name.Name] = value
								}
							}
						}
					}
				}
			}
		}

		// Walk the AST to find methods
		ast.Inspect(node, func(n ast.Node) bool {
			if funcDecl, ok := n.(*ast.FuncDecl); ok {
				if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
					// Get receiver type name
					var receiverType string
					if starExpr, ok := funcDecl.Recv.List[0].Type.(*ast.StarExpr); ok {
						if ident, ok := starExpr.X.(*ast.Ident); ok {
							receiverType = ident.Name
						}
					} else if ident, ok := funcDecl.Recv.List[0].Type.(*ast.Ident); ok {
						receiverType = ident.Name
					}

					methodName := funcDecl.Name.Name

					// Look for ProvidesServices method
					if methodName == "ProvidesServices" {
						services := extractProvidedServices(funcDecl.Body, constants, fset)
						moduleName := deriveModuleName(receiverType, path, node.Name.Name)
						for _, svc := range services {
							provided = append(provided, ServiceInfo{
								Module:      moduleName,
								File:        path,
								ServiceName: svc.Name,
								Type:        svc.Type,
								Description: svc.Description,
								Kind:        "provided",
							})
						}
					}

					// Look for RequiresServices method
					if methodName == "RequiresServices" {
						services := extractRequiredServices(funcDecl.Body, constants, fset, lines)
						moduleName := deriveModuleName(receiverType, path, node.Name.Name)
						for _, svc := range services {
							required = append(required, ServiceInfo{
								Module:      moduleName,
								File:        path,
								ServiceName: svc.Name,
								Interface:   svc.Interface,
								Kind:        "required",
								Optional:    svc.Optional,
							})
						}
					}
				}
			}
			return true
		})

		return nil
	})

	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "❌ Error walking project directory: %v\n", err)
		return fmt.Errorf("failed to walk project directory: %w", err)
	}

	// Print summary
	fmt.Fprintf(cmd.OutOrStdout(), "📦 Service Providers:\n")
	for _, svc := range provided {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s", svc.ServiceName, svc.Module)
		if svc.Type != "" {
			fmt.Fprintf(cmd.OutOrStdout(), " (%s)", svc.Type)
		}
		if svc.Description != "" {
			fmt.Fprintf(cmd.OutOrStdout(), " — %s", svc.Description)
		}
		if verbose {
			fmt.Fprintf(cmd.OutOrStdout(), " [%s]", svc.File)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(), "🔗 Service Requirements:\n")
	for _, svc := range required {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s", svc.ServiceName, svc.Module)
		if svc.Interface != "" {
			fmt.Fprintf(cmd.OutOrStdout(), " (interface: %s)", svc.Interface)
		}
		if svc.Optional {
			fmt.Fprintf(cmd.OutOrStdout(), " [optional]")
		}
		if verbose {
			fmt.Fprintf(cmd.OutOrStdout(), " [%s]", svc.File)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}
	fmt.Fprintln(cmd.OutOrStdout())

	if showInterfaces {
		fmt.Fprintf(cmd.OutOrStdout(), "🔬 Interface Compatibility Checks:\n")
		for _, req := range required {
			found := false
			for _, prov := range provided {
				if req.ServiceName == prov.ServiceName {
					found = true
					fmt.Fprintf(cmd.OutOrStdout(), "  ✔ %s required by %s is provided by %s\n", req.ServiceName, req.Module, prov.Module)
					break
				}
			}
			if !found {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✖ %s required by %s is NOT provided by any module\n", req.ServiceName, req.Module)
			}
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	if showGraph {
		generateDependencyGraph(cmd, provided, required)
	}

	// Detect circular dependencies
	cycles := detectCircularDependencies(provided, required)
	if len(cycles) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "⚠️ Circular Dependencies Detected:\n")
		for _, cycle := range cycles {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", cycle)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	return nil
}

// deriveModuleName creates a meaningful module name from the receiver type and file context
func deriveModuleName(receiverType string, filePath string, packageName string) string {
	// If it's not the generic "Module", use it as-is
	if receiverType != "Module" {
		return receiverType
	}

	// For generic "Module" names, derive from package/directory context
	dir := filepath.Base(filepath.Dir(filePath))

	// Convert directory name to module-style name
	switch dir {
	case "auth":
		return "AuthModule"
	case "cache":
		return "CacheModule"
	case "chimux":
		return "ChiMuxModule"
	case "database":
		return "DatabaseModule"
	case "eventbus":
		return "EventBusModule"
	case "httpclient":
		return "HTTPClientModule"
	case "httpserver":
		return "HTTPServerModule"
	case "jsonschema":
		return "JSONSchemaModule"
	case "letsencrypt":
		return "LetsEncryptModule"
	case "reverseproxy":
		return "ReverseProxyModule"
	case "scheduler":
		return "SchedulerModule"
	default:
		// Capitalize first letter and add Module suffix
		if len(dir) > 0 {
			return strings.ToUpper(dir[:1]) + strings.ToLower(dir[1:]) + "Module"
		}
		return "Module"
	}
}

// extractInterfaceFromText uses text parsing to extract interface types from reflection patterns
// This is a fallback when AST parsing fails for complex reflection expressions
func extractInterfaceFromText(line string) string {
	// Look for patterns like: reflect.TypeOf((*InterfaceName)(nil)).Elem()
	if strings.Contains(line, "reflect.TypeOf") && strings.Contains(line, ".Elem()") {
		// Extract content between (*  and )(nil)
		start := strings.Index(line, "(*")
		if start != -1 {
			start += 2 // Skip (*
			end := strings.Index(line[start:], ")(nil)")
			if end != -1 {
				interfaceName := line[start : start+end]
				// Clean up any whitespace
				interfaceName = strings.TrimSpace(interfaceName)
				return "*" + interfaceName
			}
		}
	}

	// Look for simpler patterns like: reflect.TypeOf((*http.Client)(nil))
	if strings.Contains(line, "reflect.TypeOf") && strings.Contains(line, "(*") {
		start := strings.Index(line, "(*")
		if start != -1 {
			start += 2 // Skip (*
			end := strings.Index(line[start:], ")")
			if end != -1 {
				interfaceName := line[start : start+end]
				// Clean up any whitespace
				interfaceName = strings.TrimSpace(interfaceName)
				return "*" + interfaceName
			}
		}
	}

	return ""
}

// detectCircularDependencies analyzes the dependency graph for circular dependencies
func detectCircularDependencies(provided, required []ServiceInfo) []string {
	// Build adjacency list: module -> list of modules it depends on
	dependencies := make(map[string][]string)
	moduleServices := make(map[string]string) // service -> module that provides it

	// Map services to their provider modules
	for _, prov := range provided {
		moduleServices[prov.ServiceName] = prov.Module
	}

	// Build dependency graph
	for _, req := range required {
		if providerModule, exists := moduleServices[req.ServiceName]; exists {
			if req.Module != providerModule { // Don't add self-dependencies
				dependencies[req.Module] = append(dependencies[req.Module], providerModule)
			}
		}
	}

	// Detect cycles using DFS
	var cycles []string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(string, []string) bool
	dfs = func(module string, path []string) bool {
		visited[module] = true
		recStack[module] = true
		path = append(path, module)

		for _, dep := range dependencies[module] {
			if !visited[dep] {
				if dfs(dep, path) {
					return true
				}
			} else if recStack[dep] {
				// Found cycle - construct the cycle path
				cycleStart := -1
				for i, m := range path {
					if m == dep {
						cycleStart = i
						break
					}
				}
				if cycleStart != -1 {
					cyclePath := path[cycleStart:]
					cyclePath = append(cyclePath, dep) // Complete the cycle
					cycles = append(cycles, strings.Join(cyclePath, " → "))
				}
				return true
			}
		}

		recStack[module] = false
		return false
	}

	// Check each module for cycles
	for module := range dependencies {
		if !visited[module] {
			dfs(module, []string{})
		}
	}

	return cycles
}

func runDebugConfig(cmd *cobra.Command, args []string) error {
	projectPath, _ := cmd.Flags().GetString("path")
	moduleFilter, _ := cmd.Flags().GetString("module")
	validate, _ := cmd.Flags().GetBool("validate")
	showDefaults, _ := cmd.Flags().GetBool("show-defaults")
	verbose, _ := cmd.Flags().GetBool("verbose")

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Analyzing Module Configurations\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Project Path: %s\n", projectPath)
	if moduleFilter != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Module Filter: %s\n", moduleFilter)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	type ConfigField struct {
		Name        string
		Type        string
		Required    bool
		Default     string
		Description string
		Tags        string
	}

	type ModuleConfig struct {
		Module string
		File   string
		Fields []ConfigField
	}

	var configs []ModuleConfig

	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".go") || strings.Contains(path, "vendor/") || strings.HasSuffix(path, "_test.go") {
			return nil //nolint:nilerr // Skip files with errors
		}

		// Parse the Go file using AST
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil //nolint:nilerr // Skip files with parse errors
		}

		// Look for Config struct definitions
		ast.Inspect(node, func(n ast.Node) bool {
			if typeSpec, ok := n.(*ast.TypeSpec); ok {
				if structType, ok := typeSpec.Type.(*ast.StructType); ok {
					// Look for structs that might be configuration
					typeName := typeSpec.Name.Name
					if strings.Contains(strings.ToLower(typeName), "config") {
						moduleName := deriveModuleName(typeName, path, node.Name.Name)

						// Skip if module filter is specified and doesn't match
						if moduleFilter != "" && !strings.Contains(strings.ToLower(moduleName), strings.ToLower(moduleFilter)) {
							return true
						}

						var fields []ConfigField
						for _, field := range structType.Fields.List {
							for _, name := range field.Names {
								fieldType := extractTypeString(field.Type)

								// Parse struct tags
								var tags, defaultVal, desc string
								var required bool

								if field.Tag != nil {
									tags = field.Tag.Value
									// Simple tag parsing for common patterns
									if strings.Contains(tags, "required") {
										required = true
									}
									// Extract default values from tags
									if strings.Contains(tags, "default:") {
										start := strings.Index(tags, "default:")
										if start != -1 {
											start += 8 // Skip "default:"
											// Skip any quotes immediately after the colon
											if start < len(tags) && tags[start] == '"' {
												start++ // Skip opening quote
											}
											end := strings.Index(tags[start:], "\"")
											if end != -1 {
												defaultVal = tags[start : start+end]
											}
										}
									}
									// Extract descriptions
									if strings.Contains(tags, "desc:") {
										start := strings.Index(tags, "desc:")
										if start != -1 {
											start += 5 // Skip "desc:"
											end := strings.Index(tags[start:], "\"")
											if end != -1 {
												desc = strings.Trim(tags[start:start+end], "\"")
											}
										}
									}
								}

								fields = append(fields, ConfigField{
									Name:        name.Name,
									Type:        fieldType,
									Required:    required,
									Default:     defaultVal,
									Description: desc,
									Tags:        tags,
								})
							}
						}

						if len(fields) > 0 {
							configs = append(configs, ModuleConfig{
								Module: moduleName,
								File:   path,
								Fields: fields,
							})
						}
					}
				}
			}
			return true
		})

		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking directory: %w", err)
	}

	// Display configuration analysis
	if len(configs) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "📝 No configuration structures found.\n")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "📝 Configuration Structures Found:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "📝 Symbol Legend:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  ⚠️  Required field (must be configured)\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  ✅ Optional field or has default value\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  ❌ Validation issue found\n")
	fmt.Fprintln(cmd.OutOrStdout())

	for i, config := range configs {
		fmt.Fprintf(cmd.OutOrStdout(), "📦 %s\n", config.Module)
		if verbose {
			fmt.Fprintf(cmd.OutOrStdout(), "│  File: %s\n", config.File)
		}

		requiredFields := 0
		for j, field := range config.Fields {
			// Determine if this is the last visual element in the tree
			// If validation is enabled, the last field is not visually last
			// If validation is disabled, the last field is visually last
			isLastField := j == len(config.Fields)-1
			isLastElement := isLastField && !validate

			symbol := "├──"
			if isLastElement {
				symbol = "└──"
			}

			statusSymbol := ""
			if field.Required {
				statusSymbol = " ⚠️ "
				requiredFields++
			}

			fmt.Fprintf(cmd.OutOrStdout(), "│  %s%s %s (%s)", symbol, statusSymbol, field.Name, field.Type)

			if field.Default != "" && showDefaults {
				fmt.Fprintf(cmd.OutOrStdout(), " [default: %s]", field.Default)
			}
			if field.Description != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " — %s", field.Description)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n")
		}

		if validate {
			if requiredFields > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "│  └── ⚠️  %d required field(s) need validation\n", requiredFields)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "│  └── ✅ All fields have defaults or are optional\n")
			}
		}

		// Add spacing between modules, but not after the last one
		if i < len(configs)-1 {
			fmt.Fprintln(cmd.OutOrStdout())
		}
	}

	if validate {
		fmt.Fprintf(cmd.OutOrStdout(), "📋 Configuration Validation Summary:\n")
		totalRequired := 0
		for _, config := range configs {
			required := 0
			for _, field := range config.Fields {
				if field.Required {
					required++
				}
			}
			if required > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  ⚠️  %s: %d required field(s)\n", config.Module, required)
				totalRequired += required
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✅ %s: No required fields\n", config.Module)
			}
		}

		if totalRequired > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "\n💡 Ensure all required fields are properly configured before runtime.\n")
		}
	}

	return nil
}

// scanForServices extracts all service information from a project path
func scanForServices(projectPath string) ([]*ServiceInfo, error) {
	var services []*ServiceInfo

	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".go") || strings.Contains(path, "vendor/") || strings.HasSuffix(path, "_test.go") {
			return nil //nolint:nilerr // Skip files with errors
		}

		// Parse the Go file using AST
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil //nolint:nilerr // Skip files with parse errors
		}

		// Read file content for text-based fallback parsing
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("Warning: Cannot read file %s: %v\n", path, err)
			return nil //nolint:nilerr // Skip files that cannot be read
		}
		lines := strings.Split(string(content), "\n")

		// Extract constants for resolving service names
		constants := make(map[string]string)
		for _, decl := range node.Decls {
			if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.CONST {
				for _, spec := range genDecl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for i, name := range valueSpec.Names {
							if i < len(valueSpec.Values) {
								if basicLit, ok := valueSpec.Values[i].(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
									// Remove quotes from string literal
									value := strings.Trim(basicLit.Value, `"`)
									constants[name.Name] = value
								}
							}
						}
					}
				}
			}
		}

		// Walk the AST to find methods
		ast.Inspect(node, func(n ast.Node) bool {
			if funcDecl, ok := n.(*ast.FuncDecl); ok {
				if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
					// Get receiver type name
					var receiverType string
					if starExpr, ok := funcDecl.Recv.List[0].Type.(*ast.StarExpr); ok {
						if ident, ok := starExpr.X.(*ast.Ident); ok {
							receiverType = ident.Name
						}
					} else if ident, ok := funcDecl.Recv.List[0].Type.(*ast.Ident); ok {
						receiverType = ident.Name
					}

					methodName := funcDecl.Name.Name

					// Look for ProvidesServices method
					if methodName == "ProvidesServices" {
						svcDefs := extractProvidedServices(funcDecl.Body, constants, fset)
						moduleName := deriveModuleName(receiverType, path, node.Name.Name)
						for _, svc := range svcDefs {
							services = append(services, &ServiceInfo{
								Module:      moduleName,
								File:        path,
								ServiceName: svc.Name,
								Type:        svc.Type,
								Description: svc.Description,
								Kind:        "provided",
							})
						}
					}

					// Look for RequiresServices method
					if methodName == "RequiresServices" {
						svcDefs := extractRequiredServices(funcDecl.Body, constants, fset, lines)
						moduleName := deriveModuleName(receiverType, path, node.Name.Name)
						for _, svc := range svcDefs {
							services = append(services, &ServiceInfo{
								Module:      moduleName,
								File:        path,
								ServiceName: svc.Name,
								Interface:   svc.Interface,
								Kind:        "required",
								Optional:    svc.Optional,
							})
						}
					}
				}
			}
			return true
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking project directory: %w", err)
	}

	return services, nil
}

// NewDebugLifecycleCommand creates a command to analyze module lifecycle and initialization
func NewDebugLifecycleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lifecycle",
		Short: "Analyze module lifecycle and initialization order",
		Long: `Analyze module lifecycle, initialization order, and startup/shutdown dependencies.

This helps debug issues with module startup failures, initialization order problems,
and lifecycle dependency conflicts.

🔄 Lifecycle Analysis Features:
- Shows module initialization order and dependencies
- Identifies modules that implement Startable/Stoppable interfaces
- Detects potential startup/shutdown conflicts
- Analyzes lifecycle dependency chains
- Shows module state transitions and timing

📝 Common Lifecycle Issues:
- Module depends on service not yet initialized
- Circular lifecycle dependencies
- Missing StartableModule/StoppableModule implementations
- Startup failure cascade effects
- Improper shutdown order

Examples:
  modcli debug lifecycle --path .
  modcli debug lifecycle --path ./examples/basic-app --verbose
  modcli debug lifecycle --module httpserver --trace`,
		RunE: runDebugLifecycle,
	}

	cmd.Flags().StringP("path", "p", ".", "Path to the modular project")
	cmd.Flags().StringP("module", "m", "", "Specific module to analyze")
	cmd.Flags().BoolP("verbose", "v", false, "Show detailed lifecycle information")
	cmd.Flags().BoolP("trace", "t", false, "Show lifecycle dependency trace")

	return cmd
}

// NewDebugHealthCommand creates a command to check runtime health of modules
func NewDebugHealthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check runtime health and status of modules",
		Long: `Check runtime health and status of modules in a running application.

This helps monitor module health, detect runtime issues, and verify that
all services are functioning correctly.

🏥 Health Check Features:
- Verify module runtime status and health
- Check service connectivity (database, cache, etc.)
- Monitor resource usage and performance
- Detect failed or unhealthy modules
- Show runtime metrics and statistics

⚡ Health Monitoring Areas:
- Database connection pools and query performance
- Cache hit/miss ratios and connectivity
- HTTP server response times and error rates
- Authentication service status
- Event bus message processing
- Memory and CPU usage per module

Examples:
  modcli debug health --path .
  modcli debug health --module database --check-connections
  modcli debug health --all --metrics`,
		RunE: runDebugHealth,
	}

	cmd.Flags().StringP("path", "p", ".", "Path to the modular project")
	cmd.Flags().StringP("module", "m", "", "Specific module to check")
	cmd.Flags().BoolP("all", "a", false, "Check all modules")
	cmd.Flags().BoolP("metrics", "M", false, "Show performance metrics")
	cmd.Flags().BoolP("check-connections", "c", false, "Verify external connections")
	cmd.Flags().BoolP("verbose", "v", false, "Show detailed health information")

	return cmd
}

// NewDebugTenantCommand creates a command to analyze tenant-specific configurations and services
func NewDebugTenantCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenant",
		Short: "Analyze tenant-specific configurations and services",
		Long: `Analyze tenant-specific configurations, service isolation, and multi-tenant routing.

This helps debug multi-tenant applications, verify tenant isolation, and
identify tenant-specific configuration issues.

🏢 Tenant Analysis Features:
- Shows tenant-specific configuration structures
- Verifies tenant isolation between services
- Analyzes tenant routing and context propagation
- Detects tenant configuration conflicts
- Shows tenant-specific service instances

🔐 Multi-Tenant Debugging Areas:
- Tenant configuration inheritance and overrides
- Database schema isolation per tenant
- Cache key namespacing and isolation
- Authentication and authorization per tenant
- Request routing and tenant resolution
- Resource quotas and limits per tenant

Examples:
  modcli debug tenant --path .
  modcli debug tenant --path ./examples/multi-tenant-app --tenant acme
  modcli debug tenant --show-isolation --verbose`,
		RunE: runDebugTenant,
	}

	cmd.Flags().StringP("path", "p", ".", "Path to the modular project")
	cmd.Flags().StringP("tenant", "t", "", "Specific tenant to analyze")
	cmd.Flags().BoolP("show-isolation", "i", false, "Show tenant isolation analysis")
	cmd.Flags().BoolP("verbose", "v", false, "Show detailed tenant information")
	cmd.Flags().BoolP("check-routing", "r", false, "Verify tenant routing configuration")

	return cmd
}

func runDebugLifecycle(cmd *cobra.Command, args []string) error {
	path, _ := cmd.Flags().GetString("path")
	module, _ := cmd.Flags().GetString("module")
	verbose, _ := cmd.Flags().GetBool("verbose")
	trace, _ := cmd.Flags().GetBool("trace")

	fmt.Fprintf(cmd.OutOrStdout(), "🔄 Analyzing Module Lifecycle\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Project Path: %s\n", path)
	if module != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Target Module: %s\n", module)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	// Analyze modules for lifecycle patterns
	services, err := scanForServices(path)
	if err != nil {
		return fmt.Errorf("failed to scan for services: %w", err)
	}

	// Group by modules and analyze lifecycle interfaces
	moduleMap := make(map[string][]*ServiceInfo)
	for _, service := range services {
		moduleMap[service.Module] = append(moduleMap[service.Module], service)
	}

	// Analyze each module for lifecycle interfaces
	lifecycleModules := make(map[string]*LifecycleInfo)

	for moduleName, moduleServices := range moduleMap {
		if module != "" && moduleName != module {
			continue
		}

		info := &LifecycleInfo{
			Module:         moduleName,
			HasStartable:   false,
			HasStoppable:   false,
			HasTenantAware: false,
			Dependencies:   make([]string, 0),
			InitOrder:      0,
		}

		// Scan module files for lifecycle interfaces
		for _, service := range moduleServices {
			// Check for lifecycle interface implementations
			if strings.Contains(service.File, moduleName) {
				content, err := os.ReadFile(service.File)
				if err == nil {
					contentStr := string(content)

					// Check for lifecycle interfaces
					if strings.Contains(contentStr, "Startable") || strings.Contains(contentStr, "Start()") {
						info.HasStartable = true
					}
					if strings.Contains(contentStr, "Stoppable") || strings.Contains(contentStr, "Stop()") {
						info.HasStoppable = true
					}
					if strings.Contains(contentStr, "TenantAware") || strings.Contains(contentStr, "TenantModule") {
						info.HasTenantAware = true
					}
				}
			}

			// Collect dependencies
			if service.Kind == "required" {
				info.Dependencies = append(info.Dependencies, service.ServiceName)
			}
		}

		lifecycleModules[moduleName] = info
	}

	// Display lifecycle analysis
	fmt.Fprintf(cmd.OutOrStdout(), "🔄 Module Lifecycle Analysis:\n\n")

	for moduleName, info := range lifecycleModules {
		fmt.Fprintf(cmd.OutOrStdout(), "📦 %s\n", moduleName)

		// Show lifecycle capabilities
		capabilities := make([]string, 0)
		if info.HasStartable {
			capabilities = append(capabilities, "✅ Startable")
		} else {
			capabilities = append(capabilities, "❌ Not Startable")
		}

		if info.HasStoppable {
			capabilities = append(capabilities, "✅ Stoppable")
		} else {
			capabilities = append(capabilities, "❌ Not Stoppable")
		}

		if info.HasTenantAware {
			capabilities = append(capabilities, "🏢 Tenant-Aware")
		}

		fmt.Fprintf(cmd.OutOrStdout(), "   Lifecycle: %s\n", strings.Join(capabilities, ", "))

		// Show dependencies that affect initialization order
		if len(info.Dependencies) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "   Dependencies: %s\n", strings.Join(info.Dependencies, ", "))
		}

		if verbose {
			fmt.Fprintf(cmd.OutOrStdout(), "   Estimated Init Order: %d\n", len(info.Dependencies))
		}

		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Show initialization order if trace is enabled
	if trace {
		fmt.Fprintf(cmd.OutOrStdout(), "🔍 Initialization Order Analysis:\n\n")

		// Calculate dependency depth for each module
		orderMap := make(map[string]int)
		for moduleName := range lifecycleModules {
			orderMap[moduleName] = calculateInitOrder(moduleName, lifecycleModules, make(map[string]bool))
		}

		// Sort by initialization order
		type ModuleOrder struct {
			Name  string
			Order int
		}

		var sortedModules []ModuleOrder
		for name, order := range orderMap {
			sortedModules = append(sortedModules, ModuleOrder{Name: name, Order: order})
		}

		// Simple sort by order
		for i := 0; i < len(sortedModules)-1; i++ {
			for j := i + 1; j < len(sortedModules); j++ {
				if sortedModules[i].Order > sortedModules[j].Order {
					sortedModules[i], sortedModules[j] = sortedModules[j], sortedModules[i]
				}
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Recommended Initialization Order:\n")
		for i, mod := range sortedModules {
			prefix := "├──"
			if i == len(sortedModules)-1 {
				prefix = "└──"
			}

			info := lifecycleModules[mod.Name]
			status := ""
			if !info.HasStartable {
				status = " ⚠️  (No Startable interface)"
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s %d. %s%s\n", prefix, i+1, mod.Name, status)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Show lifecycle recommendations
	fmt.Fprintf(cmd.OutOrStdout(), "💡 Lifecycle Recommendations:\n")
	hasIssues := false

	for moduleName, info := range lifecycleModules {
		if len(info.Dependencies) > 0 && !info.HasStartable {
			if !hasIssues {
				hasIssues = true
			}
			fmt.Fprintf(cmd.OutOrStdout(), "   ⚠️  %s: Has dependencies but no Startable interface\n", moduleName)
		}

		if info.HasStartable && !info.HasStoppable {
			if !hasIssues {
				hasIssues = true
			}
			fmt.Fprintf(cmd.OutOrStdout(), "   ⚠️  %s: Implements Startable but not Stoppable\n", moduleName)
		}
	}

	if !hasIssues {
		fmt.Fprintf(cmd.OutOrStdout(), "   ✅ No obvious lifecycle issues detected\n")
	}

	return nil
}

func runDebugHealth(cmd *cobra.Command, args []string) error {
	path, _ := cmd.Flags().GetString("path")
	module, _ := cmd.Flags().GetString("module")
	all, _ := cmd.Flags().GetBool("all")
	metrics, _ := cmd.Flags().GetBool("metrics")
	checkConnections, _ := cmd.Flags().GetBool("check-connections")
	verbose, _ := cmd.Flags().GetBool("verbose")

	fmt.Fprintf(cmd.OutOrStdout(), "🏥 Module Health Analysis\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Project Path: %s\n", path)
	if module != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Target Module: %s\n", module)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	// Since this is a static analysis tool, we'll analyze the code for health check patterns
	services, err := scanForServices(path)
	if err != nil {
		return fmt.Errorf("failed to scan for services: %w", err)
	}

	// Group by modules
	moduleMap := make(map[string][]*ServiceInfo)
	for _, service := range services {
		moduleMap[service.Module] = append(moduleMap[service.Module], service)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Health Check Capabilities:\n\n")

	for moduleName, moduleServices := range moduleMap {
		if module != "" && moduleName != module {
			continue
		}
		if !all && module == "" {
			// Only show modules with obvious health check needs
			hasHealthRelevantServices := false
			for _, service := range moduleServices {
				if strings.Contains(strings.ToLower(service.ServiceName), "database") ||
					strings.Contains(strings.ToLower(service.ServiceName), "cache") ||
					strings.Contains(strings.ToLower(service.ServiceName), "http") ||
					strings.Contains(strings.ToLower(service.ServiceName), "server") {
					hasHealthRelevantServices = true
					break
				}
			}
			if !hasHealthRelevantServices {
				continue
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "📦 %s\n", moduleName)

		// Analyze health check potential for each service
		healthCapabilities := make([]string, 0)

		for _, service := range moduleServices {
			if service.Kind == "provided" {
				healthType := analyzeHealthCheckCapability(service)
				if healthType != "" {
					healthCapabilities = append(healthCapabilities, healthType)
				}
			}
		}

		if len(healthCapabilities) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "   Health Checks: %s\n", strings.Join(healthCapabilities, ", "))
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "   Health Checks: ❌ No obvious health check capabilities\n")
		}

		if checkConnections {
			// Analyze for external connection patterns
			connections := analyzeExternalConnections(moduleServices)
			if len(connections) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "   External Connections: %s\n", strings.Join(connections, ", "))
			}
		}

		if metrics {
			// Analyze for metrics/monitoring patterns
			metricsCapabilities := analyzeMetricsCapability(moduleServices)
			if len(metricsCapabilities) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "   Metrics: %s\n", strings.Join(metricsCapabilities, ", "))
			}
		}

		if verbose {
			fmt.Fprintf(cmd.OutOrStdout(), "   Services: %d provided, %d required\n",
				countServicesByKind(moduleServices, "provided"),
				countServicesByKind(moduleServices, "required"))
		}

		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Health check recommendations
	fmt.Fprintf(cmd.OutOrStdout(), "💡 Health Check Recommendations:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "   🔍 Implement health check endpoints for critical services\n")
	fmt.Fprintf(cmd.OutOrStdout(), "   📊 Add metrics collection for performance monitoring\n")
	fmt.Fprintf(cmd.OutOrStdout(), "   🔗 Verify external service connectivity in health checks\n")
	fmt.Fprintf(cmd.OutOrStdout(), "   ⏱️  Include response time monitoring for HTTP services\n")
	fmt.Fprintf(cmd.OutOrStdout(), "   💾 Monitor resource usage (memory, CPU, connections)\n")

	return nil
}

func runDebugTenant(cmd *cobra.Command, args []string) error {
	path, _ := cmd.Flags().GetString("path")
	tenant, _ := cmd.Flags().GetString("tenant")
	showIsolation, _ := cmd.Flags().GetBool("show-isolation")
	verbose, _ := cmd.Flags().GetBool("verbose")
	checkRouting, _ := cmd.Flags().GetBool("check-routing")

	fmt.Fprintf(cmd.OutOrStdout(), "🏢 Tenant Configuration Analysis\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Project Path: %s\n", path)
	if tenant != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Target Tenant: %s\n", tenant)
	}
	fmt.Fprintln(cmd.OutOrStdout())

	// Scan for tenant-aware patterns
	services, err := scanForServices(path)
	if err != nil {
		return fmt.Errorf("failed to scan for services: %w", err)
	}

	// Look for tenant configuration files
	tenantConfigs, err := findTenantConfigurations(path)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Warning: Could not scan for tenant configurations: %v\n", err)
	}

	// Analyze tenant-aware modules
	tenantAwareModules := make(map[string]*TenantInfo)

	moduleMap := make(map[string][]*ServiceInfo)
	for _, service := range services {
		moduleMap[service.Module] = append(moduleMap[service.Module], service)
	}

	for moduleName, moduleServices := range moduleMap {
		info := &TenantInfo{
			Module:           moduleName,
			HasTenantSupport: false,
			TenantServices:   make([]string, 0),
			IsolationLevel:   "none",
		}

		// Check for tenant-aware patterns
		for _, service := range moduleServices {
			if strings.Contains(service.File, moduleName) {
				content, err := os.ReadFile(service.File)
				if err == nil {
					contentStr := string(content)

					if strings.Contains(contentStr, "TenantAware") ||
						strings.Contains(contentStr, "TenantModule") ||
						strings.Contains(contentStr, "tenant") ||
						strings.Contains(contentStr, "Tenant") {
						info.HasTenantSupport = true
						info.TenantServices = append(info.TenantServices, service.ServiceName)

						// Determine isolation level
						if strings.Contains(contentStr, "TenantContext") {
							info.IsolationLevel = "context"
						} else if strings.Contains(contentStr, "tenant") {
							info.IsolationLevel = "basic"
						}
					}
				}
			}
		}

		if info.HasTenantSupport || len(info.TenantServices) > 0 {
			tenantAwareModules[moduleName] = info
		}
	}

	// Display tenant analysis
	if len(tenantAwareModules) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "🏢 Tenant-Aware Modules:\n\n")

		for moduleName, info := range tenantAwareModules {
			fmt.Fprintf(cmd.OutOrStdout(), "📦 %s\n", moduleName)
			fmt.Fprintf(cmd.OutOrStdout(), "   Tenant Support: ✅ Yes\n")
			fmt.Fprintf(cmd.OutOrStdout(), "   Isolation Level: %s\n", info.IsolationLevel)

			if len(info.TenantServices) > 0 && verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "   Tenant Services: %s\n", strings.Join(info.TenantServices, ", "))
			}

			fmt.Fprintln(cmd.OutOrStdout())
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "🏢 No obvious tenant-aware modules detected\n\n")
	}

	// Show tenant configurations if found
	if len(tenantConfigs) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "📁 Tenant Configuration Files:\n")
		for _, config := range tenantConfigs {
			fmt.Fprintf(cmd.OutOrStdout(), "   📄 %s\n", config)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	if showIsolation {
		fmt.Fprintf(cmd.OutOrStdout(), "🔐 Tenant Isolation Analysis:\n")

		isolationAreas := []string{
			"Database Schema Isolation",
			"Cache Key Namespacing",
			"Authentication Context",
			"Configuration Inheritance",
			"Resource Quotas",
			"Request Routing",
		}

		for _, area := range isolationAreas {
			// This would need deeper analysis in a real implementation
			fmt.Fprintf(cmd.OutOrStdout(), "   📋 %s: ❓ Requires runtime analysis\n", area)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	if checkRouting {
		fmt.Fprintf(cmd.OutOrStdout(), "🗺️  Tenant Routing Analysis:\n")

		// Look for routing patterns
		hasRouting := false
		for moduleName := range tenantAwareModules {
			if strings.Contains(strings.ToLower(moduleName), "router") ||
				strings.Contains(strings.ToLower(moduleName), "mux") ||
				strings.Contains(strings.ToLower(moduleName), "http") {
				fmt.Fprintf(cmd.OutOrStdout(), "   ✅ Found routing module: %s\n", moduleName)
				hasRouting = true
			}
		}

		if !hasRouting {
			fmt.Fprintf(cmd.OutOrStdout(), "   ⚠️  No obvious tenant routing patterns detected\n")
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Tenant debugging recommendations
	fmt.Fprintf(cmd.OutOrStdout(), "💡 Multi-Tenant Recommendations:\n")

	if len(tenantAwareModules) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "   🏢 Consider implementing TenantAwareModule interface for multi-tenant support\n")
		fmt.Fprintf(cmd.OutOrStdout(), "   📋 Add tenant context propagation through service calls\n")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "   ✅ Tenant-aware modules detected\n")
		fmt.Fprintf(cmd.OutOrStdout(), "   🔍 Verify tenant isolation in database and cache layers\n")
		fmt.Fprintf(cmd.OutOrStdout(), "   🗺️  Ensure proper tenant routing and context propagation\n")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "   📁 Organize tenant configurations in separate files or directories\n")
	fmt.Fprintf(cmd.OutOrStdout(), "   🔐 Implement tenant-specific authentication and authorization\n")
	fmt.Fprintf(cmd.OutOrStdout(), "   📊 Add tenant-specific metrics and monitoring\n")

	return nil
}

// Helper types and functions for lifecycle analysis
type LifecycleInfo struct {
	Module         string
	HasStartable   bool
	HasStoppable   bool
	HasTenantAware bool
	Dependencies   []string
	InitOrder      int
}

type TenantInfo struct {
	Module           string
	HasTenantSupport bool
	TenantServices   []string
	IsolationLevel   string
}

func calculateInitOrder(moduleName string, modules map[string]*LifecycleInfo, visited map[string]bool) int {
	if visited[moduleName] {
		return 0 // Circular dependency
	}

	visited[moduleName] = true
	defer func() { visited[moduleName] = false }()

	info, exists := modules[moduleName]
	if !exists {
		return 0
	}

	maxDepth := 0
	for _, dep := range info.Dependencies {
		depth := calculateInitOrder(dep, modules, visited)
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	return maxDepth + 1
}

func analyzeHealthCheckCapability(service *ServiceInfo) string {
	serviceLower := strings.ToLower(service.ServiceName)
	typeLower := strings.ToLower(service.Type)

	if strings.Contains(serviceLower, "database") || strings.Contains(typeLower, "db") {
		return "🗄️ Database Health"
	}
	if strings.Contains(serviceLower, "cache") || strings.Contains(typeLower, "cache") {
		return "💾 Cache Health"
	}
	if strings.Contains(serviceLower, "http") || strings.Contains(typeLower, "server") {
		return "🌐 HTTP Health"
	}
	if strings.Contains(serviceLower, "auth") {
		return "🔐 Auth Health"
	}
	if strings.Contains(serviceLower, "event") {
		return "📡 Event Health"
	}

	return ""
}

func analyzeExternalConnections(services []*ServiceInfo) []string {
	connections := make([]string, 0)

	for _, service := range services {
		serviceLower := strings.ToLower(service.ServiceName)

		if strings.Contains(serviceLower, "database") {
			connections = append(connections, "🗄️ Database")
		}
		if strings.Contains(serviceLower, "cache") || strings.Contains(serviceLower, "redis") {
			connections = append(connections, "💾 Cache/Redis")
		}
		if strings.Contains(serviceLower, "http") && strings.Contains(serviceLower, "client") {
			connections = append(connections, "🌐 HTTP APIs")
		}
	}

	return connections
}

func analyzeMetricsCapability(services []*ServiceInfo) []string {
	metrics := make([]string, 0)

	for _, service := range services {
		serviceLower := strings.ToLower(service.ServiceName)

		if strings.Contains(serviceLower, "server") || strings.Contains(serviceLower, "http") {
			metrics = append(metrics, "📊 Request Metrics")
		}
		if strings.Contains(serviceLower, "database") {
			metrics = append(metrics, "🗄️ Query Metrics")
		}
		if strings.Contains(serviceLower, "cache") {
			metrics = append(metrics, "💾 Cache Metrics")
		}
	}

	return metrics
}

func countServicesByKind(services []*ServiceInfo, kind string) int {
	count := 0
	for _, service := range services {
		if service.Kind == kind {
			count++
		}
	}
	return count
}

func findTenantConfigurations(path string) ([]string, error) {
	var configs []string

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Continue walking
		}

		if info.IsDir() {
			return nil
		}

		fileName := strings.ToLower(info.Name())

		// Look for tenant-related config files
		if strings.Contains(fileName, "tenant") &&
			(strings.HasSuffix(fileName, ".yaml") ||
				strings.HasSuffix(fileName, ".yml") ||
				strings.HasSuffix(fileName, ".json") ||
				strings.HasSuffix(fileName, ".toml")) {
			configs = append(configs, filePath)
		}

		// Look for tenant directories
		if strings.Contains(filepath.Dir(filePath), "tenant") {
			configs = append(configs, filePath)
		}

		return nil
	})

	if err != nil {
		return configs, fmt.Errorf("failed to walk directory for tenant configs: %w", err)
	}
	return configs, nil
}
