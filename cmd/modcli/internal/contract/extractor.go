package contract

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/tools/go/packages"
)

// Extractor handles API contract extraction from Go packages
type Extractor struct {
	// IncludePrivate determines whether to include unexported items
	IncludePrivate bool
	// IncludeTests determines whether to include test files
	IncludeTests bool
	// IncludeInternal determines whether to include internal packages
	IncludeInternal bool
}

// NewExtractor creates a new API contract extractor
func NewExtractor() *Extractor {
	return &Extractor{
		IncludePrivate:  false,
		IncludeTests:    false,
		IncludeInternal: false,
	}
}

// ExtractFromPackage extracts the API contract from a Go package path
func (e *Extractor) ExtractFromPackage(packagePath string) (*Contract, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes |
			packages.NeedSyntax | packages.NeedTypesInfo,
	}

	pkgs, err := packages.Load(cfg, packagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load package %s: %w", packagePath, err)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoPackagesFound, packagePath)
	}

	if len(pkgs[0].Errors) > 0 {
		var errors []string
		for _, err := range pkgs[0].Errors {
			errors = append(errors, err.Error())
		}
		return nil, fmt.Errorf("%w: %s", ErrPackageErrors, strings.Join(errors, "; "))
	}

	return e.extractFromPackageInfo(pkgs[0])
}

// ExtractFromDirectory extracts the API contract from a directory containing Go files
func (e *Extractor) ExtractFromDirectory(dir string) (*Contract, error) {
	fset := token.NewFileSet()

	// Parse all Go files in the directory
	pkgs, err := parser.ParseDir(fset, dir, func(info os.FileInfo) bool {
		name := info.Name()
		if !strings.HasSuffix(name, ".go") {
			return false
		}
		if !e.IncludeTests && strings.HasSuffix(name, "_test.go") {
			return false
		}
		return true
	}, parser.ParseComments)

	if err != nil {
		return nil, fmt.Errorf("failed to parse directory %s: %w", dir, err)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("%w in %s", ErrNoGoPackagesFound, dir)
	}

	// Use the first non-main package, or main if that's all there is
	var pkg *ast.Package
	for name, p := range pkgs {
		if name != "main" {
			pkg = p
			break
		}
	}
	if pkg == nil {
		for _, p := range pkgs {
			pkg = p
			break
		}
	}

	return e.extractFromAST(pkg, fset)
}

// extractFromPackageInfo extracts contract from packages.Package
func (e *Extractor) extractFromPackageInfo(pkg *packages.Package) (*Contract, error) {
	contract := &Contract{
		PackageName: pkg.Name,
		ModulePath:  pkg.PkgPath,
		Timestamp:   time.Now(),
		Interfaces:  []InterfaceContract{},
		Types:       []TypeContract{},
		Functions:   []FunctionContract{},
		Variables:   []VariableContract{},
		Constants:   []ConstantContract{},
	}

	// Process all objects in the package scope
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)

		// Skip unexported items if not including private
		if !e.IncludePrivate && !obj.Exported() {
			continue
		}

		switch obj := obj.(type) {
		case *types.TypeName:
			e.extractType(contract, pkg, obj)
		case *types.Func:
			e.extractFunction(contract, pkg, obj)
		case *types.Var:
			e.extractVariable(contract, pkg, obj)
		case *types.Const:
			e.extractConstant(contract, pkg, obj)
		}
	}

	// Sort slices for consistent output
	e.sortContract(contract)

	return contract, nil
}

// extractFromAST extracts contract from AST (used for directory parsing)
func (e *Extractor) extractFromAST(pkg *ast.Package, fset *token.FileSet) (*Contract, error) {
	contract := &Contract{
		PackageName: pkg.Name,
		Timestamp:   time.Now(),
		Interfaces:  []InterfaceContract{},
		Types:       []TypeContract{},
		Functions:   []FunctionContract{},
		Variables:   []VariableContract{},
		Constants:   []ConstantContract{},
	}

	// Create a doc.Package to get documentation
	docPkg := doc.New(pkg, "", 0)

	// Extract types (including interfaces)
	for _, t := range docPkg.Types {
		if !e.IncludePrivate && !ast.IsExported(t.Name) {
			continue
		}

		typeContract := e.extractTypeFromDoc(t, fset)

		if e.isInterface(t) {
			// Convert to interface contract
			ifaceContract := InterfaceContract{
				Name:       typeContract.Name,
				Package:    typeContract.Package,
				DocComment: typeContract.DocComment,
				Methods:    typeContract.Methods,
				Position:   typeContract.Position,
			}
			contract.Interfaces = append(contract.Interfaces, ifaceContract)
		} else {
			contract.Types = append(contract.Types, typeContract)
		}
	}

	// Extract functions
	for _, f := range docPkg.Funcs {
		if !e.IncludePrivate && !ast.IsExported(f.Name) {
			continue
		}

		funcContract := e.extractFunctionFromDoc(f, fset)
		contract.Functions = append(contract.Functions, funcContract)
	}

	// Extract variables and constants
	for _, v := range docPkg.Vars {
		for _, name := range v.Names {
			if !e.IncludePrivate && !ast.IsExported(name) {
				continue
			}

			varContract := VariableContract{
				Name:       name,
				Package:    pkg.Name,
				DocComment: v.Doc,
				Position:   getPositionInfo(fset, v.Decl.Pos()),
			}
			contract.Variables = append(contract.Variables, varContract)
		}
	}

	for _, c := range docPkg.Consts {
		for _, name := range c.Names {
			if !e.IncludePrivate && !ast.IsExported(name) {
				continue
			}

			constContract := ConstantContract{
				Name:       name,
				Package:    pkg.Name,
				DocComment: c.Doc,
				Position:   getPositionInfo(fset, c.Decl.Pos()),
			}
			contract.Constants = append(contract.Constants, constContract)
		}
	}

	e.sortContract(contract)
	return contract, nil
}

// extractType extracts type information
func (e *Extractor) extractType(contract *Contract, pkg *packages.Package, obj *types.TypeName) {
	named, ok := obj.Type().(*types.Named)
	if !ok {
		return
	}

	typeContract := TypeContract{
		Name:       obj.Name(),
		Package:    pkg.Name,
		DocComment: e.getDocComment(pkg, obj.Pos()),
		Position:   getPositionInfo(pkg.Fset, obj.Pos()),
	}

	underlying := named.Underlying()

	// Check if it's an interface
	if iface, ok := underlying.(*types.Interface); ok {
		ifaceContract := InterfaceContract{
			Name:       typeContract.Name,
			Package:    typeContract.Package,
			DocComment: typeContract.DocComment,
			Position:   typeContract.Position,
			Methods:    []MethodContract{},
		}

		// Extract interface methods
		for i := 0; i < iface.NumMethods(); i++ {
			method := iface.Method(i)
			if !e.IncludePrivate && !method.Exported() {
				continue
			}

			methodContract := e.extractMethodSignature(method, pkg.Fset)
			ifaceContract.Methods = append(ifaceContract.Methods, methodContract)
		}

		contract.Interfaces = append(contract.Interfaces, ifaceContract)
		return
	}

	// Handle other types (struct, alias, etc.)
	switch underlying := underlying.(type) {
	case *types.Struct:
		typeContract.Kind = "struct"
		typeContract.Fields = e.extractStructFields(underlying, pkg.Fset)
	case *types.Basic:
		typeContract.Kind = "basic"
		typeContract.Underlying = underlying.String()
	default:
		typeContract.Kind = "alias"
		typeContract.Underlying = formatType(underlying)
	}

	// Extract methods for named types
	for i := 0; i < named.NumMethods(); i++ {
		method := named.Method(i)
		if !e.IncludePrivate && !method.Exported() {
			continue
		}

		methodContract := e.extractMethodSignature(method, pkg.Fset)
		typeContract.Methods = append(typeContract.Methods, methodContract)
	}

	contract.Types = append(contract.Types, typeContract)
}

// extractFunction extracts function information
func (e *Extractor) extractFunction(contract *Contract, pkg *packages.Package, obj *types.Func) {
	if obj.Type() == nil {
		return
	}

	sig, ok := obj.Type().(*types.Signature)
	if !ok {
		return
	}

	funcContract := FunctionContract{
		Name:       obj.Name(),
		Package:    pkg.Name,
		DocComment: e.getDocComment(pkg, obj.Pos()),
		Parameters: e.extractParameters(sig.Params()),
		Results:    e.extractParameters(sig.Results()),
		Position:   getPositionInfo(pkg.Fset, obj.Pos()),
	}

	contract.Functions = append(contract.Functions, funcContract)
}

// extractVariable extracts variable information
func (e *Extractor) extractVariable(contract *Contract, pkg *packages.Package, obj *types.Var) {
	varContract := VariableContract{
		Name:       obj.Name(),
		Package:    pkg.Name,
		Type:       formatType(obj.Type()),
		DocComment: e.getDocComment(pkg, obj.Pos()),
		Position:   getPositionInfo(pkg.Fset, obj.Pos()),
	}

	contract.Variables = append(contract.Variables, varContract)
}

// extractConstant extracts constant information
func (e *Extractor) extractConstant(contract *Contract, pkg *packages.Package, obj *types.Const) {
	constContract := ConstantContract{
		Name:       obj.Name(),
		Package:    pkg.Name,
		Type:       formatType(obj.Type()),
		Value:      obj.Val().String(),
		DocComment: e.getDocComment(pkg, obj.Pos()),
		Position:   getPositionInfo(pkg.Fset, obj.Pos()),
	}

	contract.Constants = append(contract.Constants, constContract)
}

// Helper methods for AST-based extraction

func (e *Extractor) extractTypeFromDoc(t *doc.Type, fset *token.FileSet) TypeContract {
	typeContract := TypeContract{
		Name:       t.Name,
		DocComment: t.Doc,
		Methods:    []MethodContract{},
		Fields:     []FieldContract{},
	}

	// Extract methods
	for _, method := range t.Methods {
		if !e.IncludePrivate && !ast.IsExported(method.Name) {
			continue
		}

		// Parse method signature completely
		methodContract := MethodContract{
			Name:       method.Name,
			DocComment: method.Doc,
			Position:   getPositionInfo(fset, method.Decl.Pos()),
		}

		// Extract full method signature including parameters and results
		if method.Decl != nil {
			funDecl := method.Decl
			if funDecl.Type != nil {
				// Extract receiver
				if funDecl.Recv != nil && len(funDecl.Recv.List) > 0 {
					recv := funDecl.Recv.List[0]
					methodContract.Receiver = e.extractReceiverInfo(recv)
				}

				// Extract parameters
				if funDecl.Type.Params != nil {
					methodContract.Parameters = e.extractParameterList(funDecl.Type.Params)
				}

				// Extract results
				if funDecl.Type.Results != nil {
					methodContract.Results = e.extractParameterList(funDecl.Type.Results)
				}
			}
		}
		typeContract.Methods = append(typeContract.Methods, methodContract)
	}

	return typeContract
}

func (e *Extractor) extractFunctionFromDoc(f *doc.Func, fset *token.FileSet) FunctionContract {
	funcContract := FunctionContract{
		Name:       f.Name,
		DocComment: f.Doc,
		Position:   getPositionInfo(fset, f.Decl.Pos()),
	}

	// Extract full function signature including parameters and results
	if f.Decl != nil {
		funDecl := f.Decl
		if funDecl.Type != nil {
			// Extract parameters
			if funDecl.Type.Params != nil {
				funcContract.Parameters = e.extractParameterList(funDecl.Type.Params)
			}

			// Extract results
			if funDecl.Type.Results != nil {
				funcContract.Results = e.extractParameterList(funDecl.Type.Results)
			}
		}
	}

	return funcContract
}

func (e *Extractor) isInterface(t *doc.Type) bool {
	// Simple check - in a full implementation, you'd parse the type spec
	return strings.Contains(t.Doc, "interface") ||
		(t.Decl != nil && e.isInterfaceDecl(t.Decl))
}

func (e *Extractor) isInterfaceDecl(decl *ast.GenDecl) bool {
	for _, spec := range decl.Specs {
		if typeSpec, ok := spec.(*ast.TypeSpec); ok {
			if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
				return true
			}
		}
	}
	return false
}

// Helper methods

func (e *Extractor) extractMethodSignature(method *types.Func, fset *token.FileSet) MethodContract {
	sig := method.Type().(*types.Signature)

	methodContract := MethodContract{
		Name:       method.Name(),
		Parameters: e.extractParameters(sig.Params()),
		Results:    e.extractParameters(sig.Results()),
		Position:   getPositionInfo(fset, method.Pos()),
	}

	// Extract receiver info
	if recv := sig.Recv(); recv != nil {
		methodContract.Receiver = &ReceiverInfo{
			Type: formatType(recv.Type()),
		}

		// Check if it's a pointer receiver
		if ptr, ok := recv.Type().(*types.Pointer); ok {
			methodContract.Receiver.Pointer = true
			methodContract.Receiver.Type = formatType(ptr.Elem())
		}
	}

	return methodContract
}

func (e *Extractor) extractParameters(tuple *types.Tuple) []ParameterInfo {
	if tuple == nil {
		return nil
	}

	params := make([]ParameterInfo, tuple.Len())
	for i := 0; i < tuple.Len(); i++ {
		param := tuple.At(i)
		params[i] = ParameterInfo{
			Name: param.Name(),
			Type: formatType(param.Type()),
		}
	}
	return params
}

func (e *Extractor) extractStructFields(structType *types.Struct, fset *token.FileSet) []FieldContract {
	fields := make([]FieldContract, structType.NumFields())

	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)

		if !e.IncludePrivate && !field.Exported() {
			continue
		}

		fieldContract := FieldContract{
			Name: field.Name(),
			Type: formatType(field.Type()),
		}

		if tag := structType.Tag(i); tag != "" {
			fieldContract.Tag = tag
		}

		fields[i] = fieldContract
	}

	return fields
}

func (e *Extractor) getDocComment(pkg *packages.Package, pos token.Pos) string {
	// Full implementation to map positions to AST nodes and extract comments
	for _, syntax := range pkg.Syntax {
		if syntax.Pos() <= pos && pos < syntax.End() {
			// Find the node at this position
			return e.extractCommentForPos(syntax, pkg.Fset, pos)
		}
	}
	return ""
}

func (e *Extractor) extractCommentForPos(file *ast.File, fset *token.FileSet, pos token.Pos) string {
	// Walk the AST to find the node at the given position
	var result string
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		
		// Check if this node contains our position
		if n.Pos() <= pos && pos < n.End() {
			switch node := n.(type) {
			case *ast.GenDecl:
				if node.Doc != nil {
					result = node.Doc.Text()
				}
			case *ast.FuncDecl:
				if node.Doc != nil {
					result = node.Doc.Text()
				}
			case *ast.TypeSpec:
				if node.Doc != nil {
					result = node.Doc.Text()
				}
			case *ast.Field:
				if node.Doc != nil {
					result = node.Doc.Text()
				}
			}
			return true
		}
		return true
	})
	
	return strings.TrimSpace(result)
}

func (e *Extractor) extractReceiverInfo(field *ast.Field) *ReceiverInfo {
	if field == nil || field.Type == nil {
		return nil
	}
	
	receiverInfo := &ReceiverInfo{}
	
	// Get receiver name if available
	if len(field.Names) > 0 {
		receiverInfo.Name = field.Names[0].Name
	}
	
	// Determine if it's a pointer and get the type
	switch t := field.Type.(type) {
	case *ast.StarExpr:
		receiverInfo.Pointer = true
		receiverInfo.Type = e.typeToString(t.X)
	default:
		receiverInfo.Pointer = false
		receiverInfo.Type = e.typeToString(t)
	}
	
	return receiverInfo
}

func (e *Extractor) extractParameterList(fieldList *ast.FieldList) []ParameterInfo {
	if fieldList == nil {
		return nil
	}
	
	var parameters []ParameterInfo
	
	for _, field := range fieldList.List {
		typeStr := e.typeToString(field.Type)
		
		if len(field.Names) > 0 {
			// Named parameters
			for _, name := range field.Names {
				parameters = append(parameters, ParameterInfo{
					Name: name.Name,
					Type: typeStr,
				})
			}
		} else {
			// Unnamed parameter (e.g., in function results)
			parameters = append(parameters, ParameterInfo{
				Type: typeStr,
			})
		}
	}
	
	return parameters
}

func (e *Extractor) typeToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return e.typeToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + e.typeToString(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + e.typeToString(t.Elt)
		}
		return "[" + e.exprToString(t.Len) + "]" + e.typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + e.typeToString(t.Key) + "]" + e.typeToString(t.Value)
	case *ast.ChanType:
		switch t.Dir {
		case ast.SEND:
			return "chan<- " + e.typeToString(t.Value)
		case ast.RECV:
			return "<-chan " + e.typeToString(t.Value)
		default:
			return "chan " + e.typeToString(t.Value)
		}
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{}"
	case *ast.FuncType:
		return e.funcTypeToString(t)
	case *ast.Ellipsis:
		return "..." + e.typeToString(t.Elt)
	default:
		// Fallback to basic string representation
		return fmt.Sprintf("%T", t)
	}
}

func (e *Extractor) exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Value
	case *ast.Ident:
		return e.Name
	default:
		return ""
	}
}

func (e *Extractor) funcTypeToString(ft *ast.FuncType) string {
	var parts []string
	parts = append(parts, "func")
	
	if ft.Params != nil {
		var params []string
		for _, field := range ft.Params.List {
			typeStr := e.typeToString(field.Type)
			if len(field.Names) > 0 {
				for range field.Names {
					params = append(params, typeStr)
				}
			} else {
				params = append(params, typeStr)
			}
		}
		parts = append(parts, "("+strings.Join(params, ", ")+")")
	} else {
		parts = append(parts, "()")
	}
	
	if ft.Results != nil && len(ft.Results.List) > 0 {
		var results []string
		for _, field := range ft.Results.List {
			typeStr := e.typeToString(field.Type)
			if len(field.Names) > 0 {
				for range field.Names {
					results = append(results, typeStr)
				}
			} else {
				results = append(results, typeStr)
			}
		}
		if len(results) == 1 {
			parts = append(parts, " "+results[0])
		} else {
			parts = append(parts, " ("+strings.Join(results, ", ")+")")
		}
	}
	
	return strings.Join(parts, "")
}

func (e *Extractor) sortContract(contract *Contract) {
	// Sort all slices for consistent output
	sort.Slice(contract.Interfaces, func(i, j int) bool {
		return contract.Interfaces[i].Name < contract.Interfaces[j].Name
	})

	sort.Slice(contract.Types, func(i, j int) bool {
		return contract.Types[i].Name < contract.Types[j].Name
	})

	sort.Slice(contract.Functions, func(i, j int) bool {
		return contract.Functions[i].Name < contract.Functions[j].Name
	})

	sort.Slice(contract.Variables, func(i, j int) bool {
		return contract.Variables[i].Name < contract.Variables[j].Name
	})

	sort.Slice(contract.Constants, func(i, j int) bool {
		return contract.Constants[i].Name < contract.Constants[j].Name
	})

	// Sort methods within interfaces and types
	for i := range contract.Interfaces {
		sort.Slice(contract.Interfaces[i].Methods, func(a, b int) bool {
			return contract.Interfaces[i].Methods[a].Name < contract.Interfaces[i].Methods[b].Name
		})
	}

	for i := range contract.Types {
		sort.Slice(contract.Types[i].Methods, func(a, b int) bool {
			return contract.Types[i].Methods[a].Name < contract.Types[i].Methods[b].Name
		})
		sort.Slice(contract.Types[i].Fields, func(a, b int) bool {
			return contract.Types[i].Fields[a].Name < contract.Types[i].Fields[b].Name
		})
	}
}

// SaveToFile saves the contract to a JSON file
func (c *Contract) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal contract: %w", err)
	}

	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write contract file: %w", err)
	}

	return nil
}

// LoadFromFile loads a contract from a JSON file
func LoadFromFile(filename string) (*Contract, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read contract file: %w", err)
	}

	var contract Contract
	if err := json.Unmarshal(data, &contract); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract: %w", err)
	}

	return &contract, nil
}
