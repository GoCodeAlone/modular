# Authentication Module Demo

This example demonstrates how to use the auth module for JWT-based authentication, password hashing, and user management.

## Overview

The example sets up:
- JWT token generation and validation
- Password hashing with bcrypt
- User registration and login endpoints
- Protected routes that require authentication
- In-memory user storage for demonstration

## Features Demonstrated

1. **JWT Authentication**: Generate and validate JWT tokens
2. **Password Security**: Hash passwords with bcrypt
3. **User Management**: Register new users and authenticate existing ones
4. **Protected Routes**: Secure endpoints that require valid tokens
5. **HTTP Integration**: RESTful API endpoints for auth operations

## API Endpoints

- `POST /api/register` - Register a new user
- `POST /api/login` - Login with username/password
- `GET /api/profile` - Get user profile (requires JWT token)
- `POST /api/refresh` - Refresh JWT token

## Running the Example

1. Start the application:
   ```bash
   go run main.go
   ```

2. The application will start on port 8080

## Testing Authentication

### Register a new user
```bash
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{"username": "testuser", "password": "SecurePassword123!"}'
```

### Login with credentials
```bash
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"username": "testuser", "password": "SecurePassword123!"}'
```

This will return a JWT token that you can use for authenticated requests.

### Access protected endpoint
```bash
# Replace {TOKEN} with the JWT token from login
curl -H "Authorization: Bearer {TOKEN}" \
  http://localhost:8080/api/profile
```

### Refresh token
```bash
# Replace {TOKEN} with the JWT token
curl -X POST http://localhost:8080/api/refresh \
  -H "Authorization: Bearer {TOKEN}"
```

## Configuration

The auth module is configured in `config.yaml`:

```yaml
auth:
  jwt_secret: "your-super-secret-key-change-in-production"
  jwt_expiration: 3600  # 1 hour in seconds
  password_min_length: 8
  bcrypt_cost: 12
```

## Security Features

1. **Strong Password Requirements**: Configurable minimum length and complexity
2. **JWT Expiration**: Tokens expire after a configurable time
3. **Secure Password Hashing**: Uses bcrypt with configurable cost
4. **Token Validation**: Comprehensive JWT token validation

## Error Handling

The example includes proper error handling for:
- Invalid credentials
- Expired tokens
- Malformed requests
- User registration conflicts
- Password validation failures

This demonstrates how to build secure authentication into modular applications.