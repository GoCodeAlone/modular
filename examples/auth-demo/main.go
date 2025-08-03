package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
	"github.com/GoCodeAlone/modular/modules/auth"
	"github.com/GoCodeAlone/modular/modules/chimux"
	"github.com/GoCodeAlone/modular/modules/httpserver"
	"github.com/go-chi/chi/v5"
)

type AppConfig struct {
	Name string `yaml:"name" default:"Auth Demo"`
}

type UserRegistration struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserLogin struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  string `json:"user"`
}

type ProfileResponse struct {
	Username string `json:"username"`
	UserID   string `json:"user_id"`
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create config provider
	appConfig := &AppConfig{}
	configProvider := modular.NewStdConfigProvider(appConfig)

	// Create application
	app := modular.NewStdApplication(configProvider, logger)

	// Set up configuration feeders
	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewYamlFeeder("config.yaml"),
		feeders.NewEnvFeeder(),
	}

	// Register modules
	app.RegisterModule(auth.NewModule())
	app.RegisterModule(chimux.NewChiMuxModule())
	app.RegisterModule(httpserver.NewHTTPServerModule())

	// Register API routes module
	app.RegisterModule(NewAPIModule())

	// Run the application
	if err := app.Run(); err != nil {
		logger.Error("Application error", "error", err)
		os.Exit(1)
	}
}

// APIModule provides HTTP routes for authentication
type APIModule struct {
	router      chi.Router
	authService auth.AuthService
	logger      modular.Logger
}

func NewAPIModule() modular.Module {
	return &APIModule{}
}

func (m *APIModule) Name() string {
	return "api"
}

func (m *APIModule) Dependencies() []string {
	return []string{"auth", "chimux"}
}

func (m *APIModule) RegisterConfig(app modular.Application) error {
	// No additional config needed
	return nil
}

func (m *APIModule) Init(app modular.Application) error {
	m.logger = app.Logger()

	// Get auth service
	if err := app.GetService("authService", &m.authService); err != nil {
		return fmt.Errorf("failed to get auth service: %w", err)
	}

	// Get router
	if err := app.GetService("chimux.router", &m.router); err != nil {
		return fmt.Errorf("failed to get router service: %w", err)
	}

	m.setupRoutes()
	return nil
}

func (m *APIModule) setupRoutes() {
	m.router.Route("/api", func(r chi.Router) {
		r.Post("/register", m.handleRegister)
		r.Post("/login", m.handleLogin)
		r.Post("/refresh", m.handleRefresh)
		
		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(m.authMiddleware)
			r.Get("/profile", m.handleProfile)
		})
	})
}

func (m *APIModule) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req UserRegistration
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate password strength
	if err := m.authService.ValidatePasswordStrength(req.Password); err != nil {
		http.Error(w, fmt.Sprintf("Password validation failed: %v", err), http.StatusBadRequest)
		return
	}

	// Hash password
	hashedPassword, err := m.authService.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	// In a real application, you would store this in a database
	// For demo purposes, we'll just log it
	m.logger.Info("User registered", "username", req.Username, "hashedPassword", hashedPassword)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User registered successfully",
		"username": req.Username,
	})
}

func (m *APIModule) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req UserLogin
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// In a real application, you would fetch the user from database
	// For demo purposes, we'll hash the password and verify it matches
	hashedPassword, err := m.authService.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	// Verify password (in real app, you'd compare with stored hash)
	if err := m.authService.VerifyPassword(hashedPassword, req.Password); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate JWT token
	token, err := m.authService.GenerateToken(req.Username, map[string]interface{}{
		"user_id": "demo_" + req.Username,
	})
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token: token.AccessToken,
		User:  req.Username,
	})
}

func (m *APIModule) handleRefresh(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Authorization header required", http.StatusUnauthorized)
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	
	// Validate current token
	claims, err := m.authService.ValidateToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Extract username from claims
	username := claims.Subject
	if username == "" {
		http.Error(w, "Invalid token claims", http.StatusUnauthorized)
		return
	}

	// Generate new token
	newToken, err := m.authService.RefreshToken(tokenString)
	if err != nil {
		http.Error(w, "Failed to refresh token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token: newToken.AccessToken,
		User:  username,
	})
}

func (m *APIModule) handleProfile(w http.ResponseWriter, r *http.Request) {
	// Get user info from context (set by middleware)
	username := r.Context().Value("username").(string)
	userID := r.Context().Value("user_id").(string)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ProfileResponse{
		Username: username,
		UserID:   userID,
	})
}

func (m *APIModule) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		
		claims, err := m.authService.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add user info to context
		ctx := r.Context()
		ctx = context.WithValue(ctx, "username", claims.Subject)
		if userID, ok := claims.Custom["user_id"]; ok {
			ctx = context.WithValue(ctx, "user_id", userID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *APIModule) Start(ctx context.Context) error {
	m.logger.Info("API module started")
	return nil
}

func (m *APIModule) Stop(ctx context.Context) error {
	m.logger.Info("API module stopped")
	return nil
}

func (m *APIModule) ProvidesServices() []modular.ServiceProvider {
	return []modular.ServiceProvider{}
}

func (m *APIModule) RequiresServices() []modular.ServiceDependency {
	return []modular.ServiceDependency{
		{Name: "authService", Required: true},
		{Name: "chimux.router", Required: true},
	}
}