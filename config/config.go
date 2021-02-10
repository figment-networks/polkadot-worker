package config

// Config struct for config.yml
type Config struct {
	Worker                WorkerConfig
	CeloClientBaseURL     string               `json:"celo_client_base_url"`
	PolkadotClientBaseURL string               `json:"polkadot_client_base_url"`
	IndexerManager        IndexerManagerConfig `json:"indexer_manager"`
}

// WorkerConfig config
type WorkerConfig struct {
	ChainID  string
	LogLevel string
	Network  string
	Version  string
	Address  WorkerAddress
}

// WorkerAddress host and port
type WorkerAddress struct {
	Host string
	Port string
}

// IndexerManagerConfig url and page size
type IndexerManagerConfig struct {
	BaseURL string `json:"base_url"`
	Page    uint64
}
