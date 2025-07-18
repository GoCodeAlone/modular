module advanced-logging

go 1.24.2

toolchain go1.24.4

require (
	github.com/CrisisTextLine/modular v1.4.0
	github.com/CrisisTextLine/modular/modules/chimux v1.1.0
	github.com/CrisisTextLine/modular/modules/httpclient v0.1.0
	github.com/CrisisTextLine/modular/modules/httpserver v0.1.1
	github.com/CrisisTextLine/modular/modules/reverseproxy v1.1.0
)

require (
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/go-chi/chi/v5 v5.2.2 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golobby/cast v1.3.3 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/CrisisTextLine/modular => ../../

replace github.com/CrisisTextLine/modular/modules/chimux => ../../modules/chimux

replace github.com/CrisisTextLine/modular/modules/httpclient => ../../modules/httpclient

replace github.com/CrisisTextLine/modular/modules/httpserver => ../../modules/httpserver

replace github.com/CrisisTextLine/modular/modules/reverseproxy => ../../modules/reverseproxy
