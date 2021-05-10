package http

import "github.com/figment-networks/indexing-engine/metrics"

var (
	endpointDuration = metrics.MustNewHistogramWithTags(metrics.HistogramOptions{
		Namespace: "indexerworkerlive",
		Subsystem: "client",
		Name:      "endpoint_duration",
		Desc:      "Duration how long it takes for each endpoint",
		Tags:      []string{"type"},
	})
)
