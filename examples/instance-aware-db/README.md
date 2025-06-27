# Instance-Aware Database Configuration Example

This example demonstrates the new instance-aware environment variable configuration system for multiple database connections.

## Features Demonstrated

- Multiple database connections (primary, secondary, cache)
- Instance-specific environment variable mapping
- Automatic configuration from environment variables
- Consistent naming convention

## Environment Variables

The example uses the following environment variable pattern:

```bash
DB_<INSTANCE>_<FIELD>=<VALUE>
```

For example:
- `DB_PRIMARY_DRIVER=sqlite`
- `DB_PRIMARY_DSN=./primary.db`
- `DB_SECONDARY_DRIVER=sqlite`
- `DB_SECONDARY_DSN=./secondary.db`
- `DB_CACHE_DRIVER=sqlite`
- `DB_CACHE_DSN=:memory:`

## Running the Example

```bash
go run main.go
```

The example will:
1. Set up environment variables programmatically
2. Initialize the modular application with database module
3. Demonstrate multiple database connections
4. Show how each connection is configured independently
5. Clean up resources

## Key Benefits

1. **Separation of Concerns**: Each database instance has its own environment variables
2. **No Conflicts**: Different database connections don't interfere with each other
3. **Consistent Naming**: Predictable environment variable names
4. **Easy Configuration**: Simple to set up in different environments
5. **Automatic Mapping**: No manual configuration code needed