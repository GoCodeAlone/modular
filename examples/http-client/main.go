package main

import (
	"log/slog"
	"os"

	"github.com/CrisisTextLine/modular"
	"github.com/CrisisTextLine/modular/feeders"
	"github.com/CrisisTextLine/modular/modules/chimux"
	"github.com/CrisisTextLine/modular/modules/httpclient"
	"github.com/CrisisTextLine/modular/modules/httpserver"
	"github.com/CrisisTextLine/modular/modules/reverseproxy"
)

type AppConfig struct {
	// Empty config struct for the http-client example
	// Configuration is handled by individual modules
}

func main() {
	// Configure feeders
	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewYamlFeeder("config.yaml"),
		feeders.NewEnvFeeder(),
	}

	// Create a new application
	app := modular.NewStdApplication(
		modular.NewStdConfigProvider(&AppConfig{}),
		slog.New(slog.NewTextHandler(
			os.Stdout,
			&slog.HandlerOptions{},
		)),
	)

	// Register the modules in the correct order
	// First the httpclient module, so it's available for the reverseproxy module
	app.RegisterModule(httpclient.NewHTTPClientModule())

	// Then the modules that depend on it
	app.RegisterModule(chimux.NewChiMuxModule())
	app.RegisterModule(reverseproxy.NewModule())
	app.RegisterModule(httpserver.NewHTTPServerModule())

	// Run application with lifecycle management
	if err := app.Run(); err != nil {
		app.Logger().Error("Application error", "error", err)
		os.Exit(1)
	}
}
