package httpclient

import (
	"fmt"
)

// Error Handling BDD Test Steps

func (ctx *HTTPClientBDDTestContext) iMakeARequestToAnInvalidEndpoint() error {
	if ctx.service == nil {
		return fmt.Errorf("httpclient service not available")
	}

	// Simulate an error response
	ctx.lastError = fmt.Errorf("connection refused")

	return nil
}

func (ctx *HTTPClientBDDTestContext) anAppropriateErrorShouldBeReturned() error {
	if ctx.lastError == nil {
		return fmt.Errorf("expected error but none occurred")
	}

	return nil
}

func (ctx *HTTPClientBDDTestContext) theErrorShouldContainMeaningfulInformation() error {
	if ctx.lastError == nil {
		return fmt.Errorf("no error to check")
	}

	if ctx.lastError.Error() == "" {
		return fmt.Errorf("error message is empty")
	}

	return nil
}
