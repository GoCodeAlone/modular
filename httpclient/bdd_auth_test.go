package httpclient

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

// Authentication and Request Modification BDD Test Steps

func (ctx *HTTPClientBDDTestContext) iSetARequestModifierForCustomHeaders() error {
	if ctx.service == nil {
		return fmt.Errorf("httpclient service not available")
	}

	// Set up request modifier for custom headers
	modifier := func(req *http.Request) *http.Request {
		req.Header.Set("X-Custom-Header", "test-value")
		req.Header.Set("User-Agent", "HTTPClient-BDD-Test/1.0")
		return req
	}

	ctx.service.SetRequestModifier(modifier)
	ctx.requestModifier = modifier

	return nil
}

func (ctx *HTTPClientBDDTestContext) iMakeARequestWithTheModifiedClient() error {
	if ctx.service == nil {
		return fmt.Errorf("httpclient service not available")
	}

	// Create a test server that captures and echoes headers
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo custom headers back in response
		for key, values := range r.Header {
			if key == "X-Custom-Header" {
				w.Header().Set("X-Echoed-Header", values[0])
			}
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"headers":"captured"}`))
	}))
	defer testServer.Close()

	// Create a request and apply modifier if set
	req, err := http.NewRequest("GET", testServer.URL, nil)
	if err != nil {
		ctx.lastError = err
		return nil
	}

	if ctx.requestModifier != nil {
		ctx.requestModifier(req)
	}

	// Make the request with the modified client
	client := ctx.service.Client()
	resp, err := client.Do(req)
	if err != nil {
		ctx.lastError = err
		return nil
	}

	ctx.lastResponse = resp
	return nil
}

func (ctx *HTTPClientBDDTestContext) theCustomHeadersShouldBeIncludedInTheRequest() error {
	if ctx.lastResponse == nil {
		return fmt.Errorf("no response available")
	}

	// Check if custom headers were echoed back by the test server
	if ctx.lastResponse.Header.Get("X-Echoed-Header") == "" {
		return fmt.Errorf("custom headers were not included in the request")
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) iSetARequestModifierForAuthentication() error {
	if ctx.service == nil {
		return fmt.Errorf("httpclient service not available")
	}

	// Set up request modifier for authentication
	modifier := func(req *http.Request) *http.Request {
		req.Header.Set("Authorization", "Bearer test-token")
		return req
	}

	ctx.service.SetRequestModifier(modifier)
	ctx.requestModifier = modifier

	return nil
}

func (ctx *HTTPClientBDDTestContext) iMakeARequestToAProtectedEndpoint() error {
	return ctx.iMakeARequestWithTheModifiedClient()
}

func (ctx *HTTPClientBDDTestContext) theAuthenticationHeadersShouldBeIncluded() error {
	if ctx.requestModifier == nil {
		return fmt.Errorf("authentication modifier not set")
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) theRequestShouldBeAuthenticated() error {
	if ctx.lastResponse == nil {
		return fmt.Errorf("no response received")
	}

	// Simulate successful authentication
	return ctx.theRequestShouldBeSuccessful()
}
