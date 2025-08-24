# LogMasker Module

[![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/logmasker.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/logmasker)

The LogMasker Module provides centralized log masking functionality for Modular applications. It acts as a decorator around the standard Logger interface to automatically redact sensitive information from log output based on configurable rules.

## Features

- **Logger Decorator**: Wraps any `modular.Logger` implementation with masking capabilities
- **Field-Based Masking**: Define rules for specific field names (e.g., "password", "token")  
- **Pattern-Based Masking**: Use regex patterns to detect sensitive data (e.g., credit cards, SSNs)
- **MaskableValue Interface**: Allow values to control their own masking behavior
- **Multiple Masking Strategies**: Redact, partial mask, hash, or leave unchanged
- **Configurable Rules**: Full YAML/JSON configuration support
- **Performance Optimized**: Minimal overhead for production use
- **Framework Integration**: Seamless integration with the Modular framework

## Installation

Add the logmasker module to your project:

```bash
go get github.com/GoCodeAlone/modular/modules/logmasker
```

## Configuration

The logmasker module can be configured using the following options:

```yaml
logmasker:
  enabled: true                    # Enable/disable log masking
  defaultMaskStrategy: redact      # Default strategy: redact, partial, hash, none
  
  fieldRules:                      # Field-based masking rules
    - fieldName: password
      strategy: redact
    - fieldName: email  
      strategy: partial
      partialConfig:
        showFirst: 2
        showLast: 2
        maskChar: "*"
        minLength: 4
    - fieldName: token
      strategy: redact
    - fieldName: secret
      strategy: redact
    - fieldName: key
      strategy: redact
  
  patternRules:                    # Pattern-based masking rules
    - pattern: '\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b'  # Credit cards
      strategy: redact
    - pattern: '\b\d{3}-\d{2}-\d{4}\b'  # SSN format
      strategy: redact
  
  defaultPartialConfig:            # Default partial masking settings
    showFirst: 2
    showLast: 2  
    maskChar: "*"
    minLength: 4
```

## Usage

### Basic Usage

Register the module and use the masking logger service:

```go
package main

import (
    "github.com/GoCodeAlone/modular"
    "github.com/GoCodeAlone/modular/modules/logmasker"
)

func main() {
    // Create application with your config and logger
    app := modular.NewApplication(configProvider, logger)
    
    // Register the logmasker module
    app.RegisterModule(logmasker.NewModule())
    
    // Initialize the application
    if err := app.Init(); err != nil {
        log.Fatal(err)
    }
    
    // Get the masking logger service
    var maskingLogger modular.Logger
    err := app.GetService("logmasker.logger", &maskingLogger)
    if err != nil {
        log.Fatal(err)
    }
    
    // Use the masking logger - sensitive data will be automatically masked
    maskingLogger.Info("User login", 
        "email", "user@example.com",     // Will be partially masked
        "password", "secret123",          // Will be redacted
        "sessionId", "abc-123-def")       // Will remain unchanged
    
    // Output: "User login" email="us*****.com" password="[REDACTED]" sessionId="abc-123-def"
}
```

### MaskableValue Interface

Create values that control their own masking behavior:

```go
// SensitiveToken implements MaskableValue
type SensitiveToken struct {
    Value string
    IsPublic bool
}

func (t *SensitiveToken) ShouldMask() bool {
    return !t.IsPublic
}

func (t *SensitiveToken) GetMaskedValue() any {
    return "[SENSITIVE-TOKEN]"
}

func (t *SensitiveToken) GetMaskStrategy() logmasker.MaskStrategy {
    return logmasker.MaskStrategyRedact
}

// Usage
token := &SensitiveToken{Value: "secret-token", IsPublic: false}
maskingLogger.Info("API call", "token", token)
// Output: "API call" token="[SENSITIVE-TOKEN]"
```

### Custom Configuration

Override default masking behavior:

```go
// Custom configuration in your config file
config := &logmasker.LogMaskerConfig{
    Enabled: true,
    DefaultMaskStrategy: logmasker.MaskStrategyPartial,
    FieldRules: []logmasker.FieldMaskingRule{
        {
            FieldName: "creditCard",
            Strategy:  logmasker.MaskStrategyHash,
        },
        {
            FieldName: "phone", 
            Strategy:  logmasker.MaskStrategyPartial,
            PartialConfig: &logmasker.PartialMaskConfig{
                ShowFirst: 3,
                ShowLast:  4,
                MaskChar:  "#",
                MinLength: 10,
            },
        },
    },
    PatternRules: []logmasker.PatternMaskingRule{
        {
            Pattern:  `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // Email regex
            Strategy: logmasker.MaskStrategyPartial,
            PartialConfig: &logmasker.PartialMaskConfig{
                ShowFirst: 2,
                ShowLast:  8, // Show domain
                MaskChar:  "*",
                MinLength: 6,
            },
        },
    },
}
```

### Integration with Other Modules

The logmasker works seamlessly with other modules:

```go
// In a module that needs masked logging
type MyModule struct {
    logger modular.Logger
}

func (m *MyModule) Init(app modular.Application) error {
    // Get the masking logger instead of the original logger
    return app.GetService("logmasker.logger", &m.logger)
}

func (m *MyModule) ProcessUser(user *User) {
    // All sensitive data will be automatically masked
    m.logger.Info("Processing user",
        "id", user.ID,
        "email", user.Email,           // Masked based on field rules
        "password", user.Password,     // Redacted
        "profile", user.Profile)       // Unchanged
}
```

## Masking Strategies

### Redact Strategy
Replaces the entire value with `[REDACTED]`:
```
password: "secret123" → "[REDACTED]"
```

### Partial Strategy  
Shows only specified characters, masking the rest:
```
email: "user@example.com" → "us**********com" (showFirst: 2, showLast: 3)
phone: "555-123-4567" → "555-***-4567" (showFirst: 3, showLast: 4)
```

### Hash Strategy
Replaces value with a hash:
```
token: "abc123" → "[HASH:2c26b46b]"
```

### None Strategy
Leaves the value unchanged (useful for overriding default behavior):
```
publicId: "user-123" → "user-123"
```

## Field Rules vs Pattern Rules

- **Field Rules**: Match exact field names in key-value logging pairs
- **Pattern Rules**: Match regex patterns in string values regardless of field name

Field rules take precedence over pattern rules for the same value.

## Performance Considerations

- **Lazy Compilation**: Regex patterns are compiled once during module initialization
- **Early Exit**: When masking is disabled, no processing overhead occurs
- **Efficient Matching**: Field rules use map lookup, pattern matching is optimized
- **Memory Efficient**: No unnecessary string copies for unmasked values

## Error Handling

The module handles various error conditions gracefully:

- **Invalid Regex Patterns**: Module initialization fails with descriptive error
- **Missing Logger Service**: Module initialization fails if logger service unavailable  
- **Configuration Errors**: Reported during module initialization
- **Runtime Errors**: Malformed log calls are passed through unchanged

## Security Considerations

When using log masking in production:

- **Review Field Rules**: Ensure all sensitive field names are covered
- **Test Pattern Rules**: Validate regex patterns match expected sensitive data
- **Audit Log Output**: Regularly review logs to ensure masking is working
- **Performance Impact**: Monitor performance in high-throughput scenarios
- **Configuration Security**: Ensure masking configuration itself doesn't contain secrets

## Testing

Run the module tests:

```bash
cd modules/logmasker
go test ./... -v
```

The module includes comprehensive tests covering:
- Field-based masking rules
- Pattern-based masking rules  
- MaskableValue interface behavior
- All masking strategies
- Partial masking configuration
- Module lifecycle and integration
- Performance edge cases

## Implementation Notes

- The module wraps the original logger using the decorator pattern
- MaskableValue interface allows for anytype-compatible value wrappers
- Configuration supports full validation with default values
- Regex patterns are pre-compiled for performance
- The module integrates seamlessly with the framework's service system

## Integration with Existing Logging

The logmasker module is designed to be a drop-in replacement for the standard logger:

```go
// Before: Using standard logger
var logger modular.Logger
app.GetService("logger", &logger)

// After: Using masking logger  
var maskingLogger modular.Logger
app.GetService("logmasker.logger", &maskingLogger)

// Same interface, automatic masking
maskingLogger.Info("message", "key", "value")
```

This allows existing code to benefit from masking without modifications.