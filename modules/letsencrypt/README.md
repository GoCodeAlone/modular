# Let's Encrypt Module

The Let's Encrypt module provides automatic SSL/TLS certificate generation and management using Let's Encrypt's ACME protocol. It integrates seamlessly with the Modular framework to provide HTTPS capabilities for your applications.

[![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/modular/modules/letsencrypt.svg)](https://pkg.go.dev/github.com/GoCodeAlone/modular/modules/letsencrypt)

## Features

- **Automatic Certificate Generation**: Obtain SSL/TLS certificates from Let's Encrypt automatically
- **Multiple Challenge Types**: Support for HTTP-01 and DNS-01 challenges
- **Auto-Renewal**: Automatic certificate renewal before expiration
- **Multiple DNS Providers**: Support for various DNS providers (Cloudflare, Route53, Azure DNS, etc.)
- **Staging Environment**: Use Let's Encrypt's staging environment for testing
- **Certificate Storage**: Persistent storage of certificates and account information
- **Production Ready**: Built with best practices for production deployments

## Installation

```bash
go get github.com/GoCodeAlone/modular/modules/letsencrypt
```

## Quick Start

### Basic Usage with HTTP Challenge

```go
package main

import (
    "context"
    "log/slog"
    "os"

    "github.com/GoCodeAlone/modular"
    "github.com/GoCodeAlone/modular/modules/letsencrypt"
    "github.com/GoCodeAlone/modular/modules/httpserver"
)

type AppConfig struct {
    LetsEncrypt letsencrypt.LetsEncryptConfig `yaml:"letsencrypt"`
    HTTPServer  httpserver.HTTPServerConfig   `yaml:"httpserver"`
}

func main() {
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
    
    config := &AppConfig{
        LetsEncrypt: letsencrypt.LetsEncryptConfig{
            Email:       "your-email@example.com",
            Domains:     []string{"example.com", "www.example.com"},
            UseStaging:  false, // Set to true for testing
            StoragePath: "./certs",
            AutoRenew:   true,
            RenewBefore: 30, // Renew 30 days before expiration
        },
        HTTPServer: httpserver.HTTPServerConfig{
            Host: "0.0.0.0",
            Port: 443,
            TLS:  &httpserver.TLSConfig{Enabled: true},
        },
    }

    configProvider := modular.NewStdConfigProvider(config)
    app := modular.NewStdApplication(configProvider, logger)

    // Register modules
    app.RegisterModule(letsencrypt.NewLetsEncryptModule())
    app.RegisterModule(httpserver.NewHTTPServerModule())

    if err := app.Run(); err != nil {
        logger.Error("Application error", "error", err)
        os.Exit(1)
    }
}
```

### DNS Challenge with Cloudflare

```go
config := &AppConfig{
    LetsEncrypt: letsencrypt.LetsEncryptConfig{
        Email:       "your-email@example.com",
        Domains:     []string{"*.example.com", "example.com"},
        UseStaging:  false,
        StoragePath: "./certs",
        AutoRenew:   true,
        UseDNS:      true,
        DNSProvider: &letsencrypt.DNSProviderConfig{
            Name: "cloudflare",
        },
        DNSConfig: map[string]string{
            "CLOUDFLARE_EMAIL":   "your-email@example.com",
            "CLOUDFLARE_API_KEY": "your-api-key",
        },
    },
}
```

## Configuration

### LetsEncryptConfig

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `email` | `string` | Email address for Let's Encrypt registration | Required |
| `domains` | `[]string` | List of domain names to obtain certificates for | Required |
| `use_staging` | `bool` | Use Let's Encrypt staging environment | `false` |
| `storage_path` | `string` | Directory for certificate storage | `"./letsencrypt"` |
| `renew_before` | `int` | Days before expiry to renew certificates | `30` |
| `auto_renew` | `bool` | Enable automatic renewal | `true` |
| `use_dns` | `bool` | Use DNS-01 challenges instead of HTTP-01 | `false` |

### DNS Provider Configuration

For DNS challenges, configure the DNS provider:

```yaml
letsencrypt:
  email: "your-email@example.com"
  domains:
    - "example.com"
    - "*.example.com"
  use_dns: true
  dns_provider:
    name: "cloudflare"
  dns_config:
    CLOUDFLARE_EMAIL: "your-email@example.com"
    CLOUDFLARE_API_KEY: "your-api-key"
```

### Supported DNS Providers

- **Cloudflare**: `cloudflare`
- **Route53 (AWS)**: `route53`
- **Azure DNS**: `azuredns`
- **Google Cloud DNS**: `gcloud`
- **DigitalOcean**: `digitalocean`
- **Namecheap**: `namecheap`

Each provider requires specific environment variables or configuration parameters.

## Integration with HTTP Server

The Let's Encrypt module works seamlessly with the HTTP Server module by implementing the `CertificateService` interface:

```go
// The HTTP server module will automatically use certificates from Let's Encrypt
app.RegisterModule(letsencrypt.NewLetsEncryptModule())
app.RegisterModule(httpserver.NewHTTPServerModule())
```

## Advanced Usage

### Custom Certificate Handling

```go
// Get certificate service for custom handling
var certService httpserver.CertificateService
app.GetService("certificateService", &certService)

// Get certificate for a specific domain
cert := certService.GetCertificate("example.com")
```

### Manual Certificate Operations

```go
letsEncryptModule := letsencrypt.NewLetsEncryptModule()

// Force certificate renewal
if err := letsEncryptModule.RenewCertificate("example.com"); err != nil {
    log.Printf("Failed to renew certificate: %v", err)
}
```

## Environment Variables

You can configure the module using environment variables:

```bash
LETSENCRYPT_EMAIL=your-email@example.com
LETSENCRYPT_DOMAINS=example.com,www.example.com
LETSENCRYPT_USE_STAGING=false
LETSENCRYPT_STORAGE_PATH=./certs
LETSENCRYPT_AUTO_RENEW=true
```

## Best Practices

1. **Use Staging for Testing**: Always test with `use_staging: true` to avoid rate limits
2. **Secure Storage**: Ensure certificate storage directory has proper permissions
3. **Monitor Renewals**: Set up monitoring for certificate renewal failures
4. **Backup Certificates**: Regularly backup your certificate storage directory
5. **DNS Challenge for Wildcards**: Use DNS challenges for wildcard certificates

## Troubleshooting

### Common Issues

1. **Rate Limits**: Use staging environment for testing
2. **DNS Propagation**: DNS challenges may take time to propagate
3. **Firewall**: Ensure port 80 is accessible for HTTP challenges
4. **Domain Validation**: Verify domain ownership and DNS configuration

### Debug Mode

Enable debug logging to troubleshoot issues:

```go
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
```

## Examples

See the [examples directory](../../examples/) for complete working examples:

- Basic HTTPS server with Let's Encrypt
- Multi-domain certificate management
- DNS challenge configuration

## Dependencies

- [lego](https://github.com/go-acme/lego) - ACME client library
- Works with the [httpserver](../httpserver/) module for HTTPS support

## License

This module is part of the Modular framework and is licensed under the [MIT License](../../LICENSE).
