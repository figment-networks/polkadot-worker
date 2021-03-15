package config

// Config struct for config.yml
type Config struct {
	IndexerManager IndexerManager `json:"indexer_manager"`
	ProxyBaseURL   string         `json:"proxy_base_url"`
	Worker         WorkerConfig
}

// IndexerManager base url and listening port
type IndexerManager struct {
	BaseURL         string `json:"base_url"`
	Host            string `json:"host"`
	ListenPort      string `json:"listen_port"`
	MaxHeightsToGet uint64 `json:"max_heights_to_get"`
}

// WorkerConfig config
type WorkerConfig struct {
	ChainID  string `json:"chain_id"`
	Currency string `json:"currency"`
	Exp      int    `json:"exp"`
	LogLevel string `json:"log_level"`
	Network  string `json:"network"`
	Version  string `json:"version"`
	Host     string `json:"host"`
	Port     string `json:"port"`
}
