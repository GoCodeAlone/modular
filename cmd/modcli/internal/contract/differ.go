package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Define static errors
var (
	ErrNilContracts      = errors.New("contracts cannot be nil")
	ErrUnsupportedFormat = errors.New("unsupported output format")
	ErrNoPackagesFound   = errors.New("no packages found")
	ErrNoGoPackagesFound = errors.New("no Go packages found in directory")
	ErrPackageErrors     = errors.New("package compilation errors")
)

// Differ handles comparing two API contracts
type Differ struct {
	// IgnorePositions determines whether to ignore source position changes
	IgnorePositions bool
	// IgnoreComments determines whether to ignore documentation comment changes
	IgnoreComments bool
}

// NewDiffer creates a new contract differ
func NewDiffer() *Differ {
	return &Differ{
		IgnorePositions: true,
		IgnoreComments:  false,
	}
}

// Compare compares two contracts and returns the differences
func (d *Differ) Compare(old, new *Contract) (*ContractDiff, error) {
	if old == nil || new == nil {
		return nil, ErrNilContracts
	}

	diff := &ContractDiff{
		PackageName:     new.PackageName,
		OldVersion:      old.Version,
		NewVersion:      new.Version,
		BreakingChanges: []BreakingChange{},
		AddedItems:      []AddedItem{},
		ModifiedItems:   []ModifiedItem{},
	}

	// Compare interfaces
	d.compareInterfaces(old.Interfaces, new.Interfaces, diff)

	// Compare types
	d.compareTypes(old.Types, new.Types, diff)

	// Compare functions
	d.compareFunctions(old.Functions, new.Functions, diff)

	// Compare variables
	d.compareVariables(old.Variables, new.Variables, diff)

	// Compare constants
	d.compareConstants(old.Constants, new.Constants, diff)

	// Calculate summary
	diff.Summary = DiffSummary{
		TotalBreakingChanges: len(diff.BreakingChanges),
		TotalAdditions:       len(diff.AddedItems),
		TotalModifications:   len(diff.ModifiedItems),
		HasBreakingChanges:   len(diff.BreakingChanges) > 0,
	}

	return diff, nil
}

// compareInterfaces compares interface contracts
func (d *Differ) compareInterfaces(old, new []InterfaceContract, diff *ContractDiff) {
	oldMap := make(map[string]InterfaceContract)
	newMap := make(map[string]InterfaceContract)

	for _, iface := range old {
		oldMap[iface.Name] = iface
	}
	for _, iface := range new {
		newMap[iface.Name] = iface
	}

	// Check for removed interfaces (breaking change)
	for name, oldIface := range oldMap {
		if _, exists := newMap[name]; !exists {
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Type:        "removed_interface",
				Item:        name,
				Description: fmt.Sprintf("Interface %s was removed", name),
				OldValue:    d.interfaceSignature(oldIface),
			})
		}
	}

	// Check for added interfaces
	for name := range newMap {
		if _, exists := oldMap[name]; !exists {
			diff.AddedItems = append(diff.AddedItems, AddedItem{
				Type:        "interface",
				Item:        name,
				Description: fmt.Sprintf("New interface %s was added", name),
			})
		}
	}

	// Check for modified interfaces
	for name, newIface := range newMap {
		if oldIface, exists := oldMap[name]; exists {
			d.compareInterfaceMethods(oldIface, newIface, diff)
		}
	}
}

// compareInterfaceMethods compares methods within an interface
func (d *Differ) compareInterfaceMethods(old, new InterfaceContract, diff *ContractDiff) {
	oldMethods := make(map[string]MethodContract)
	newMethods := make(map[string]MethodContract)

	for _, method := range old.Methods {
		oldMethods[method.Name] = method
	}
	for _, method := range new.Methods {
		newMethods[method.Name] = method
	}

	// Check for removed methods (breaking change)
	for methodName, oldMethod := range oldMethods {
		if _, exists := newMethods[methodName]; !exists {
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Type:        "removed_method",
				Item:        fmt.Sprintf("%s.%s", old.Name, methodName),
				Description: fmt.Sprintf("Method %s was removed from interface %s", methodName, old.Name),
				OldValue:    d.methodSignature(oldMethod),
			})
		}
	}

	// Check for added methods
	for methodName := range newMethods {
		if _, exists := oldMethods[methodName]; !exists {
			diff.AddedItems = append(diff.AddedItems, AddedItem{
				Type:        "method",
				Item:        fmt.Sprintf("%s.%s", new.Name, methodName),
				Description: fmt.Sprintf("New method %s was added to interface %s", methodName, new.Name),
			})
		}
	}

	// Check for modified method signatures (breaking change)
	for methodName, newMethod := range newMethods {
		if oldMethod, exists := oldMethods[methodName]; exists {
			if !d.methodSignaturesEqual(oldMethod, newMethod) {
				diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
					Type:        "changed_method_signature",
					Item:        fmt.Sprintf("%s.%s", new.Name, methodName),
					Description: fmt.Sprintf("Method %s signature changed in interface %s", methodName, new.Name),
					OldValue:    d.methodSignature(oldMethod),
					NewValue:    d.methodSignature(newMethod),
				})
			} else if !d.IgnoreComments && oldMethod.DocComment != newMethod.DocComment {
				diff.ModifiedItems = append(diff.ModifiedItems, ModifiedItem{
					Type:        "method_comment",
					Item:        fmt.Sprintf("%s.%s", new.Name, methodName),
					Description: fmt.Sprintf("Method %s documentation changed in interface %s", methodName, new.Name),
					OldValue:    oldMethod.DocComment,
					NewValue:    newMethod.DocComment,
				})
			}
		}
	}
}

// compareTypes compares type contracts
func (d *Differ) compareTypes(old, new []TypeContract, diff *ContractDiff) {
	oldMap := make(map[string]TypeContract)
	newMap := make(map[string]TypeContract)

	for _, typ := range old {
		oldMap[typ.Name] = typ
	}
	for _, typ := range new {
		newMap[typ.Name] = typ
	}

	// Check for removed types (breaking change)
	for name, oldType := range oldMap {
		if _, exists := newMap[name]; !exists {
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Type:        "removed_type",
				Item:        name,
				Description: fmt.Sprintf("Type %s was removed", name),
				OldValue:    d.typeSignature(oldType),
			})
		}
	}

	// Check for added types
	for name := range newMap {
		if _, exists := oldMap[name]; !exists {
			diff.AddedItems = append(diff.AddedItems, AddedItem{
				Type:        "type",
				Item:        name,
				Description: fmt.Sprintf("New type %s was added", name),
			})
		}
	}

	// Check for modified types
	for name, newType := range newMap {
		if oldType, exists := oldMap[name]; exists {
			d.compareTypeDetails(oldType, newType, diff)
		}
	}
}

// compareTypeDetails compares the details of a specific type
func (d *Differ) compareTypeDetails(old, new TypeContract, diff *ContractDiff) {
	// Check for changes in type kind (breaking change)
	if old.Kind != new.Kind {
		diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
			Type:        "changed_type_kind",
			Item:        old.Name,
			Description: fmt.Sprintf("Type %s kind changed from %s to %s", old.Name, old.Kind, new.Kind),
			OldValue:    old.Kind,
			NewValue:    new.Kind,
		})
		return // Don't check further details if kind changed
	}

	// Compare struct fields if it's a struct
	if old.Kind == "struct" {
		d.compareStructFields(old, new, diff)
	}

	// Compare methods
	d.compareTypeMethods(old, new, diff)

	// Check for underlying type changes (potentially breaking)
	if old.Underlying != new.Underlying && (old.Underlying != "" || new.Underlying != "") {
		if old.Kind == "alias" {
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Type:        "changed_type_underlying",
				Item:        old.Name,
				Description: fmt.Sprintf("Type alias %s underlying type changed", old.Name),
				OldValue:    old.Underlying,
				NewValue:    new.Underlying,
			})
		}
	}
}

// compareStructFields compares struct fields
func (d *Differ) compareStructFields(old, new TypeContract, diff *ContractDiff) {
	oldFields := make(map[string]FieldContract)
	newFields := make(map[string]FieldContract)

	for _, field := range old.Fields {
		oldFields[field.Name] = field
	}
	for _, field := range new.Fields {
		newFields[field.Name] = field
	}

	// Check for removed fields (breaking change)
	for fieldName, oldField := range oldFields {
		if _, exists := newFields[fieldName]; !exists {
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Type:        "removed_field",
				Item:        fmt.Sprintf("%s.%s", old.Name, fieldName),
				Description: fmt.Sprintf("Field %s was removed from struct %s", fieldName, old.Name),
				OldValue:    d.fieldSignature(oldField),
			})
		}
	}

	// Check for added fields
	for fieldName := range newFields {
		if _, exists := oldFields[fieldName]; !exists {
			diff.AddedItems = append(diff.AddedItems, AddedItem{
				Type:        "field",
				Item:        fmt.Sprintf("%s.%s", new.Name, fieldName),
				Description: fmt.Sprintf("New field %s was added to struct %s", fieldName, new.Name),
			})
		}
	}

	// Check for modified fields (breaking change)
	for fieldName, newField := range newFields {
		if oldField, exists := oldFields[fieldName]; exists {
			if oldField.Type != newField.Type {
				diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
					Type:        "changed_field_type",
					Item:        fmt.Sprintf("%s.%s", new.Name, fieldName),
					Description: fmt.Sprintf("Field %s type changed in struct %s", fieldName, new.Name),
					OldValue:    oldField.Type,
					NewValue:    newField.Type,
				})
			} else if oldField.Tag != newField.Tag {
				diff.ModifiedItems = append(diff.ModifiedItems, ModifiedItem{
					Type:        "field_tag",
					Item:        fmt.Sprintf("%s.%s", new.Name, fieldName),
					Description: fmt.Sprintf("Field %s tag changed in struct %s", fieldName, new.Name),
					OldValue:    oldField.Tag,
					NewValue:    newField.Tag,
				})
			}
		}
	}
}

// compareTypeMethods compares methods of a type
func (d *Differ) compareTypeMethods(old, new TypeContract, diff *ContractDiff) {
	oldMethods := make(map[string]MethodContract)
	newMethods := make(map[string]MethodContract)

	for _, method := range old.Methods {
		oldMethods[method.Name] = method
	}
	for _, method := range new.Methods {
		newMethods[method.Name] = method
	}

	// Check for removed methods (breaking change)
	for methodName, oldMethod := range oldMethods {
		if _, exists := newMethods[methodName]; !exists {
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Type:        "removed_method",
				Item:        fmt.Sprintf("%s.%s", old.Name, methodName),
				Description: fmt.Sprintf("Method %s was removed from type %s", methodName, old.Name),
				OldValue:    d.methodSignature(oldMethod),
			})
		}
	}

	// Check for added methods
	for methodName := range newMethods {
		if _, exists := oldMethods[methodName]; !exists {
			diff.AddedItems = append(diff.AddedItems, AddedItem{
				Type:        "method",
				Item:        fmt.Sprintf("%s.%s", new.Name, methodName),
				Description: fmt.Sprintf("New method %s was added to type %s", methodName, new.Name),
			})
		}
	}

	// Check for modified method signatures (breaking change)
	for methodName, newMethod := range newMethods {
		if oldMethod, exists := oldMethods[methodName]; exists {
			if !d.methodSignaturesEqual(oldMethod, newMethod) {
				diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
					Type:        "changed_method_signature",
					Item:        fmt.Sprintf("%s.%s", new.Name, methodName),
					Description: fmt.Sprintf("Method %s signature changed in type %s", methodName, new.Name),
					OldValue:    d.methodSignature(oldMethod),
					NewValue:    d.methodSignature(newMethod),
				})
			}
		}
	}
}

// compareFunctions compares function contracts
func (d *Differ) compareFunctions(old, new []FunctionContract, diff *ContractDiff) {
	oldMap := make(map[string]FunctionContract)
	newMap := make(map[string]FunctionContract)

	for _, fn := range old {
		oldMap[fn.Name] = fn
	}
	for _, fn := range new {
		newMap[fn.Name] = fn
	}

	// Check for removed functions (breaking change)
	for name, oldFunc := range oldMap {
		if _, exists := newMap[name]; !exists {
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Type:        "removed_function",
				Item:        name,
				Description: fmt.Sprintf("Function %s was removed", name),
				OldValue:    d.functionSignature(oldFunc),
			})
		}
	}

	// Check for added functions
	for name := range newMap {
		if _, exists := oldMap[name]; !exists {
			diff.AddedItems = append(diff.AddedItems, AddedItem{
				Type:        "function",
				Item:        name,
				Description: fmt.Sprintf("New function %s was added", name),
			})
		}
	}

	// Check for modified function signatures (breaking change)
	for name, newFunc := range newMap {
		if oldFunc, exists := oldMap[name]; exists {
			if !d.functionSignaturesEqual(oldFunc, newFunc) {
				diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
					Type:        "changed_function_signature",
					Item:        name,
					Description: fmt.Sprintf("Function %s signature changed", name),
					OldValue:    d.functionSignature(oldFunc),
					NewValue:    d.functionSignature(newFunc),
				})
			}
		}
	}
}

// compareVariables compares variable contracts
func (d *Differ) compareVariables(old, new []VariableContract, diff *ContractDiff) {
	oldMap := make(map[string]VariableContract)
	newMap := make(map[string]VariableContract)

	for _, v := range old {
		oldMap[v.Name] = v
	}
	for _, v := range new {
		newMap[v.Name] = v
	}

	// Check for removed variables (breaking change)
	for name, oldVar := range oldMap {
		if _, exists := newMap[name]; !exists {
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Type:        "removed_variable",
				Item:        name,
				Description: fmt.Sprintf("Variable %s was removed", name),
				OldValue:    fmt.Sprintf("var %s %s", name, oldVar.Type),
			})
		}
	}

	// Check for added variables
	for name := range newMap {
		if _, exists := oldMap[name]; !exists {
			diff.AddedItems = append(diff.AddedItems, AddedItem{
				Type:        "variable",
				Item:        name,
				Description: fmt.Sprintf("New variable %s was added", name),
			})
		}
	}

	// Check for modified variable types (breaking change)
	for name, newVar := range newMap {
		if oldVar, exists := oldMap[name]; exists {
			if oldVar.Type != newVar.Type {
				diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
					Type:        "changed_variable_type",
					Item:        name,
					Description: fmt.Sprintf("Variable %s type changed", name),
					OldValue:    oldVar.Type,
					NewValue:    newVar.Type,
				})
			}
		}
	}
}

// compareConstants compares constant contracts
func (d *Differ) compareConstants(old, new []ConstantContract, diff *ContractDiff) {
	oldMap := make(map[string]ConstantContract)
	newMap := make(map[string]ConstantContract)

	for _, c := range old {
		oldMap[c.Name] = c
	}
	for _, c := range new {
		newMap[c.Name] = c
	}

	// Check for removed constants (breaking change)
	for name, oldConst := range oldMap {
		if _, exists := newMap[name]; !exists {
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Type:        "removed_constant",
				Item:        name,
				Description: fmt.Sprintf("Constant %s was removed", name),
				OldValue:    fmt.Sprintf("const %s %s = %s", name, oldConst.Type, oldConst.Value),
			})
		}
	}

	// Check for added constants
	for name := range newMap {
		if _, exists := oldMap[name]; !exists {
			diff.AddedItems = append(diff.AddedItems, AddedItem{
				Type:        "constant",
				Item:        name,
				Description: fmt.Sprintf("New constant %s was added", name),
			})
		}
	}

	// Check for modified constants
	for name, newConst := range newMap {
		if oldConst, exists := oldMap[name]; exists {
			if oldConst.Type != newConst.Type {
				diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
					Type:        "changed_constant_type",
					Item:        name,
					Description: fmt.Sprintf("Constant %s type changed", name),
					OldValue:    oldConst.Type,
					NewValue:    newConst.Type,
				})
			} else if oldConst.Value != newConst.Value {
				// Value change may or may not be breaking, but it's worth noting
				diff.ModifiedItems = append(diff.ModifiedItems, ModifiedItem{
					Type:        "constant_value",
					Item:        name,
					Description: fmt.Sprintf("Constant %s value changed", name),
					OldValue:    oldConst.Value,
					NewValue:    newConst.Value,
				})
			}
		}
	}
}

// Helper methods for signature comparison and formatting

func (d *Differ) methodSignaturesEqual(old, new MethodContract) bool {
	if old.Name != new.Name {
		return false
	}

	// Compare receiver
	if !d.receiversEqual(old.Receiver, new.Receiver) {
		return false
	}

	// Compare parameters
	if !d.parametersEqual(old.Parameters, new.Parameters) {
		return false
	}

	// Compare results
	return d.parametersEqual(old.Results, new.Results)
}

func (d *Differ) functionSignaturesEqual(old, new FunctionContract) bool {
	if old.Name != new.Name {
		return false
	}

	// Compare parameters
	if !d.parametersEqual(old.Parameters, new.Parameters) {
		return false
	}

	// Compare results
	return d.parametersEqual(old.Results, new.Results)
}

func (d *Differ) receiversEqual(old, new *ReceiverInfo) bool {
	if old == nil && new == nil {
		return true
	}
	if old == nil || new == nil {
		return false
	}
	return old.Type == new.Type && old.Pointer == new.Pointer
}

func (d *Differ) parametersEqual(old, new []ParameterInfo) bool {
	if len(old) != len(new) {
		return false
	}

	for i, oldParam := range old {
		newParam := new[i]
		if oldParam.Type != newParam.Type {
			return false
		}
		// Note: Parameter names can change without breaking compatibility
	}

	return true
}

// Signature formatting methods

func (d *Differ) interfaceSignature(iface InterfaceContract) string {
	var methods []string
	for _, method := range iface.Methods {
		methods = append(methods, d.methodSignature(method))
	}
	sort.Strings(methods)
	return fmt.Sprintf("interface { %s }", strings.Join(methods, "; "))
}

func (d *Differ) methodSignature(method MethodContract) string {
	var parts []string

	if method.Receiver != nil {
		receiver := method.Receiver.Type
		if method.Receiver.Pointer {
			receiver = "*" + receiver
		}
		parts = append(parts, fmt.Sprintf("(%s)", receiver))
	}

	parts = append(parts, method.Name)
	parts = append(parts, fmt.Sprintf("(%s)", d.formatParameters(method.Parameters)))

	if len(method.Results) > 0 {
		parts = append(parts, fmt.Sprintf("(%s)", d.formatParameters(method.Results)))
	}

	return strings.Join(parts, " ")
}

func (d *Differ) functionSignature(fn FunctionContract) string {
	parts := []string{fn.Name}
	parts = append(parts, fmt.Sprintf("(%s)", d.formatParameters(fn.Parameters)))

	if len(fn.Results) > 0 {
		parts = append(parts, fmt.Sprintf("(%s)", d.formatParameters(fn.Results)))
	}

	return strings.Join(parts, " ")
}

func (d *Differ) typeSignature(typ TypeContract) string {
	return fmt.Sprintf("type %s %s", typ.Name, typ.Kind)
}

func (d *Differ) fieldSignature(field FieldContract) string {
	signature := fmt.Sprintf("%s %s", field.Name, field.Type)
	if field.Tag != "" {
		signature += fmt.Sprintf(" `%s`", field.Tag)
	}
	return signature
}

func (d *Differ) formatParameters(params []ParameterInfo) string {
	var formatted []string
	for _, param := range params {
		if param.Name != "" {
			formatted = append(formatted, fmt.Sprintf("%s %s", param.Name, param.Type))
		} else {
			formatted = append(formatted, param.Type)
		}
	}
	return strings.Join(formatted, ", ")
}

// SaveToFile saves the diff to a JSON file
func (d *ContractDiff) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal diff: %w", err)
	}

	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write diff file: %w", err)
	}

	return nil
}

// LoadDiffFromFile loads a contract diff from a JSON file
func LoadDiffFromFile(filename string) (*ContractDiff, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read diff file: %w", err)
	}

	var diff ContractDiff
	if err := json.Unmarshal(data, &diff); err != nil {
		return nil, fmt.Errorf("failed to unmarshal diff: %w", err)
	}

	return &diff, nil
}
