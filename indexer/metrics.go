package indexer

import "github.com/figment-networks/indexing-engine/metrics"

var (
	endpointDuration = metrics.MustNewHistogramWithTags(metrics.HistogramOptions{
		Namespace: "indexers",
		Subsystem: "worker_client_polkadot",
		Name:      "endpoint_duration",
		Desc:      "Duration how long it takes for each endpoint",
		Tags:      []string{"type"},
	})

	newStreamsMetric = metrics.MustNewCounterWithTags(metrics.Options{
		Namespace: "indexers",
		Subsystem: "worker_client_polkadot",
		Name:      "new_streams",
		Desc:      "New Streams",
	})

	receivedRequestsMetric = metrics.MustNewCounterWithTags(metrics.Options{
		Namespace: "indexers",
		Subsystem: "worker_client_polkadot",
		Name:      "received_requests",
		Desc:      "Received requests to process by client",
		Tags:      []string{"type"},
	})

	sendResponseMetric = metrics.MustNewCounterWithTags(metrics.Options{
		Namespace: "indexers",
		Subsystem: "worker_client_polkadot",
		Name:      "responses_sent",
		Desc:      "Responses to be sent from client",
		Tags:      []string{"type", "final"},
	})
)
