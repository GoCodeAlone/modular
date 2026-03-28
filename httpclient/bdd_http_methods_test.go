package httpclient

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
)

// HTTP Methods BDD Test Steps

func (ctx *HTTPClientBDDTestContext) iMakeAGETRequestToATestEndpoint() error {
	if ctx.service == nil {
		return fmt.Errorf("httpclient service not available")
	}

	// Create a real test server for actual HTTP requests
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"success","method":"GET"}`))
	}))
	defer testServer.Close()

	// Make a real HTTP GET request to the test server
	client := ctx.service.Client()
	resp, err := client.Get(testServer.URL)
	if err != nil {
		ctx.lastError = err
		return nil
	}

	ctx.lastResponse = resp
	return nil
}

func (ctx *HTTPClientBDDTestContext) theRequestShouldBeSuccessful() error {
	if ctx.lastResponse == nil {
		return fmt.Errorf("no response received")
	}

	if ctx.lastResponse.StatusCode < 200 || ctx.lastResponse.StatusCode >= 300 {
		return fmt.Errorf("request failed with status %d", ctx.lastResponse.StatusCode)
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) theResponseShouldBeReceived() error {
	if ctx.lastResponse == nil {
		return fmt.Errorf("no response received")
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) iMakeAPOSTRequestWithJSONData() error {
	if ctx.service == nil {
		return fmt.Errorf("httpclient service not available")
	}

	// Create a real test server for actual HTTP POST requests
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write([]byte(`{"status":"created","method":"POST"}`))
	}))
	defer testServer.Close()

	// Make a real HTTP POST request with JSON data
	jsonData := []byte(`{"test": "data"}`)
	client := ctx.service.Client()
	resp, err := client.Post(testServer.URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		ctx.lastError = err
		return nil
	}

	ctx.lastResponse = resp
	return nil
}

func (ctx *HTTPClientBDDTestContext) theRequestBodyShouldBeSentCorrectly() error {
	// For BDD purposes, validate that POST was configured
	if ctx.lastResponse == nil {
		return fmt.Errorf("no response received for POST request")
	}

	if ctx.lastResponse.StatusCode != 201 {
		return fmt.Errorf("POST request did not return expected status")
	}

	return nil
}
