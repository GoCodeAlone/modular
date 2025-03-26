package main

import (
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

	err := app.Init()
	if err != nil {
		app.Logger().Error("Failed to initialize application:", "error", err)
		return
	}
	app.Logger().Info("Initialized application")
	app.Logger().Info("App Config:", "cfg", (app.ConfigProvider().GetConfig()).(*myCfg))
}

type myCfg struct {
	AppName string `yaml:"appName"`
}
