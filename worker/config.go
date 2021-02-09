package worker

// Config struct for config.yml
type Config struct {
	ChainID    string `yaml:"chainID"`
	Host       string `yaml:"host"`
	LogLevel   string `yaml:"logLevel"`
	Network    string `yaml:"network"`
	ProxyPort  string `yaml:"proxy_port"`
	WorkerPort string `yaml:"worker_port"`
	Proxy      Proxy
	Indexer    Indexer
}

// Proxy config
type Proxy struct {
	Client ProxyClient
}

// ProxyClient config
type ProxyClient struct {
	URL string `yaml:"url"`
}

// Indexer config
type Indexer struct {
	Client  IndexerClient
	Manager IndexerManager
}

// IndexerClient config
type IndexerClient struct {
	Page uint64 `yaml:"page"`
}

// IndexerManager config
type IndexerManager struct {
	Address string `yaml:"address"`
}
