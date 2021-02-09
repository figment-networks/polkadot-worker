package proxy

import "github.com/figment-networks/indexing-engine/metrics"

var (
	conversionDuration = metrics.MustNewHistogramWithTags(metrics.HistogramOptions{
		Namespace: "indexers",
		Subsystem: "worker_api_polkadot",
		Name:      "conversion_duration",
		Desc:      "Duration how long it takes to convert from proxy to database model",
		Tags:      []string{"type"},
	})

	requestDuration = metrics.MustNewHistogramWithTags(metrics.HistogramOptions{
		Namespace: "indexers",
		Subsystem: "worker_api_polkadot",
		Name:      "request_duration",
		Desc:      "Duration how long it takes to take data from polkadot",
		Tags:      []string{"endpoint", "status"},
	})

	// BlockConversionDuration time duration to convert Block
	BlockConversionDuration *metrics.GroupObserver

	// TransactionConversionDuration  time duration to convert Transaction
	TransactionConversionDuration *metrics.GroupObserver
)
