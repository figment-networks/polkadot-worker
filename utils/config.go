package utils

// Config struct for config.yml
type Config struct {
	ChainID string `yaml:"chainID"`
	Client  Client
}

// Client config struct
type Client struct {
	Grcp Grcp
}

// Grcp config struct
type Grcp struct {
	MaxMsgSize int    `yaml:"maxMsgSize"`
	URL        string `yaml:"url"`
}
