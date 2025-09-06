module github.com/GoCodeAlone/modular/examples/testing-scenarios

go 1.25

toolchain go1.25.0

require (
	github.com/GoCodeAlone/modular v1.4.2
	github.com/GoCodeAlone/modular/modules/chimux v0.0.0-00010101000000-000000000000
	github.com/GoCodeAlone/modular/modules/httpserver v0.0.0-00010101000000-000000000000
	github.com/GoCodeAlone/modular/modules/reverseproxy v0.0.0-00010101000000-000000000000
)

require (
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/cloudevents/sdk-go/v2 v2.16.1 // indirect
	github.com/go-chi/chi/v5 v5.2.2 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golobby/cast v1.3.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/GoCodeAlone/modular => ../../

replace github.com/GoCodeAlone/modular/modules/chimux => ../../modules/chimux

replace github.com/GoCodeAlone/modular/modules/httpserver => ../../modules/httpserver

replace github.com/GoCodeAlone/modular/modules/reverseproxy => ../../modules/reverseproxy
