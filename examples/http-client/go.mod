module http-client

go 1.24.2

toolchain go1.24.4

require (
	github.com/GoCodeAlone/modular v1.3.9
	github.com/GoCodeAlone/modular/modules/chimux v0.0.0
	github.com/GoCodeAlone/modular/modules/httpclient v0.0.0
	github.com/GoCodeAlone/modular/modules/httpserver v0.0.0
	github.com/GoCodeAlone/modular/modules/reverseproxy v0.0.0
)

require (
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/go-chi/chi/v5 v5.2.2 // indirect
	github.com/golobby/cast v1.3.3 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/GoCodeAlone/modular => ../../

replace github.com/GoCodeAlone/modular/modules/chimux => ../../modules/chimux

replace github.com/GoCodeAlone/modular/modules/httpclient => ../../modules/httpclient

replace github.com/GoCodeAlone/modular/modules/httpserver => ../../modules/httpserver

replace github.com/GoCodeAlone/modular/modules/reverseproxy => ../../modules/reverseproxy
