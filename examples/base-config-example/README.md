# Base Configuration Example

This example demonstrates the base configuration support in the Modular framework, allowing you to manage configuration across multiple environments efficiently.

## Directory Structure

The example uses the following configuration structure:

```
config/
├── base/
│   └── default.yaml              # Baseline config shared across all environments
└── environments/
    ├── prod/
    │   └── overrides.yaml        # Production-specific overrides
    ├── staging/
    │   └── overrides.yaml        # Staging-specific overrides
    └── dev/
        └── overrides.yaml        # Development-specific overrides
```

## How It Works

1. **Base Configuration**: The `config/base/default.yaml` file contains shared configuration that applies to all environments
2. **Environment Overrides**: Each environment directory contains `overrides.yaml` files that override specific values from the base configuration
3. **Deep Merging**: The framework performs deep merging, so you only need to specify the values that change per environment

## Running the Example

### Prerequisites

```bash
cd examples/base-config-example
go mod tidy
```

### Run with Different Environments

```bash
# Run with development environment (default)
go run main.go dev

# Run with staging environment  
go run main.go staging

# Run with production environment
go run main.go prod
```

### Using Environment Variables

You can also set the environment using environment variables:

```bash
# Using APP_ENVIRONMENT
APP_ENVIRONMENT=prod go run main.go

# Using ENVIRONMENT  
ENVIRONMENT=staging go run main.go

# Using ENV
ENV=dev go run main.go
```

## Configuration Differences by Environment

### Development (dev)
- Simple database password
- Debug features enabled
- Caching disabled for easier debugging
- Basic server configuration

### Staging (staging)  
- Staging database host
- Metrics enabled for testing
- Redis enabled 
- Medium server load capacity

### Production (prod)
- Production database with secure password
- Metrics enabled, debug disabled
- All external services enabled (Redis, RabbitMQ)
- High server capacity and HTTPS port

## Key Benefits

1. **DRY Principle**: Common configuration is defined once in base config
2. **Environment Specific**: Only differences need to be specified per environment
3. **Easy Maintenance**: Adding new environments only requires creating override files
4. **Version Control Friendly**: Clear separation between base and environment-specific configs
5. **Deep Merging**: Nested objects are merged intelligently

## Example Output

When you run the example, you'll see the final merged configuration showing how base values are combined with environment-specific overrides:

```
=== Base Configuration Example ===

Running in environment: prod

=== Final Configuration ===
App Name: Base Config Example
Environment: production

Database:
  Host: prod-db.example.com
  Port: 5432
  Name: prod_app_db
  Username: app_user
  Password: su*********************

Server:
  Host: localhost
  Port: 443
  Timeout: 60 seconds
  Max Connections: 1000

Features:
  caching: enabled
  debug: disabled
  logging: enabled  
  metrics: enabled

External Services:
  Redis: enabled (Host: prod-redis.example.com:6379)
  RabbitMQ: enabled (Host: prod-rabbitmq.example.com:5672)
```

## Integration with Existing Apps

To use base configuration support in your existing Modular applications:

1. **Create the directory structure**:
   ```bash
   mkdir -p config/base config/environments/prod config/environments/staging
   ```

2. **Move your existing config** to `config/base/default.yaml`

3. **Create environment overrides** in `config/environments/{env}/overrides.yaml`

4. **Enable base config support** in your application:
   ```go
   // Set base configuration support
   modular.SetBaseConfig("config", environment)
   
   // Or let the framework auto-detect the structure
   app := modular.NewStdApplication(configProvider, logger)
   ```

The framework will automatically detect the base config structure and enable the feature if you don't explicitly set it up.