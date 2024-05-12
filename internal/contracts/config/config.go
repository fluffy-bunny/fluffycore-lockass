package config

import (
	fluffycore_contracts_config "github.com/fluffy-bunny/fluffycore/contracts/config"
	fluffycore_contracts_ddprofiler "github.com/fluffy-bunny/fluffycore/contracts/ddprofiler"
)

type (
	JWTValidators struct {
		Issuers  []string `json:"issuers"`
		JWKSURLS []string `json:"jwksUrls"`
	}

	ConfigFiles struct {
		CorePath string `json:"corePath"`
	}
	InitialConfig struct {
		ConfigFiles ConfigFiles `json:"configFiles"`
	}
	MongoConfig struct {
		MongoUrl string `json:"mongoUrl"`
		Database string `json:"database"`
	}
)
type EchoConfig struct {
	Port int `json:"port"`
}
type Config struct {
	fluffycore_contracts_config.CoreConfig `mapstructure:",squash"`

	ConfigFiles      ConfigFiles                             `json:"configFiles"`
	OAuth2Port       int                                     `json:"oauth2Port"`
	JWTValidators    JWTValidators                           `json:"jwtValidators"`
	DDProfilerConfig *fluffycore_contracts_ddprofiler.Config `json:"ddProfilerConfig"`
	Echo             EchoConfig                              `json:"echo"`
	MongoConfig      MongoConfig                             `json:"mongoConfig"`
}

// ConfigDefaultJSON default json
var ConfigDefaultJSON = []byte(`
{
	"APPLICATION_NAME": "in-environment",
	"APPLICATION_ENVIRONMENT": "in-environment",
	"PRETTY_LOG": false,
	"LOG_LEVEL": "info",
	"PORT": 50051,
	"REST_PORT": 50052,
	"oauth2Port": 50053,
	"GRPC_GATEWAY_ENABLED": true,
	"jwtValidators": {},
	"mongoConfig": {
		"mongoUrl": "NA",
		"database": "lockaas"
	},
	"configFiles": {
        "corePath": "./config/core.json"
     },
	"ddProfilerConfig": {
		"enabled": false,
		"serviceName": "in-environment",
		"applicationEnvironment": "in-environment",
		"version": "1.0.0"
	},
	"echo": {
		"port": 9044 
	}

  }
`)
