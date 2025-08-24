package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
)

// MockOAuth2Server provides a mock OAuth2 server for testing
type MockOAuth2Server struct {
	server       *httptest.Server
	clientID     string
	clientSecret string
	validCode    string
	validToken   string
	userInfo     map[string]interface{}
}

// NewMockOAuth2Server creates a new mock OAuth2 server
func NewMockOAuth2Server() *MockOAuth2Server {
	mock := &MockOAuth2Server{
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
		validCode:    "valid-auth-code",
		validToken:   "mock-access-token",
		userInfo: map[string]interface{}{
			"id":      "12345",
			"email":   "testuser@example.com",
			"name":    "Test User",
			"picture": "https://example.com/avatar.jpg",
		},
	}

	// Create HTTP server with OAuth2 endpoints
	mux := http.NewServeMux()
	
	// Authorization endpoint
	mux.HandleFunc("/oauth2/auth", mock.handleAuthEndpoint)
	
	// Token exchange endpoint
	mux.HandleFunc("/oauth2/token", mock.handleTokenEndpoint)
	
	// User info endpoint
	mux.HandleFunc("/oauth2/userinfo", mock.handleUserInfoEndpoint)

	mock.server = httptest.NewServer(mux)
	return mock
}

// Close closes the mock server
func (m *MockOAuth2Server) Close() {
	m.server.Close()
}

// GetBaseURL returns the base URL of the mock server
func (m *MockOAuth2Server) GetBaseURL() string {
	return m.server.URL
}

// GetClientID returns the test client ID
func (m *MockOAuth2Server) GetClientID() string {
	return m.clientID
}

// GetClientSecret returns the test client secret
func (m *MockOAuth2Server) GetClientSecret() string {
	return m.clientSecret
}

// GetValidCode returns a valid authorization code for testing
func (m *MockOAuth2Server) GetValidCode() string {
	return m.validCode
}

// GetValidToken returns a valid access token for testing
func (m *MockOAuth2Server) GetValidToken() string {
	return m.validToken
}

// SetUserInfo sets the user info that will be returned by the userinfo endpoint
func (m *MockOAuth2Server) SetUserInfo(userInfo map[string]interface{}) {
	m.userInfo = userInfo
}

// handleAuthEndpoint handles the OAuth2 authorization endpoint
func (m *MockOAuth2Server) handleAuthEndpoint(w http.ResponseWriter, r *http.Request) {
	// This endpoint would normally show a login form and redirect back with a code
	// For testing, we just return the parameters that would be used
	query := r.URL.Query()
	
	response := map[string]interface{}{
		"client_id":     query.Get("client_id"),
		"redirect_uri":  query.Get("redirect_uri"),
		"scope":         query.Get("scope"),
		"state":         query.Get("state"),
		"response_type": query.Get("response_type"),
		"auth_url":      r.URL.String(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleTokenEndpoint handles the OAuth2 token exchange endpoint
func (m *MockOAuth2Server) handleTokenEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Validate client credentials
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	if clientID != m.clientID || clientSecret != m.clientSecret {
		http.Error(w, "Invalid client credentials", http.StatusUnauthorized)
		return
	}

	// Validate grant type
	grantType := r.FormValue("grant_type")
	if grantType != "authorization_code" {
		http.Error(w, "Unsupported grant type", http.StatusBadRequest)
		return
	}

	// Validate authorization code
	code := r.FormValue("code")
	if code != m.validCode {
		http.Error(w, "Invalid authorization code", http.StatusBadRequest)
		return
	}

	// Return access token
	tokenResponse := map[string]interface{}{
		"access_token":  m.validToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": "mock-refresh-token",
		"scope":         "openid email profile",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokenResponse)
}

// handleUserInfoEndpoint handles the OAuth2 user info endpoint
func (m *MockOAuth2Server) handleUserInfoEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for valid access token
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Missing or invalid authorization header", http.StatusUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token != m.validToken {
		http.Error(w, "Invalid access token", http.StatusUnauthorized)
		return
	}

	// Return user info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m.userInfo)
}

// OAuth2Config creates an OAuth2 config for testing with this mock server
func (m *MockOAuth2Server) OAuth2Config(redirectURL string) OAuth2Provider {
	baseURL := m.GetBaseURL()
	return OAuth2Provider{
		ClientID:     m.clientID,
		ClientSecret: m.clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		AuthURL:      baseURL + "/oauth2/auth",
		TokenURL:     baseURL + "/oauth2/token",
		UserInfoURL:  baseURL + "/oauth2/userinfo",
	}
}