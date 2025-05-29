# Multi-Tenant Application Example

This example demonstrates how to build a multi-tenant application using the Modular framework. It showcases both standard modules and tenant-aware modules that can have different configurations per tenant.

## Overview

The application includes:

- **Standard Modules**: `webserver`, `router`, `api` - These modules operate globally across all tenants
- **Tenant-Aware Modules**: `content`, `notifications` - These modules can have different configurations per tenant

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Application                              │
├─────────────────────────────────────────────────────────────┤
│  Standard Modules (Global)                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  WebServer  │  │   Router    │  │     API     │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
├─────────────────────────────────────────────────────────────┤
│  Tenant-Aware Modules                                       │
│  ┌─────────────┐  ┌─────────────┐                          │
│  │   Content   │  │Notifications│                          │
│  │   Manager   │  │   Manager   │                          │
│  └─────────────┘  └─────────────┘                          │
├─────────────────────────────────────────────────────────────┤
│  Tenant Service & Configuration                             │
│  ┌─────────────────────────────────────────────────────────┤
│  │ Tenants:                                                │
│  │ ├── quiksilver.yaml (Custom content & notifications)    │
│  │ └── roxy.yaml (Custom content & notifications)          │
│  └─────────────────────────────────────────────────────────┘
└─────────────────────────────────────────────────────────────┘
```

## Configuration

### Global Configuration (`config.yaml`)

The main configuration file defines default settings for all modules:

```yaml
appName: Multi-Tenant Example Application
environment: development

webserver:
  port: 8080

content:
  defaultTemplate: standard
  cacheTTL: 300

notifications:
  provider: smtp
  fromAddress: noreply@example.com
  maxRetries: 3
```

### Tenant-Specific Configuration

Each tenant can override default settings with their own configuration files in the `tenants/` directory.

#### QuikSilver Tenant (`tenants/quiksilver.yaml`)
```yaml
content:
  defaultTemplate: quiksilver-branded
  cacheTTL: 600

notifications:
  provider: aws-sns
  fromAddress: support@quiksilver.example.com
  maxRetries: 5
```

#### Roxy Tenant (`tenants/roxy.yaml`)
```yaml
content:
  defaultTemplate: roxy-premium
  cacheTTL: 900

notifications:
  provider: twilio
  fromAddress: hello@roxy.example.com
  maxRetries: 2
```

## Modules

### Standard Modules (Non-Tenant-Aware)

1. **WebServer**: Handles HTTP server functionality on port 8080
2. **Router**: Manages routing logic (depends on WebServer)
3. **API**: Provides API endpoints (depends on Router)

### Tenant-Aware Modules

1. **ContentManager**: 
   - Manages content templates and caching per tenant
   - Uses default configuration but can be overridden per tenant
   - Implements `TenantAware` interface

2. **NotificationManager**:
   - Handles notification providers and settings per tenant
   - Different tenants can use different notification providers (SMTP, AWS SNS, Twilio)
   - Implements `TenantAware` interface

## Key Features Demonstrated

### 1. Tenant Service Integration
```go
// Register tenant service
tenantService := modular.NewStandardTenantService(app.Logger())
app.RegisterService("tenantService", tenantService)
```

### 2. File-Based Tenant Configuration
```go
// Register tenant config loader
tenantConfigLoader := modular.NewFileBasedTenantConfigLoader(modular.TenantConfigParams{
    ConfigNameRegex: regexp.MustCompile("^\\w+\\.yaml$"),
    ConfigDir:       "tenants",
    ConfigFeeders: []modular.Feeder{
        feeders.NewTenantAffixedEnvFeeder(func(tenantId string) string {
            return fmt.Sprintf("%s_", tenantId)
        }, func(s string) string { return "" }),
    },
})
```

### 3. Tenant-Aware Module Implementation
```go
func (cm *ContentManager) OnTenantRegistered(tenantID modular.TenantID) {
    cm.logger.Info("Tenant registered in Content Manager", "tenant", tenantID)
}

func (cm *ContentManager) OnTenantRemoved(tenantID modular.TenantID) {
    cm.logger.Info("Tenant removed from Content Manager", "tenant", tenantID)
}
```

## Running the Application

1. **Install dependencies**:
   ```bash
   go mod tidy
   ```

2. **Run the application**:
   ```bash
   go run .
   ```

## Testing the Application

### What This Example Demonstrates

This is a **conceptual demonstration** of the modular framework's multi-tenant architecture. It showcases:

- **Module System**: How different types of modules (standard and tenant-aware) are structured
- **Tenant Configuration**: Automatic loading of tenant-specific configurations from YAML files
- **Lifecycle Management**: Complete application startup, module initialization, and tenant registration
- **Configuration Overrides**: How tenant settings override global defaults

**Note**: The WebServer module in this example logs that it starts on port 8080, but it's a simplified demonstration module that doesn't implement actual HTTP handlers. The focus is on the modular architecture and tenant management system.

### 3. **Expected output**:
   ```
   level=INFO msg="WebServer initialized" port=8080
   level=INFO msg="Router initialized"
   level=INFO msg="API module initialized"
   level=INFO msg="Content manager initialized with default template" template=standard cacheTTL=300
   level=INFO msg="Notification manager initialized" provider=smtp fromAddress=noreply@example.com
   level=INFO msg="Loading tenant config" tenant=quiksilver file=tenants/quiksilver.yaml
   level=INFO msg="Loading tenant config" tenant=roxy file=tenants/roxy.yaml
   level=INFO msg="Tenant registered in Content Manager" tenant=quiksilver
   level=INFO msg="Tenant registered in Notification Manager" tenant=quiksilver
   level=INFO msg="Tenant registered in Content Manager" tenant=roxy
   level=INFO msg="Tenant registered in Notification Manager" tenant=roxy
   level=INFO msg="WebServer started" port=8080
   ```

## Environment Variables

You can override tenant-specific settings using environment variables with the tenant prefix:

```bash
# Override QuikSilver's notification provider
export quiksilver_notifications_provider=sendgrid

# Override Roxy's cache TTL
export roxy_content_cacheTTL=1200

go run .
```

## Adding New Tenants

1. Create a new YAML file in the `tenants/` directory (e.g., `tenants/newclient.yaml`)
2. Define tenant-specific overrides:
   ```yaml
   content:
     defaultTemplate: newclient-custom
     cacheTTL: 450
   
   notifications:
     provider: mailgun
     fromAddress: support@newclient.com
     maxRetries: 4
   ```
3. Restart the application - the new tenant will be automatically loaded

## Use Cases

This pattern is ideal for:

- **SaaS Applications**: Different customers need different configurations
- **White-Label Solutions**: Each brand needs custom templates and settings
- **Multi-Environment Deployments**: Different environments with tenant-specific configs
- **Feature Toggles**: Enable/disable features per tenant
- **Regional Compliance**: Different regions with specific requirements

## Key Benefits

1. **Clean Separation**: Global vs tenant-specific configurations
2. **Runtime Flexibility**: Add/remove tenants without code changes
3. **Environment Overrides**: Support for environment-specific settings
4. **Type Safety**: Strongly typed configuration structures
5. **Dependency Management**: Proper module dependency resolution
6. **Lifecycle Management**: Automatic tenant registration/removal notifications
