package reverseproxy

import "net/http"

// TenantIDFromRequest extracts the tenant ID from an HTTP request if present.
// It returns the tenant ID as a string and a boolean indicating if a tenant ID was found.
//
// Parameters:
//   - tenantIDHeader: The name of the HTTP header containing the tenant ID
//   - req: The HTTP request to extract the tenant ID from
//
// Returns:
//   - tenantID: The extracted tenant ID or an empty string if not found
//   - found: True if a tenant ID was found, false otherwise
func TenantIDFromRequest(tenantIDHeader string, req *http.Request) (string, bool) {
	tenantID := req.Header.Get(tenantIDHeader)
	if tenantID == "" {
		return "", false
	}
	return tenantID, true
}
