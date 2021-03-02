package config

// Config struct for config.yml
type Config struct {
	Worker                WorkerConfig
	ProxyBaseURL          string `json:"proxy_base_url"`
	IndexerManagerBaseURL string `json:"indexer_manager_base_url"`
}

// WorkerConfig config
type WorkerConfig struct {
	ChainID  string `json:"chain_id"`
	Currency string `json:"currency"`
	Exp      int    `json:"exp"`
	LogLevel string `json:"log_level"`
	Network  string `json:"network"`
	Version  string `json:"version"`
	Address  WorkerAddress
}

// WorkerAddress host and port
type WorkerAddress struct {
	Host     string `json:"host"`
	GRCPPort string `json:"grcp_port"`
	HTTPPort string `json:"http_port"`
}
