# Contract: Authentication (Conceptual)

## Supported Mechanisms
- JWT (HS256, RS256)
- OIDC Authorization Code
- API Key (header)
- Custom pluggable authenticators

## Operations
- Authenticate(requestContext) → Principal|error
- ValidateToken(token) → Claims|error
- RefreshMetadata() → error (key rotation / JWKS)

## Principal Fields
- subject
- roles[]
- tenantID (optional)
- issuedAt
- expiresAt

## Error Cases
- ErrInvalidToken
- ErrExpiredToken
- ErrUnsupportedMechanism
