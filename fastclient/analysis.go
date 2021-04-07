package fastclient

import "github.com/prometheus/client_golang/prometheus"

// DoRequestMetrics reports the the metrics pertaining to the request details
func DoRequestMetrics(detail BodyDetail, hasHeaders prometheus.Counter, histogram prometheus.Histogram) {

	// report size
	histogram.Observe(float64(detail.Size))

	// increment the headerless counter
	if detail.HasHeaders {
		hasHeaders.Inc()
	}
}