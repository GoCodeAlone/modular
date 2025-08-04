# Let's Encrypt Demo

A demonstration of SSL/TLS concepts and the Let's Encrypt module integration patterns for the Modular framework.

## ⚠️ Important Note

This demo demonstrates the **concepts and patterns** for Let's Encrypt integration rather than actual certificate generation. The Let's Encrypt module requires specific configuration and production setup that would be complex for a simple demo environment.

## Features

- **SSL/TLS Concepts**: Demonstrates TLS connection analysis and security headers
- **Integration Patterns**: Shows how to structure applications for Let's Encrypt integration
- **Certificate Monitoring**: API endpoints to inspect SSL/TLS configuration patterns
- **Security Headers**: Demonstration of secure HTTP headers
- **Interactive Web Interface**: Browser-accessible interface showing SSL concepts

## Quick Start

**Demo Mode**: This example demonstrates SSL/TLS concepts without actual Let's Encrypt certificates.

1. **Start the application:**
   ```bash
   go run main.go
   ```

2. **Access via HTTP:**
   ```bash
   curl http://localhost:8080/
   
   # Or open in browser
   # http://localhost:8080/
   ```

3. **Check health:**
   ```bash
   curl http://localhost:8080/health
   ```

4. **View SSL information:**
   ```bash
   curl http://localhost:8080/api/ssl/info
   ```

## API Endpoints

### SSL Information

- **GET /api/ssl/info** - TLS connection details
  ```json
  {
    "tls_version": "TLS 1.3",
    "cipher_suite": "TLS_AES_256_GCM_SHA384",
    "server_name": "localhost",
    "handshake_complete": true,
    "certificate": {
      "subject": "CN=localhost",
      "issuer": "CN=Let's Encrypt Staging",
      "not_before": "2024-01-15T10:30:00Z",
      "not_after": "2024-04-15T10:30:00Z",
      "dns_names": ["localhost", "127.0.0.1"]
    }
  }
  ```

- **GET /api/ssl/certificates** - Certificate service status
- **GET /api/ssl/test** - SSL security tests

### General

- **GET /** - Interactive web interface showing SSL status
- **GET /health** - Health check endpoint

## Configuration

The demo is configured in `config.yaml`:

```yaml
letsencrypt:
  email: "demo@example.com"         # Required for Let's Encrypt registration
  domains:
    - "localhost"                   # Demo domains (staging only)
    - "127.0.0.1"
  use_staging: true                 # IMPORTANT: Use staging for demo/testing
  storage_path: "./certs"           # Certificate storage directory
  auto_renew: true                  # Enable automatic renewal
  renew_before: 30                  # Renew 30 days before expiry

httpserver:
  port: 8443                        # HTTPS port
  host: "0.0.0.0"
  tls:
    enabled: true                   # Enable TLS/HTTPS
```

## Demo Features

### 1. Staging Environment Safety
- Uses Let's Encrypt staging environment to avoid rate limits
- Generates untrusted certificates for testing purposes
- Safe for development and demonstration

### 2. Certificate Information
View detailed certificate information including:
- Subject and issuer details
- Validity period (not before/after dates)
- Supported domain names (SAN)
- Serial number and CA status

### 3. TLS Connection Analysis
Inspect TLS connection properties:
- TLS version (1.2, 1.3)
- Cipher suite negotiated
- Protocol negotiation results
- Handshake completion status

### 4. Security Headers
Demonstrates security best practices:
- `Strict-Transport-Security` (HSTS)
- `X-Content-Type-Options`
- `X-Frame-Options`
- `X-XSS-Protection`

### 5. Interactive Web Interface
Browser-accessible interface showing:
- Current connection security status
- Certificate configuration details
- Links to API endpoints for testing
- Production setup guidance

## Example Usage

### Check SSL Status

```bash
# Get comprehensive SSL information
curl -k https://localhost:8443/api/ssl/info | jq .

# Run security tests
curl -k https://localhost:8443/api/ssl/test | jq .

# Check certificate service
curl -k https://localhost:8443/api/ssl/certificates | jq .
```

### Browser Testing

1. Open `https://localhost:8443/` in your browser
2. Accept the security warning (staging certificates are untrusted)
3. View the interactive interface showing SSL status
4. Click on API endpoints to test functionality

### Certificate Inspection

```bash
# View certificate details with OpenSSL
echo | openssl s_client -connect localhost:8443 -servername localhost 2>/dev/null | openssl x509 -text -noout

# Check certificate expiry
echo | openssl s_client -connect localhost:8443 2>/dev/null | openssl x509 -noout -dates
```

## Production Setup

To use this for production with real certificates:

### 1. Update Configuration

```yaml
letsencrypt:
  email: "your-email@yourdomain.com"  # Your real email
  domains:
    - "yourdomain.com"                # Your real domain
    - "www.yourdomain.com"
  use_staging: false                  # IMPORTANT: Set to false for production
  storage_path: "/etc/letsencrypt"    # Secure storage location
  auto_renew: true
  renew_before: 30

httpserver:
  port: 443                           # Standard HTTPS port
  host: "0.0.0.0"
  tls:
    enabled: true
```

### 2. DNS Configuration

- Point your domain's A/AAAA records to your server's IP
- Ensure DNS propagation is complete before starting

### 3. Firewall Configuration

```bash
# Allow HTTP (for ACME challenges)
sudo ufw allow 80/tcp

# Allow HTTPS
sudo ufw allow 443/tcp
```

### 4. Domain Validation

- Ensure your server is reachable on port 80 for HTTP-01 challenges
- Or configure DNS-01 challenges for wildcard certificates

## Certificate Management

### Automatic Renewal
- Certificates are automatically renewed 30 days before expiry
- No manual intervention required
- Renewal process is logged for monitoring

### Manual Operations
```bash
# Force certificate renewal (if needed)
# This would be done through the application's management interface
```

### Storage
- Certificates are stored in the configured `storage_path`
- Account information is persisted for renewals
- Secure file permissions are automatically set

## Security Considerations

### Development/Testing
- ✅ Use staging environment (`use_staging: true`)
- ✅ Use localhost/test domains
- ✅ Accept certificate warnings in browsers

### Production
- ✅ Use production environment (`use_staging: false`)
- ✅ Use real, publicly accessible domains
- ✅ Implement proper monitoring for certificate expiry
- ✅ Backup certificate storage directory
- ✅ Use secure file permissions for certificate storage

## Troubleshooting

### Common Issues

1. **Rate Limits**: Use staging environment for testing
2. **Domain Validation**: Ensure domains point to your server
3. **Firewall**: Check ports 80 and 443 are accessible
4. **DNS**: Wait for DNS propagation after domain changes

### Debug Mode

Enable detailed logging:
```go
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
```

### Certificate Verification

```bash
# Check if certificate is valid
curl -I https://yourdomain.com

# Test with multiple tools
openssl s_client -connect yourdomain.com:443 -servername yourdomain.com
```

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTPS Client  │────│   TLS Server    │────│ Let's Encrypt   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                        │
                       ┌─────────────────┐    ┌─────────────────┐
                       │ Certificate     │────│ ACME Protocol   │
                       │ Management      │    │ HTTP-01/DNS-01  │
                       └─────────────────┘    └─────────────────┘
                                │
                       ┌─────────────────┐
                       │ Auto Renewal    │
                       │ & Storage       │
                       └─────────────────┘
```

## Learning Objectives

This demo teaches:

- How to integrate Let's Encrypt with Modular applications
- Automatic SSL/TLS certificate generation and management
- Proper staging vs production environment usage
- TLS connection analysis and certificate inspection
- Security header implementation
- HTTPS server configuration and best practices

## Dependencies

- [lego](https://github.com/go-acme/lego) - ACME client library for Let's Encrypt
- Integration with [httpserver](../httpserver/) module for TLS termination
- Modular framework for service orchestration

## Next Steps

- Configure DNS-01 challenges for wildcard certificates
- Implement certificate monitoring and alerting
- Add certificate backup and restore functionality
- Create load balancer integration for multi-server deployments
- Implement certificate pinning for enhanced security