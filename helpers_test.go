package connectrpc_test

import (
	"go.k6.io/k6/metrics"
)

func drainSamples(samples chan metrics.SampleContainer) []metrics.SampleContainer {
	var result []metrics.SampleContainer
	for {
		select {
		case sample := <-samples:
			result = append(result, sample)
		default:
			return result
		}
	}
}
