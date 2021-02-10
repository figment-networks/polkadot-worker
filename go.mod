module github.com/figment-networks/polkadot-worker

go 1.15

require (
	github.com/celo-org/kliento v0.1.2-0.20200608140637-c5afc8cf0f44
	github.com/ethereum/go-ethereum v1.9.8
	github.com/figment-networks/celo-indexer v0.1.8
	github.com/figment-networks/indexer-manager v0.0.9
	github.com/figment-networks/indexing-engine v0.1.14
	github.com/figment-networks/polkadothub-proxy/grpc v0.0.0-20201210164304-d5b9edfe1f12
	github.com/google/uuid v1.1.2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.13.0
	github.com/sirupsen/logrus v1.7.0 // indirect
	github.com/stretchr/testify v1.7.0
	go.uber.org/zap v1.16.0
	golang.org/x/net v0.0.0-20201010224723-4f7140c49acb // indirect
	golang.org/x/sys v0.0.0-20201013132646-2da7054afaeb // indirect
	google.golang.org/grpc v1.35.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/ethereum/go-ethereum => github.com/celo-org/celo-blockchain v1.1.0
