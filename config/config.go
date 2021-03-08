package config

// Config struct for config.yml
type Config struct {
	Worker         WorkerConfig
	ProxyBaseURL   string         `json:"proxy_base_url"`
	IndexerManager IndexerManager `json:"indexer_manager"`
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

// IndexerManager base url and listening port
type IndexerManager struct {
	BaseURL    string `json:"base_url"`
	Host       string `json:"host"`
	ListenPort string `json:"listen_port"`
}
