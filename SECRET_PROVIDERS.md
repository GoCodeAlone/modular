# Secret Providers for Modular Framework

This document describes the provider-based secret handling system implemented in the Modular framework.

## Overview

The Modular framework now supports a pluggable provider system for handling secrets, allowing for different security levels and memory handling approaches based on your requirements.

## Architecture

### Core Components

1. **SecretProvider Interface** - Defines the contract for secret storage backends
2. **SecretValue** - Enhanced to work with providers while maintaining backward compatibility
3. **SecretProviderFactory** - Creates and configures providers based on configuration
4. **SecretHandle** - Opaque reference to stored secrets

### Available Providers

#### Insecure Provider (`insecure`)
- **Security Level**: Basic obfuscation only
- **Memory Handling**: XOR encryption with limited memory protection
- **Best For**: Development, testing, non-critical secrets
- **Performance**: High (minimal overhead)
- **Availability**: Always available

**Limitations:**
- Cannot prevent memory dumps from revealing secrets
- Cannot guarantee secure memory clearing in Go
- Should NOT be used for highly sensitive secrets in production

#### Memguard Provider (`memguard`)
- **Security Level**: Cryptographically secure
- **Memory Handling**: Hardware-backed secure memory allocation
- **Best For**: Production systems with sensitive secrets
- **Performance**: Lower (security overhead)
- **Availability**: Requires CGO and platform support

**Features:**
- Secure memory allocation not swapped to disk
- Memory encryption to protect against memory dumps  
- Secure memory wiping when secrets are destroyed
- Protection against Heartbleed-style attacks
- Memory canaries to detect buffer overflows

**Requirements:**
- `github.com/awnumar/memguard` dependency
- CGO enabled (`CGO_ENABLED=1`)
- Platform support for secure memory

## Configuration

### Basic Configuration

```yaml
secret_provider:
  provider: "insecure"                # Provider to use (insecure, memguard)
  enable_secure_memory: false        # Require secure memory handling
  warn_on_insecure: true            # Warn when using insecure providers
  max_secrets: 1000                 # Maximum secrets to store (0 = unlimited)
  auto_destroy: "0s"                # Auto-destroy duration (0 = never)
```

### Environment Variables

```bash
SECRET_PROVIDER=memguard
ENABLE_SECURE_MEMORY=true
WARN_ON_INSECURE=true
MAX_SECRETS=500
AUTO_DESTROY=1h
```

### Production Configuration

```yaml
secret_provider:
  provider: "memguard"
  enable_secure_memory: true
  warn_on_insecure: true
  max_secrets: 1000
  auto_destroy: "24h"
```

## Usage

### Initialization

```go
// Initialize the secret provider system
config := SecretProviderConfig{
    Provider:           "memguard",
    EnableSecureMemory: true,
    MaxSecrets:         1000,
}

err := InitializeSecretProvider(config, logger)
if err != nil {
    log.Fatal("Failed to initialize secret provider:", err)
}
```

### Creating Secrets

```go
// Create secrets - automatically uses configured provider
password := NewPasswordSecret("super-secret-password")
apiKey := NewTokenSecret("api-key-12345")
certificate := NewCertificateSecret("cert-pem-data")

// Or specify type explicitly
secret := NewSecretValue("sensitive-data", SecretTypeGeneral)
```

### Using Secrets

```go
// Retrieve secret value (only when needed)
value := secret.Reveal()
defer func() {
    // Clean up revealed value
    for i := range value {
        value[i] = 0
    }
}()

// Secure comparison (constant-time)
if secret.EqualsString("expected-value") {
    // Handle match
}

// Clone secrets
clonedSecret := secret.Clone()

// Destroy when done
secret.Destroy()
clonedSecret.Destroy()
```

### Working with Specific Providers

```go
// Get global provider
provider := GetGlobalSecretProvider()
fmt.Printf("Using provider: %s (secure: %v)\n", 
    provider.Name(), provider.IsSecure())

// Create with specific provider
config := SecretProviderConfig{Provider: "insecure"}
factory := NewSecretProviderFactory(logger)
specificProvider, err := factory.CreateProvider(config)
if err != nil {
    return err
}

secret := NewSecretValueWithProvider("data", SecretTypeGeneral, specificProvider)
```

## Integration with Logmasker

The SecretValue type now implements a secret interface pattern that allows the logmasker module to automatically detect and mask secrets in logs without explicit coupling.

### Interface Pattern

SecretValue implements these methods:
- `ShouldMask() bool` - Returns true to indicate masking needed
- `GetMaskedValue() any` - Returns masked representation
- `GetMaskStrategy() string` - Returns masking strategy preference

### Automatic Detection

```go
// Logmasker automatically detects and masks SecretValue instances
password := NewPasswordSecret("secret123")
logger.Info("User login", "password", password)
// Output: User login password=[PASSWORD]

token := NewTokenSecret("abc123")  
logger.Info("API call", "token", token)
// Output: API call token=[TOKEN]
```

### Custom Secret Types

You can create custom types that work with logmasker:

```go
type CustomSecret struct {
    value string
    sensitive bool
}

func (c *CustomSecret) ShouldMask() bool {
    return c.sensitive
}

func (c *CustomSecret) GetMaskedValue() any {
    if c.sensitive {
        return "[CUSTOM_SECRET]"
    }
    return c.value
}

func (c *CustomSecret) GetMaskStrategy() string {
    return "redact"
}
```

## Migration Guide

### From Legacy SecretValue

The new provider system is fully backward compatible:

```go
// Old way - still works
secret := NewSecretValue("data", SecretTypePassword)

// New way - uses configured provider
secret := NewPasswordSecret("data")

// Both work identically for existing operations
value := secret.Reveal()
isEqual := secret.EqualsString("test")
```

### Upgrading to Secure Providers

1. **Update Configuration**
   ```yaml
   secret_provider:
     provider: "memguard"
     enable_secure_memory: true
   ```

2. **Add Dependencies** (if using memguard)
   ```bash
   go get github.com/awnumar/memguard
   ```

3. **Enable CGO** (if using memguard)
   ```bash
   export CGO_ENABLED=1
   ```

4. **Test Thoroughly**
   - Verify provider availability in your environment
   - Test with actual secret data
   - Monitor performance impact

## Best Practices

### Security

1. **Use Secure Providers in Production**
   ```go
   config := SecretProviderConfig{
       Provider: "memguard",
       EnableSecureMemory: true,
   }
   ```

2. **Limit Secret Lifetime**
   ```go
   config := SecretProviderConfig{
       AutoDestroy: time.Hour * 24,
   }
   ```

3. **Minimize Secret Revelation**
   ```go
   // Avoid
   password := secret.Reveal()
   processPassword(password)
   
   // Prefer
   if secret.EqualsString(expectedPassword) {
       // Handle without revealing
   }
   ```

### Performance

1. **Consider Provider Overhead**
   - Insecure provider: ~5-10ns per operation
   - Memguard provider: ~50-100ns per operation

2. **Batch Operations**
   ```go
   // Create multiple secrets at once when possible
   secrets := make([]*SecretValue, 100)
   for i := range secrets {
       secrets[i] = NewPasswordSecret(generatePassword())
   }
   ```

3. **Set Reasonable Limits**
   ```go
   config := SecretProviderConfig{
       MaxSecrets: 1000, // Prevent memory exhaustion
   }
   ```

### Monitoring

1. **Provider Status**
   ```go
   provider := GetGlobalSecretProvider()
   if !provider.IsSecure() {
       logger.Warn("Using insecure secret provider", 
           "provider", provider.Name())
   }
   ```

2. **Secret Metrics**
   ```go
   // Get provider-specific statistics
   if stats := GetInsecureProviderStats(provider); stats != nil {
       logger.Info("Provider stats", "stats", stats)
   }
   ```

## Troubleshooting

### Memguard Provider Unavailable

```
Error: failed to initialize memguard: memguard library is not available
```

**Solutions:**
1. Install memguard: `go get github.com/awnumar/memguard`
2. Enable CGO: `export CGO_ENABLED=1`
3. Check platform support
4. Fall back to insecure provider for development

### Memory Exhaustion

```
Error: maximum number of secrets reached: 1000
```

**Solutions:**
1. Increase `max_secrets` limit
2. Implement secret cleanup
3. Use `auto_destroy` for temporary secrets
4. Call `Destroy()` on unused secrets

### Performance Issues

**Symptoms:**
- Slow secret operations
- High memory usage
- CPU spikes during secret creation

**Solutions:**
1. Profile your application
2. Consider using insecure provider for non-critical secrets
3. Implement secret pooling
4. Reduce secret lifetime with `auto_destroy`

## Testing

The provider system includes comprehensive tests:

```bash
# Test all providers
go test -v -run TestSecretProviders

# Test logmasker integration  
go test -v -run TestLogmaskerSecretDetection

# Test provider factory
go test -v -run TestSecretProviderFactory
```

For custom providers, implement the test suite:

```go
func TestCustomProvider(t *testing.T) {
    provider := &CustomSecretProvider{}
    
    // Run standard test suite
    testProviderBasicOperations(t, provider)
    testProviderSecretTypes(t, provider)
    // ... other tests
}
```

## Extending the System

### Custom Providers

Implement the `SecretProvider` interface:

```go
type CustomProvider struct {
    // Implementation fields
}

func (p *CustomProvider) Name() string { return "custom" }
func (p *CustomProvider) IsSecure() bool { return true }
func (p *CustomProvider) Store(value string, secretType SecretType) (SecretHandle, error) {
    // Custom implementation
}
// ... implement all interface methods
```

Register with the factory:

```go
factory := NewSecretProviderFactory(logger)
factory.RegisterProvider("custom", func(config SecretProviderConfig) (SecretProvider, error) {
    return NewCustomProvider(config)
})
```

### Custom Secret Types

Add new secret types:

```go
const (
    SecretTypeDatabase SecretType = iota + 100
    SecretTypeLicense
)

func NewDatabaseSecret(connectionString string) *SecretValue {
    return NewSecretValue(connectionString, SecretTypeDatabase)
}
```

This completes the comprehensive provider-based secret handling system for the Modular framework.