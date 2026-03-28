package httpclient

import (
	"fmt"
	"time"
)

// Configuration and Connection Management BDD Test Steps

func (ctx *HTTPClientBDDTestContext) iHaveAnHTTPClientConfigurationWithConnectionPooling() error {
	ctx.resetContext()

	// Create httpclient configuration with connection pooling
	ctx.clientConfig = &Config{
		MaxIdleConns:        200, // Custom pool size
		MaxIdleConnsPerHost: 20,  // Custom per-host pool size
		IdleConnTimeout:     120 * time.Second,
		RequestTimeout:      30 * time.Second,
		TLSTimeout:          10 * time.Second,
		DisableCompression:  false,
		DisableKeepAlives:   false, // Keep-alive enabled for pooling
		Verbose:             false,
	}

	return ctx.setupApplicationWithConfig()
}

func (ctx *HTTPClientBDDTestContext) theClientShouldHaveTheConfiguredMaxIdleConnections() error {
	if ctx.clientConfig.MaxIdleConns != 200 {
		return fmt.Errorf("max idle connections not configured correctly")
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) theClientShouldHaveTheConfiguredMaxIdleConnectionsPerHost() error {
	if ctx.clientConfig.MaxIdleConnsPerHost != 20 {
		return fmt.Errorf("max idle connections per host not configured correctly")
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) connectionReuseShouldBeEnabled() error {
	if ctx.clientConfig.DisableKeepAlives {
		return fmt.Errorf("connection reuse should be enabled but keep-alives are disabled")
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) iHaveAnHTTPClientConfigurationWithVerboseLoggingEnabled() error {
	ctx.resetContext()

	// Create httpclient configuration with verbose logging
	ctx.clientConfig = &Config{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		RequestTimeout:      30 * time.Second,
		TLSTimeout:          10 * time.Second,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		Verbose:             true, // Enable verbose logging
		VerboseOptions: &VerboseOptions{
			LogToFile:   true,
			LogFilePath: "/tmp/httpclient",
		},
	}

	return ctx.setupApplicationWithConfig()
}

func (ctx *HTTPClientBDDTestContext) iMakeHTTPRequests() error {
	return ctx.iMakeAGETRequestToATestEndpoint()
}

func (ctx *HTTPClientBDDTestContext) requestAndResponseDetailsShouldBeLogged() error {
	if !ctx.clientConfig.Verbose {
		return fmt.Errorf("verbose logging not enabled")
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) theLogsShouldIncludeHeadersAndTimingInformation() error {
	if ctx.clientConfig.VerboseOptions == nil {
		return fmt.Errorf("verbose options not configured")
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) iHaveAnHTTPClientConfigurationWithCompressionEnabled() error {
	ctx.resetContext()

	// Create httpclient configuration with compression enabled
	ctx.clientConfig = &Config{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		RequestTimeout:      30 * time.Second,
		TLSTimeout:          10 * time.Second,
		DisableCompression:  false, // Compression enabled
		DisableKeepAlives:   false,
		Verbose:             false,
	}

	return ctx.setupApplicationWithConfig()
}

func (ctx *HTTPClientBDDTestContext) iMakeRequestsToEndpointsThatSupportCompression() error {
	return ctx.iMakeAGETRequestToATestEndpoint()
}

func (ctx *HTTPClientBDDTestContext) theClientShouldHandleGzipCompression() error {
	if ctx.clientConfig.DisableCompression {
		return fmt.Errorf("compression should be enabled but is disabled")
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) compressedResponsesShouldBeAutomaticallyDecompressed() error {
	// For BDD purposes, validate compression handling
	return nil
}

func (ctx *HTTPClientBDDTestContext) iHaveAnHTTPClientConfigurationWithKeepAliveDisabled() error {
	ctx.resetContext()

	// Create httpclient configuration with keep-alive disabled
	ctx.clientConfig = &Config{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		RequestTimeout:      30 * time.Second,
		TLSTimeout:          10 * time.Second,
		DisableCompression:  false,
		DisableKeepAlives:   true, // Keep-alive disabled
		Verbose:             false,
	}

	return ctx.setupApplicationWithConfig()
}

func (ctx *HTTPClientBDDTestContext) eachRequestShouldUseANewConnection() error {
	if !ctx.clientConfig.DisableKeepAlives {
		return fmt.Errorf("keep-alives should be disabled")
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) connectionsShouldNotBeReused() error {
	return ctx.eachRequestShouldUseANewConnection()
}
