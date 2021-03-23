package mapper

import "github.com/figment-networks/indexing-engine/metrics"

var (
	conversionDuration = metrics.MustNewHistogramWithTags(metrics.HistogramOptions{
		Namespace: "indexerworker",
		Subsystem: "api",
		Name:      "conversion_duration",
		Desc:      "Duration how long it takes to convert from raw to format",
		Tags:      []string{"type"},
	})

	blockConversionDuration       *metrics.GroupObserver
	transactionConversionDuration *metrics.GroupObserver
)
