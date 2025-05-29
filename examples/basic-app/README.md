# Basic App Example

This example demonstrates the fundamental usage of the modular framework with a simple web application.

## What it demonstrates

- **Modular Application Setup**: How to create a basic modular application using `modular.NewStdApplication()`
- **Configuration Management**: Using YAML configuration files with environment variable overrides
- **Module Registration**: Registering and using custom modules (webserver, router, API)
- **Service Dependencies**: How modules can depend on and interact with each other
- **Configuration Validation**: Custom validation logic with default values and required fields
- **Application Lifecycle**: Proper startup, running, and shutdown handling

## Features

- Custom webserver module with configurable host/port
- Router module for HTTP request routing
- API module with sample endpoints
- Health check endpoint (`/health`)
- User management endpoints (`/api/v1/users/`, `/api/v1/users/{id}`)
- Environment-specific configuration (dev, test, prod)
- CORS support with configurable origins

## Running the Example

```bash
cd examples/basic-app

# Build the application
go build -o basic-app .

# Run with default configuration
./basic-app

# Generate a sample configuration file
./basic-app --generate-config yaml config-sample.yaml
```

## Testing the Application

Once running, you can test the endpoints:

```bash
# Health check
curl http://localhost:8080/health

# List users
curl http://localhost:8080/api/v1/users/

# Get specific user
curl http://localhost:8080/api/v1/users/123
```

## Configuration

The example uses `config.yaml` for configuration with the following structure:

- `appName`: Application name
- `environment`: Environment type (dev, test, prod)
- `server`: Server configuration (host, port, timeouts)
- `database`: Database settings
- `features`: Feature toggles
- `cors`: CORS origins
- `admins`: Admin user emails

## Key Files

- `main.go`: Main application file with configuration and module registration
- `config.yaml`: Application configuration
- `webserver/webserver.go`: Custom webserver module
- `router/router.go`: HTTP router module  
- `api/api.go`: API endpoints module

This example serves as the foundation for understanding how to build modular applications with the framework.
