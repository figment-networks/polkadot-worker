package config

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/kelseyhightower/envconfig"
)

var (
	Name      = "polkadot-worker"
	Version   string
	GitSHA    string
	Timestamp string
)

const (
	modeDevelopment = "development"
	modeProduction  = "production"
)

// Config holds the configuration data
type Config struct {
	AppEnv string `json:"app_env" envconfig:"APP_ENV" default:"development"`

	Address  string `json:"address" envconfig:"ADDRESS" default:"0.0.0.0"`
	Port     string `json:"port" envconfig:"PORT" default:"3000"`
	HTTPPort string `json:"http_port" envconfig:"HTTP_PORT" default:"8087"`

	PolkadotProxyAddr string `json:"polkadot_proxy_addr" envconfig:"POLKADOT_PROXY_ADDR"`
	Network           string `json:"network" envconfig:"NETWORK" default:"polkadot"`
	ChainID           string `json:"chain_id" envconfig:"CHAIN_ID" default:"mainnet"`

	Managers        string        `json:"managers" envconfig:"MANAGERS" default:"127.0.0.1:8085"`
	ManagerInterval time.Duration `json:"manager_interval" envconfig:"MANAGER_INTERVAL" default:"10s"`
	Hostname        string        `json:"hostname" envconfig:"HOSTNAME"`

	MaximumHeightsToGet float64 `json:"maximum_heights_to_get" envconfig:"MAXIMUM_HEIGHTS_TO_GET" default:"10000"`

	// Rollbar
	RollbarAccessToken string `json:"rollbar_access_token" envconfig:"ROLLBAR_ACCESS_TOKEN"`
	RollbarServerRoot  string `json:"rollbar_server_root" envconfig:"ROLLBAR_SERVER_ROOT" default:"github.com/figment-networks/polkadot-worker"`

	HealthCheckInterval time.Duration `json:"health_check_interval" envconfig:"HEALTH_CHECK_INTERVAL" default:"10s"`

	Currency string `json:"currency" envconfig:"CURRENCY" default:"DOT"`
	Exp      int    `json:"exp" envconfig:"EXP"`
}

// FromFile reads the config from a file
func FromFile(path string, config *Config) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, config)
}

// FromEnv reads the config from environment variables
func FromEnv(config *Config) error {
	return envconfig.Process("", config)
}

/*

// IndexerManager base url and listening port
type IndexerManager struct {
	BaseURL         string `json:"base_url"`
	Host            string `json:"host"`
	ListenPort      string `json:"listen_port"`
  -----	MaxHeightsToGet uint64 `json:"max_heights_to_get"`
}

// WorkerConfig config
type WorkerConfig struct {
	---- ChainID  string `json:"chain_id"`
	--- Currency string `json:"currency"`
	--- Exp      int    `json:"exp"`
	--- LogLevel string `json:"log_level"`
	--- Network  string `json:"network"`
	--- Version  string `json:"version"`
	-- Host     string `json:"host"`
	-- Port     string `json:"port"`
}
*/
