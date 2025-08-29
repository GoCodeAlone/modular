module instance-aware-db

go 1.25

replace github.com/CrisisTextLine/modular => ../..

replace github.com/CrisisTextLine/modular/modules/database => ../../modules/database

require (
	github.com/CrisisTextLine/modular v1.9.0
	github.com/CrisisTextLine/modular/modules/database v1.1.0
	github.com/mattn/go-sqlite3 v1.14.30
)

require (
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/aws/aws-sdk-go-v2 v1.38.0 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.31.0 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.18.4 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.3 // indirect
	github.com/aws/aws-sdk-go-v2/feature/rds/auth v1.5.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.28.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.33.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.37.0 // indirect
	github.com/aws/smithy-go v1.22.5 // indirect
	github.com/cloudevents/sdk-go/v2 v2.16.1 // indirect
	github.com/golobby/cast v1.3.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
