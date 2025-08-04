# JSON Schema Demo

A comprehensive demonstration of the JSON Schema module's validation capabilities.

## Features

- **Schema Validation**: Validate JSON data against JSON Schema specifications
- **Schema Library**: Pre-loaded collection of common schemas
- **REST API**: Validate data via HTTP endpoints
- **Multiple Validation Methods**: Support for custom schemas and library schemas
- **Error Reporting**: Detailed validation error messages

## Quick Start

1. **Start the application:**
   ```bash
   go run main.go
   ```

2. **Check health:**
   ```bash
   curl http://localhost:8080/health
   ```

3. **List available schemas:**
   ```bash
   curl http://localhost:8080/api/schema/library
   ```

4. **Validate data with a library schema:**
   ```bash
   curl -X POST http://localhost:8080/api/schema/validate/user \
     -H "Content-Type: application/json" \
     -d '{
       "id": 1,
       "name": "John Doe",
       "email": "john@example.com",
       "age": 30,
       "role": "user"
     }'
   ```

## API Endpoints

### Schema Library

- **GET /api/schema/library** - List all available schemas
- **GET /api/schema/library/{name}** - Get a specific schema

### Validation

- **POST /api/schema/validate** - Validate data with custom schema
  ```json
  {
    "schema": "{\"type\": \"object\", \"properties\": {...}}",
    "data": {"key": "value"}
  }
  ```

- **POST /api/schema/validate/{name}** - Validate data with library schema
  ```json
  {"id": 1, "name": "John", "email": "john@example.com"}
  ```

### Health

- **GET /health** - Health check endpoint

## Pre-loaded Schemas

The demo includes several common schemas:

### User Schema
Validates user objects with required fields and constraints:
```json
{
  "id": 1,
  "name": "John Doe",
  "email": "john@example.com",
  "age": 30,
  "role": "user"
}
```

### Product Schema
Validates product information with pricing and categorization:
```json
{
  "id": "PROD-12345",
  "name": "Widget",
  "price": 29.99,
  "currency": "USD",
  "category": "electronics",
  "tags": ["gadget", "useful"]
}
```

### Order Schema
Validates order data with items and totals:
```json
{
  "order_id": "ORD-12345678",
  "customer_id": 1,
  "items": [
    {
      "product_id": "PROD-12345",
      "quantity": 2,
      "unit_price": 29.99
    }
  ],
  "total": 59.98,
  "status": "pending",
  "created_at": "2024-01-15T10:30:00Z"
}
```

### Configuration Schema
Validates application configuration:
```json
{
  "app_name": "MyApp",
  "version": "1.2.3",
  "debug": true,
  "database": {
    "host": "localhost",
    "port": 5432,
    "username": "user"
  },
  "features": {
    "logging": true,
    "analytics": false
  }
}
```

## Example Usage

### Validate with Library Schema

```bash
# Valid user data
curl -X POST http://localhost:8080/api/schema/validate/user \
  -H "Content-Type: application/json" \
  -d '{
    "id": 1,
    "name": "Alice Smith",
    "email": "alice@example.com",
    "age": 25,
    "role": "admin"
  }'

# Invalid user data (missing required field)
curl -X POST http://localhost:8080/api/schema/validate/user \
  -H "Content-Type: application/json" \
  -d '{
    "id": 1,
    "name": "Bob",
    "age": 25
  }'
```

### Validate Product Data

```bash
# Valid product
curl -X POST http://localhost:8080/api/schema/validate/product \
  -H "Content-Type: application/json" \
  -d '{
    "id": "PROD-67890",
    "name": "Super Widget",
    "price": 49.99,
    "currency": "USD",
    "category": "tools",
    "tags": ["premium", "durable"],
    "metadata": {
      "weight": "2.5kg",
      "dimensions": "10x15x8cm"
    }
  }'
```

### Validate with Custom Schema

```bash
curl -X POST http://localhost:8080/api/schema/validate \
  -H "Content-Type: application/json" \
  -d '{
    "schema": "{
      \"type\": \"object\",
      \"properties\": {
        \"name\": {\"type\": \"string\", \"minLength\": 1},
        \"count\": {\"type\": \"integer\", \"minimum\": 0}
      },
      \"required\": [\"name\"]
    }",
    "data": {
      "name": "Example",
      "count": 42
    }
  }'
```

### View Schema Details

```bash
# List all schemas
curl http://localhost:8080/api/schema/library

# Get specific schema
curl http://localhost:8080/api/schema/library/user
```

## Validation Features Demonstrated

1. **Type Validation**: Ensuring correct data types (string, number, boolean, etc.)
2. **Required Fields**: Validating that required fields are present
3. **Format Validation**: Email, date-time, and custom format validation
4. **Range Constraints**: Minimum/maximum values for numbers
5. **String Constraints**: Length restrictions and pattern matching
6. **Array Validation**: Item validation and uniqueness constraints
7. **Enum Validation**: Restricting values to predefined sets
8. **Nested Object Validation**: Validating complex object structures
9. **Additional Properties**: Controlling whether extra fields are allowed

## Error Handling

The API returns detailed validation errors:

```json
{
  "valid": false,
  "errors": [
    "missing property 'email'",
    "property 'age': must be <= 150",
    "property 'role': value must be one of [admin, user, guest]"
  ]
}
```

## Schema Standards

All schemas follow JSON Schema Draft 2020-12 specification:
- `$schema` declaration for version compatibility
- Proper type definitions and constraints
- Clear validation rules and error messages
- Support for nested objects and arrays
- Format validation for common data types

## Configuration

No special configuration is required for the JSON Schema module. It works with the default Modular framework configuration.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Client   │────│   REST API      │────│ Schema Service  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                       │
                       ┌─────────────────┐    ┌─────────────────┐
                       │ Schema Library  │────│ Validation      │
                       └─────────────────┘    └─────────────────┘
                                                       │
                       ┌─────────────────┐    ┌─────────────────┐
                       │ Custom Schemas  │────│ Error Reporting │
                       └─────────────────┘    └─────────────────┘
```

## Learning Objectives

This demo teaches:

- How to integrate JSON Schema module with Modular applications
- Creating and managing JSON Schema definitions
- Validating data programmatically and via API
- Handling validation errors and responses
- Building schema libraries for reusable validation
- Working with different JSON Schema features and constraints

## Production Considerations

- Cache compiled schemas for better performance
- Implement schema versioning for API evolution
- Use appropriate error handling for validation failures
- Consider schema registry for large-scale deployments
- Implement proper logging for validation events
- Use schema validation for API request/response validation

## Next Steps

- Integrate with API gateway for automatic validation
- Add schema versioning and migration support
- Create schema generation from Go structs
- Implement schema composition and inheritance
- Add custom validation keywords and formats
- Build schema-driven form generation for UIs