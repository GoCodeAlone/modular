package contract

import (
	"go/token"
	"go/types"
	"time"
)

// Contract represents the API contract of a Go package or module
type Contract struct {
	PackageName string              `json:"package_name"`
	ModulePath  string              `json:"module_path,omitempty"`
	Version     string              `json:"version,omitempty"`
	Timestamp   time.Time           `json:"timestamp"`
	Interfaces  []InterfaceContract `json:"interfaces,omitempty"`
	Types       []TypeContract      `json:"types,omitempty"`
	Functions   []FunctionContract  `json:"functions,omitempty"`
	Variables   []VariableContract  `json:"variables,omitempty"`
	Constants   []ConstantContract  `json:"constants,omitempty"`
}

// InterfaceContract represents an interface definition
type InterfaceContract struct {
	Name       string           `json:"name"`
	Package    string           `json:"package"`
	DocComment string           `json:"doc_comment,omitempty"`
	Methods    []MethodContract `json:"methods,omitempty"`
	Embedded   []string         `json:"embedded,omitempty"`
	Position   PositionInfo     `json:"position"`
}

// TypeContract represents a type definition (struct, alias, etc.)
type TypeContract struct {
	Name       string           `json:"name"`
	Package    string           `json:"package"`
	Kind       string           `json:"kind"` // "struct", "alias", "basic", etc.
	DocComment string           `json:"doc_comment,omitempty"`
	Fields     []FieldContract  `json:"fields,omitempty"`
	Methods    []MethodContract `json:"methods,omitempty"`
	Underlying string           `json:"underlying,omitempty"` // For type aliases
	Position   PositionInfo     `json:"position"`
}

// MethodContract represents a method signature
type MethodContract struct {
	Name       string          `json:"name"`
	DocComment string          `json:"doc_comment,omitempty"`
	Receiver   *ReceiverInfo   `json:"receiver,omitempty"`
	Parameters []ParameterInfo `json:"parameters,omitempty"`
	Results    []ParameterInfo `json:"results,omitempty"`
	Position   PositionInfo    `json:"position"`
}

// FunctionContract represents a function signature
type FunctionContract struct {
	Name       string          `json:"name"`
	Package    string          `json:"package"`
	DocComment string          `json:"doc_comment,omitempty"`
	Parameters []ParameterInfo `json:"parameters,omitempty"`
	Results    []ParameterInfo `json:"results,omitempty"`
	Position   PositionInfo    `json:"position"`
}

// FieldContract represents a struct field
type FieldContract struct {
	Name       string       `json:"name"`
	Type       string       `json:"type"`
	Tag        string       `json:"tag,omitempty"`
	DocComment string       `json:"doc_comment,omitempty"`
	Position   PositionInfo `json:"position"`
}

// VariableContract represents a package-level variable
type VariableContract struct {
	Name       string       `json:"name"`
	Package    string       `json:"package"`
	Type       string       `json:"type"`
	DocComment string       `json:"doc_comment,omitempty"`
	Position   PositionInfo `json:"position"`
}

// ConstantContract represents a package-level constant
type ConstantContract struct {
	Name       string       `json:"name"`
	Package    string       `json:"package"`
	Type       string       `json:"type"`
	Value      string       `json:"value,omitempty"`
	DocComment string       `json:"doc_comment,omitempty"`
	Position   PositionInfo `json:"position"`
}

// ReceiverInfo represents method receiver information
type ReceiverInfo struct {
	Name    string `json:"name,omitempty"`
	Type    string `json:"type"`
	Pointer bool   `json:"pointer"`
}

// ParameterInfo represents parameter or result information
type ParameterInfo struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type"`
}

// PositionInfo represents source position information
type PositionInfo struct {
	Filename string `json:"filename"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

// ContractDiff represents differences between two contracts
type ContractDiff struct {
	PackageName     string           `json:"package_name"`
	OldVersion      string           `json:"old_version,omitempty"`
	NewVersion      string           `json:"new_version,omitempty"`
	BreakingChanges []BreakingChange `json:"breaking_changes,omitempty"`
	AddedItems      []AddedItem      `json:"added_items,omitempty"`
	ModifiedItems   []ModifiedItem   `json:"modified_items,omitempty"`
	Summary         DiffSummary      `json:"summary"`
}

// BreakingChange represents a breaking API change
type BreakingChange struct {
	Type        string `json:"type"` // "removed_interface", "removed_method", "changed_signature", etc.
	Item        string `json:"item"` // Name of the affected item
	Description string `json:"description"`
	OldValue    string `json:"old_value,omitempty"`
	NewValue    string `json:"new_value,omitempty"`
}

// AddedItem represents a newly added API item
type AddedItem struct {
	Type        string `json:"type"` // "interface", "method", "function", etc.
	Item        string `json:"item"`
	Description string `json:"description"`
}

// ModifiedItem represents a modified API item (non-breaking)
type ModifiedItem struct {
	Type        string `json:"type"`
	Item        string `json:"item"`
	Description string `json:"description"`
	OldValue    string `json:"old_value,omitempty"`
	NewValue    string `json:"new_value,omitempty"`
}

// DiffSummary provides a high-level summary of changes
type DiffSummary struct {
	TotalBreakingChanges int  `json:"total_breaking_changes"`
	TotalAdditions       int  `json:"total_additions"`
	TotalModifications   int  `json:"total_modifications"`
	HasBreakingChanges   bool `json:"has_breaking_changes"`
}

// getPositionInfo extracts position information from a types.Object or ast.Node
func getPositionInfo(fset *token.FileSet, pos token.Pos) PositionInfo {
	if !pos.IsValid() {
		return PositionInfo{}
	}

	position := fset.Position(pos)
	return PositionInfo{
		Filename: position.Filename,
		Line:     position.Line,
		Column:   position.Column,
	}
}

// formatType formats a types.Type as a string for contract representation
func formatType(typ types.Type) string {
	return types.TypeString(typ, func(p *types.Package) string {
		if p == nil {
			return ""
		}
		return p.Name()
	})
}
