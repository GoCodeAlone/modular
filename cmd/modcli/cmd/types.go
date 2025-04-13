package cmd

// ConfigOptions contains options for config generation
type ConfigOptions struct {
	Name           string        // Name of the config struct
	TagTypes       []string      // Types of tags to include (yaml, json, etc.)
	GenerateSample bool          // Whether to generate sample configs
	Fields         []ConfigField // Fields in the config
}

// ConfigField represents a field in the config struct
type ConfigField struct {
	Name         string        // Field name
	Type         string        // Field type
	IsRequired   bool          // Whether field is required
	DefaultValue string        // Default value for field
	Description  string        // Field description
	IsNested     bool          // Whether this is a nested struct
	IsArray      bool          // Whether this is an array type
	IsMap        bool          // Whether this is a map type
	KeyType      string        // Type of map keys (when IsMap is true)
	ValueType    string        // Type of map values (when IsMap is true)
	NestedFields []ConfigField // Fields of nested struct
	Tags         []string      // Field tags
}
