package config

// Config struct for config.yml
type Config struct {
	Worker         WorkerConfig
	ProxyBaseURL   string               `json:"proxy_base_url"`
	IndexerManager IndexerManagerConfig `json:"indexer_manager"`
}

// WorkerConfig config
type WorkerConfig struct {
	ChainID  string `json:"chain_id"`
	Currency string `json:"currency"`
	LogLevel string `json:"log_level"`
	Network  string `json:"network"`
	Version  string `json:"version"`
	Address  WorkerAddress
}

// WorkerAddress host and port
type WorkerAddress struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

// IndexerManagerConfig url and page size
type IndexerManagerConfig struct {
	BaseURL string `json:"base_url"`
	Page    uint64
}
