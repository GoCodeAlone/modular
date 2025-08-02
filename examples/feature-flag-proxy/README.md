# Feature Flag Proxy Example

This example demonstrates how to use feature flags to control routing behavior in the reverse proxy module, including tenant-specific configuration loading and feature flag overrides.

## Overview

The example sets up:
- A reverse proxy with feature flag-controlled backends
- Multiple backend servers to demonstrate different routing scenarios
- Tenant-aware feature flags with configuration file loading
- Composite routes with feature flag controls
- File-based tenant configuration system

## Tenant Configuration

This example demonstrates how to load tenant-specific configurations from files:

### Tenant Configuration Files

- `tenants/beta-tenant.yaml`: Configuration for beta tenant with premium features
- `tenants/enterprise-tenant.yaml`: Configuration for enterprise tenant with analytics

### How Tenant Config Loading Works

1. **Configuration Directory**: Tenant configs are stored in the `tenants/` directory
2. **File Naming**: Each tenant has a separate YAML file named `{tenant-id}.yaml`
3. **Automatic Loading**: The `FileBasedTenantConfigLoader` automatically discovers and loads tenant configurations
4. **Module Overrides**: Tenant files can override any module configuration, including reverseproxy settings
5. **Feature Flag Integration**: Tenant configs work seamlessly with feature flag evaluations

### Example Tenant Configuration Structure

```yaml
# tenants/beta-tenant.yaml
reverseproxy:
  default_backend: "beta-backend"
  backend_services:
    beta-backend: "http://localhost:9005"
    premium-api: "http://localhost:9006"
  backend_configs:
    default:
      feature_flag_id: "beta-feature" 
      alternative_backend: "beta-backend"
  routes:
    "/api/premium": "premium-api"
```

## Feature Flags Configured

1. **`beta-feature`** (globally disabled, enabled for "beta-tenant"):
   - Controls access to the default backend
   - Falls back to alternative backend when disabled

2. **`new-backend`** (globally enabled):
   - Controls access to the new-feature backend  
   - Falls back to default backend when disabled

3. **`composite-route`** (globally enabled):
   - Controls access to the composite route that combines multiple backends
   - Falls back to default backend when disabled

4. **`premium-features`** (globally disabled, enabled for "beta-tenant"):
   - Controls access to premium API features
   - Falls back to beta backend when disabled

5. **`enterprise-analytics`** (globally disabled, enabled for "enterprise-tenant"):
   - Controls access to enterprise analytics features
   - Falls back to enterprise backend when disabled

6. **`tenant-composite-route`** (globally enabled):
   - Controls tenant-specific composite routes
   - Falls back to tenant default backend when disabled

7. **`enterprise-dashboard`** (globally enabled):
   - Controls enterprise dashboard composite route
   - Falls back to enterprise backend when disabled

## Backend Services

- **Default Backend** (port 9001): Main backend service
- **Alternative Backend** (port 9002): Fallback when feature flags are disabled
- **New Feature Backend** (port 9003): New service controlled by feature flag
- **API Backend** (port 9004): Used in composite routes
- **Beta Backend** (port 9005): Special backend for beta tenant
- **Premium API Backend** (port 9006): Premium features for beta tenant  
- **Enterprise Backend** (port 9007): Enterprise tenant backend
- **Analytics API Backend** (port 9008): Enterprise analytics backend

## Running the Example

1. Start the application:
   ```bash
   go run main.go
   ```

2. The application will start on port 8080 with backends on ports 9001-9008

## Testing Feature Flags

### Test beta-feature flag (globally disabled)

```bash
# Normal user - should get alternative backend (feature disabled)
curl http://localhost:8080/api/beta

# Beta tenant - should get default backend (feature enabled for this tenant)
curl -H "X-Tenant-ID: beta-tenant" http://localhost:8080/api/beta
```

### Test new-backend flag (globally enabled)

```bash
# Should get new-feature backend (feature enabled)
curl http://localhost:8080/api/new
```

### Test composite route flag

```bash
# Should get composite response from multiple backends (feature enabled)
curl http://localhost:8080/api/composite
```

### Test tenant-specific routing and config loading

```bash
# Beta tenant gets routed to their specific backend via tenant config
curl -H "X-Tenant-ID: beta-tenant" http://localhost:8080/

# Beta tenant can access premium features (enabled via tenant config)
curl -H "X-Tenant-ID: beta-tenant" http://localhost:8080/api/premium

# Beta tenant composite route with tenant-specific backends
curl -H "X-Tenant-ID: beta-tenant" http://localhost:8080/api/tenant-composite

# Enterprise tenant gets routed to enterprise backend via tenant config
curl -H "X-Tenant-ID: enterprise-tenant" http://localhost:8080/

# Enterprise tenant can access analytics (enabled via tenant config)
curl -H "X-Tenant-ID: enterprise-tenant" http://localhost:8080/api/analytics

# Enterprise tenant dashboard with multiple data sources
curl -H "X-Tenant-ID: enterprise-tenant" http://localhost:8080/api/dashboard
```

## Configuration

The feature flags are configured in code in this example, but in a real application they would typically be:
- Loaded from a configuration file
- Retrieved from a feature flag service (LaunchDarkly, Split.io, etc.)
- Stored in a database

### Tenant Configuration Loading

This example demonstrates the file-based tenant configuration system:

1. **Tenant Discovery**: The `FileBasedTenantConfigLoader` scans the `tenants/` directory for YAML files
2. **Automatic Loading**: Each `{tenant-id}.yaml` file is automatically loaded as tenant configuration  
3. **Module Overrides**: Tenant files can override any module configuration
4. **Environment Variables**: Tenant-specific environment variables are supported with prefixes like `beta-tenant_REVERSEPROXY_PORT`
5. **Feature Flag Integration**: Tenant configurations work seamlessly with feature flag evaluations

### Configuration Precedence

1. **Global Configuration**: `config.yaml` provides default settings
2. **Tenant Configuration**: `tenants/{tenant-id}.yaml` overrides global settings for specific tenants
3. **Environment Variables**: Environment variables override file-based configuration
4. **Feature Flags**: Feature flag evaluations control runtime behavior

## Expected Responses

Each backend returns JSON with information about which backend served the request, making it easy to verify feature flag behavior:

```json
{
  "backend": "alternative",
  "path": "/api/beta", 
  "method": "GET",
  "feature": "fallback"
}
```

## Architecture

The feature flag system works by:
1. Registering a `FeatureFlagEvaluator` service with the application
2. Configuring feature flag IDs in backend and route configurations
3. The reverse proxy evaluates feature flags on each request
4. Routes are dynamically switched based on feature flag values
5. Tenant-specific overrides are supported for multi-tenant scenarios

This allows for:
- A/B testing new backends
- Gradual rollouts of new features
- Tenant-specific feature access
- Fallback behavior when features are disabled