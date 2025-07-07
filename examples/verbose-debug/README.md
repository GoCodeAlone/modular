# Verbose Configuration Debug Example

This example demonstrates the verbose configuration debugging functionality in the Modular framework. It shows how to enable detailed DEBUG level logging during configuration processing to troubleshoot configuration issues, particularly with InstanceAware environment variable mapping in the Database module.

## Features Demonstrated

1. **Verbose Configuration Debugging**: Enable detailed logging of the configuration loading process
2. **InstanceAware Environment Variable Mapping**: Show how multiple database instances are configured from environment variables
3. **Debug Configuration Processing**: Track which configs are being processed, which keys are evaluated, and which environment variables are searched

## Usage

```bash
cd examples/verbose-debug
go run main.go
```

## Key Concepts

### Enabling Verbose Debugging

```go
// Create application
app := modular.NewStdApplication(configProvider, logger)

// Enable verbose configuration debugging
app.SetVerboseConfig(true)

// Initialize - this will now show detailed debug logs
err := app.Init()
```

### Verbose Debug Output

When verbose debugging is enabled, you'll see detailed logs showing:

- Which configuration sections are being processed
- Which environment variables are being looked up
- Which configuration keys are being evaluated
- How instance-aware mapping works
- Success/failure of configuration operations

### Environment Variable Setup

The example sets up multiple database instances using the pattern:
- `DB_PRIMARY_*` for primary database
- `DB_SECONDARY_*` for secondary database  
- `DB_CACHE_*` for cache database

Each instance gets its own set of configuration variables like:
- `DB_PRIMARY_DRIVER=sqlite`
- `DB_PRIMARY_DSN=./primary.db`
- `DB_PRIMARY_MAX_CONNS=10`

## Benefits

This verbose debugging helps with:

1. **Troubleshooting**: See exactly what the framework is doing during config loading
2. **Environment Variable Issues**: Track which env vars are being searched for
3. **Instance Mapping Problems**: Debug why instance-aware configuration isn't working
4. **Configuration Flow**: Understand the order and process of config loading
5. **Development**: Get insights into how the modular framework processes configuration

## Sample Output

When you run the example, you'll see output like:

```
=== Verbose Configuration Debug Example ===
Setting up environment variables:
  APP_NAME=Verbose Debug Example
  DB_PRIMARY_DRIVER=sqlite
  ...

🔧 Enabling verbose configuration debugging...
DEBUG Verbose configuration debugging enabled

🚀 Initializing application with verbose debugging...
DEBUG Starting configuration loading process
DEBUG Configuration feeders available count=1
DEBUG Config feeder registered index=0 type=*feeders.VerboseEnvFeeder
DEBUG Added config feeder to builder type=*feeders.VerboseEnvFeeder
DEBUG Processing configuration sections
DEBUG Processing main configuration configType=*main.AppConfig section=_main
DEBUG VerboseEnvFeeder: Starting feed process structureType=*main.AppConfig
DEBUG VerboseEnvFeeder: Processing struct structType=main.AppConfig numFields=3 prefix=
DEBUG VerboseEnvFeeder: Processing field fieldName=AppName fieldType=string fieldKind=string
DEBUG VerboseEnvFeeder: Found env tag fieldName=AppName envTag=APP_NAME
DEBUG VerboseEnvFeeder: Looking up environment variable fieldName=AppName envName=APP_NAME envTag=APP_NAME prefix=
DEBUG VerboseEnvFeeder: Environment variable found fieldName=AppName envName=APP_NAME envValue=Verbose Debug Example
DEBUG VerboseEnvFeeder: Successfully set field value fieldName=AppName envName=APP_NAME envValue=Verbose Debug Example
...
```

## Use Cases

This functionality is particularly useful for:

- **Development**: Understanding how configuration loading works
- **Debugging**: Troubleshooting configuration issues in complex applications
- **Production Support**: Diagnosing environment variable problems
- **Module Development**: Testing how modules register and load configuration
- **Integration Testing**: Verifying configuration flow in CI/CD pipelines