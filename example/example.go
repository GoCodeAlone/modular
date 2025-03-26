package main

import (
	"example/api"
	"example/router"
	"example/webserver"
	"github.com/GoCodeAlone/modular"
	"github.com/GoCodeAlone/modular/feeders"
	"log/slog"
	"os"
)

func main() {
	modular.ConfigFeeders = []modular.Feeder{
		feeders.NewYamlFeeder("config.yaml"),
		feeders.NewEnvFeeder(),
	}
	app := modular.NewApplication(
		modular.NewStdConfigProvider(&myCfg{}),
		slog.New(slog.NewTextHandler(
			os.Stdout,
			&slog.HandlerOptions{},
		)),
	)

	app.RegisterModule(webserver.NewWebServer())
	app.RegisterModule(router.NewRouter())
	app.RegisterModule(api.NewAPIModule())

	// Run application with lifecycle management
	if err := app.Run(); err != nil {
		app.Logger().Error("Application error", "error", err)
		os.Exit(1)
	}
}

type myCfg struct {
	AppName string `yaml:"appName"`
}
