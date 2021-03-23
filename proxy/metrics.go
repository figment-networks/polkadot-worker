package proxy

import "github.com/figment-networks/indexing-engine/metrics"

var (
	rawRequestHTTPDuration = metrics.MustNewHistogramWithTags(metrics.HistogramOptions{
		Namespace: "indexerworker",
		Subsystem: "api",
		Name:      "request_http",
		Desc:      "Duration how long it takes to take data from cosmos",
		Tags:      []string{"endpoint", "status"},
	})

	rawRequestGRPCDuration = metrics.MustNewHistogramWithTags(metrics.HistogramOptions{
		Namespace: "indexerworker",
		Subsystem: "api",
		Name:      "request_grpc",
		Desc:      "Duration how long it takes to take data from cosmos",
		Tags:      []string{"endpoint", "status"},
	})

	numberOfItems = metrics.MustNewCounterWithTags(metrics.Options{
		Namespace: "indexerworker",
		Subsystem: "api",
		Name:      "tx_num",
		Desc:      "Number of all transactions returned from one request",
		Tags:      []string{"type"},
	})

	numberOfItemsBlock = metrics.MustNewCounterWithTags(metrics.Options{
		Namespace: "indexerworker",
		Subsystem: "api",
		Name:      "block_tx_num",
		Desc:      "Number of all transactions returned from one request",
		Tags:      []string{"type"},
	})

	unknownTransactions = metrics.MustNewCounterWithTags(metrics.Options{
		Namespace: "indexerworker",
		Subsystem: "api",
		Name:      "tx_unknown",
		Desc:      "Number of unknown transactions",
		Tags:      []string{"type"},
	})

	brokenTransactions = metrics.MustNewCounterWithTags(metrics.Options{
		Namespace: "indexerworker",
		Subsystem: "api",
		Name:      "tx_broken",
		Desc:      "Number of broken transactions",
		Tags:      []string{"type"},
	})

	numberOfItemsTransactions *metrics.GroupCounter
	numberOfItemsInBlock      *metrics.GroupCounter
)
