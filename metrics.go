package connectrpc

import (
	"context"
	"time"

	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
)

// instanceMetrics holds the metrics for the ConnectRPC module instance
type instanceMetrics struct {
	// Unary request metrics
	ConnectRPCReqs        *metrics.Metric
	ConnectRPCReqDuration *metrics.Metric
	ConnectRPCReqErrors   *metrics.Metric

	// Stream metrics
	ConnectRPCStreams        *metrics.Metric
	ConnectRPCStreamDuration *metrics.Metric
	ConnectRPCStreamErrors   *metrics.Metric

	// Message metrics (granular)
	ConnectRPCStreamMsgsSent     *metrics.Metric
	ConnectRPCStreamMsgsReceived *metrics.Metric

	// Connection metrics
	ConnectRPCConnections        *metrics.Metric
	ConnectRPCConnectionDuration *metrics.Metric
	ConnectRPCConnectionErrors   *metrics.Metric

	// HTTP connection reuse metrics
	ConnectRPCHTTPConnectionsNew    *metrics.Metric
	ConnectRPCHTTPConnectionsReused *metrics.Metric
	ConnectRPCHTTPHandshakeDuration *metrics.Metric

	// Payload size metrics
	ConnectRPCReqSize  *metrics.Metric
	ConnectRPCRespSize *metrics.Metric
}

// MetricTags contains common tags for metrics
type MetricTags struct {
	Method      string // Full method name like "/clown.v1.ClownService/TellJoke"
	Service     string // Service name like "clown.v1.ClownService"
	Procedure   string // Method name like "TellJoke"
	Type        string // "unary" or "stream"
	Protocol    string // "connect", "grpc", etc.
	ContentType string // "application/json", "application/protobuf"
	Status      string // "success", "error", "cancelled"
}

// Helper functions for recording metrics with per-procedure tags

// recordUnaryRequest records metrics for a unary RPC call
func (m *instanceMetrics) recordUnaryRequest(ctx context.Context, vu modules.VU,
	duration time.Duration, reqSize, respSize int64, tags MetricTags, err error) {

	state := vu.State()
	if state == nil {
		return
	}

	// Get current tags and add our custom tags
	ctm := state.Tags.GetCurrentValues()
	ctm.SetTag("method", tags.Method)
	ctm.SetTag("service", tags.Service)
	ctm.SetTag("procedure", tags.Procedure)
	ctm.SetTag("type", tags.Type)
	ctm.SetTag("protocol", tags.Protocol)
	ctm.SetTag("content_type", tags.ContentType)

	if err != nil {
		ctm.SetTag("status", "error")
		// Record error
		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: m.ConnectRPCReqErrors,
				Tags:   ctm.Tags,
			},
			Time:     time.Now(),
			Metadata: ctm.Metadata,
			Value:    1,
		})
	} else {
		ctm.SetTag("status", "success")
	}

	// Record request count and duration
	now := time.Now()
	samples := []metrics.Sample{
		{
			TimeSeries: metrics.TimeSeries{
				Metric: m.ConnectRPCReqs,
				Tags:   ctm.Tags,
			},
			Time:     now,
			Metadata: ctm.Metadata,
			Value:    1,
		},
		{
			TimeSeries: metrics.TimeSeries{
				Metric: m.ConnectRPCReqDuration,
				Tags:   ctm.Tags,
			},
			Time:     now,
			Metadata: ctm.Metadata,
			Value:    metrics.D(duration),
		},
	}

	// Record payload sizes if available
	if reqSize > 0 {
		samples = append(samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: m.ConnectRPCReqSize,
				Tags:   ctm.Tags,
			},
			Time:     now,
			Metadata: ctm.Metadata,
			Value:    float64(reqSize),
		})
	}

	if respSize > 0 {
		samples = append(samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: m.ConnectRPCRespSize,
				Tags:   ctm.Tags,
			},
			Time:     now,
			Metadata: ctm.Metadata,
			Value:    float64(respSize),
		})
	}

	metrics.PushIfNotDone(ctx, state.Samples, metrics.ConnectedSamples{
		Samples: samples,
		Tags:    ctm.Tags,
		Time:    now,
	})
}

// recordStreamStart records when a stream is opened
func (m *instanceMetrics) recordStreamStart(ctx context.Context, vu modules.VU, tags MetricTags) {
	state := vu.State()
	if state == nil {
		return
	}

	ctm := state.Tags.GetCurrentValues()
	ctm.SetTag("method", tags.Method)
	ctm.SetTag("service", tags.Service)
	ctm.SetTag("procedure", tags.Procedure)
	ctm.SetTag("type", "stream")
	ctm.SetTag("protocol", tags.Protocol)
	ctm.SetTag("content_type", tags.ContentType)
	ctm.SetTag("status", "opened")

	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: m.ConnectRPCStreams,
			Tags:   ctm.Tags,
		},
		Time:     time.Now(),
		Metadata: ctm.Metadata,
		Value:    1,
	})
}

// recordStreamEnd records when a stream is closed
func (m *instanceMetrics) recordStreamEnd(ctx context.Context, vu modules.VU,
	duration time.Duration, tags MetricTags, err error) {

	state := vu.State()
	if state == nil {
		return
	}

	ctm := state.Tags.GetCurrentValues()
	ctm.SetTag("method", tags.Method)
	ctm.SetTag("service", tags.Service)
	ctm.SetTag("procedure", tags.Procedure)
	ctm.SetTag("type", "stream")
	ctm.SetTag("protocol", tags.Protocol)
	ctm.SetTag("content_type", tags.ContentType)

	if err != nil {
		ctm.SetTag("status", "error")
		// Record stream error
		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: m.ConnectRPCStreamErrors,
				Tags:   ctm.Tags,
			},
			Time:     time.Now(),
			Metadata: ctm.Metadata,
			Value:    1,
		})
	} else {
		ctm.SetTag("status", "closed")
	}

	// Record stream duration
	metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: m.ConnectRPCStreamDuration,
			Tags:   ctm.Tags,
		},
		Time:     time.Now(),
		Metadata: ctm.Metadata,
		Value:    metrics.D(duration),
	})
}

// recordStreamMessage records sent/received messages
func (m *instanceMetrics) recordStreamMessage(ctx context.Context, vu modules.VU,
	tags MetricTags, direction string, messageSize int64) {

	state := vu.State()
	if state == nil {
		return
	}

	ctm := state.Tags.GetCurrentValues()
	ctm.SetTag("method", tags.Method)
	ctm.SetTag("service", tags.Service)
	ctm.SetTag("procedure", tags.Procedure)
	ctm.SetTag("type", "stream")
	ctm.SetTag("protocol", tags.Protocol)
	ctm.SetTag("content_type", tags.ContentType)
	ctm.SetTag("direction", direction) // "sent" or "received"

	var metric *metrics.Metric
	switch direction {
	case "sent":
		metric = m.ConnectRPCStreamMsgsSent
	case "received":
		metric = m.ConnectRPCStreamMsgsReceived
	default:
		return
	}

	now := time.Now()
	samples := []metrics.Sample{
		{
			TimeSeries: metrics.TimeSeries{
				Metric: metric,
				Tags:   ctm.Tags,
			},
			Time:     now,
			Metadata: ctm.Metadata,
			Value:    1,
		},
	}

	// Record message size if available
	if messageSize > 0 {
		var sizeMetric *metrics.Metric
		if direction == "sent" {
			sizeMetric = m.ConnectRPCReqSize
		} else {
			sizeMetric = m.ConnectRPCRespSize
		}

		samples = append(samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: sizeMetric,
				Tags:   ctm.Tags,
			},
			Time:     now,
			Metadata: ctm.Metadata,
			Value:    float64(messageSize),
		})
	}

	metrics.PushIfNotDone(ctx, state.Samples, metrics.ConnectedSamples{
		Samples: samples,
		Tags:    ctm.Tags,
		Time:    now,
	})
}

// recordHTTPConnection records metrics for HTTP connection establishment or reuse
func (m *instanceMetrics) recordHTTPConnection(ctx context.Context, vu modules.VU,
	url string, isNewConnection bool, handshakeDuration time.Duration) {

	state := vu.State()
	if state == nil {
		return
	}

	ctm := state.Tags.GetCurrentValues()
	ctm.SetTag("url", url)

	now := time.Now()
	samples := []metrics.Sample{}

	if isNewConnection {
		ctm.SetTag("connection_type", "new")
		samples = append(samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: m.ConnectRPCHTTPConnectionsNew,
				Tags:   ctm.Tags,
			},
			Time:     now,
			Metadata: ctm.Metadata,
			Value:    1,
		})

		// Record handshake duration for new connections
		if handshakeDuration > 0 {
			samples = append(samples, metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: m.ConnectRPCHTTPHandshakeDuration,
					Tags:   ctm.Tags,
				},
				Time:     now,
				Metadata: ctm.Metadata,
				Value:    metrics.D(handshakeDuration),
			})
		}
	} else {
		ctm.SetTag("connection_type", "reused")
		samples = append(samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: m.ConnectRPCHTTPConnectionsReused,
				Tags:   ctm.Tags,
			},
			Time:     now,
			Metadata: ctm.Metadata,
			Value:    1,
		})
	}

	if len(samples) > 0 {
		metrics.PushIfNotDone(ctx, state.Samples, metrics.ConnectedSamples{
			Samples: samples,
			Tags:    ctm.Tags,
			Time:    now,
		})
	}
}

// registerMetrics registers the ConnectRPC module metrics
func registerMetrics(registry *metrics.Registry) (*instanceMetrics, error) {
	var err error
	m := &instanceMetrics{}

	// Unary request metrics
	if m.ConnectRPCReqs, err = registry.NewMetric(
		"connectrpc_reqs", metrics.Counter); err != nil {
		return nil, err
	}

	if m.ConnectRPCReqDuration, err = registry.NewMetric(
		"connectrpc_req_duration", metrics.Trend, metrics.Time); err != nil {
		return nil, err
	}

	if m.ConnectRPCReqErrors, err = registry.NewMetric(
		"connectrpc_req_errors", metrics.Counter); err != nil {
		return nil, err
	}

	// Stream metrics
	if m.ConnectRPCStreams, err = registry.NewMetric(
		"connectrpc_streams", metrics.Counter); err != nil {
		return nil, err
	}

	if m.ConnectRPCStreamDuration, err = registry.NewMetric(
		"connectrpc_stream_duration", metrics.Trend, metrics.Time); err != nil {
		return nil, err
	}

	if m.ConnectRPCStreamErrors, err = registry.NewMetric(
		"connectrpc_stream_errors", metrics.Counter); err != nil {
		return nil, err
	}

	// Message metrics (separated for better granularity)
	if m.ConnectRPCStreamMsgsSent, err = registry.NewMetric(
		"connectrpc_stream_msgs_sent", metrics.Counter); err != nil {
		return nil, err
	}

	if m.ConnectRPCStreamMsgsReceived, err = registry.NewMetric(
		"connectrpc_stream_msgs_received", metrics.Counter); err != nil {
		return nil, err
	}

	// Connection metrics
	if m.ConnectRPCConnections, err = registry.NewMetric(
		"connectrpc_connections", metrics.Counter); err != nil {
		return nil, err
	}

	if m.ConnectRPCConnectionDuration, err = registry.NewMetric(
		"connectrpc_connection_duration", metrics.Trend, metrics.Time); err != nil {
		return nil, err
	}

	if m.ConnectRPCConnectionErrors, err = registry.NewMetric(
		"connectrpc_connection_errors", metrics.Counter); err != nil {
		return nil, err
	}

	// HTTP connection reuse metrics
	if m.ConnectRPCHTTPConnectionsNew, err = registry.NewMetric(
		"connectrpc_http_connections_new", metrics.Counter); err != nil {
		return nil, err
	}

	if m.ConnectRPCHTTPConnectionsReused, err = registry.NewMetric(
		"connectrpc_http_connections_reused", metrics.Counter); err != nil {
		return nil, err
	}

	if m.ConnectRPCHTTPHandshakeDuration, err = registry.NewMetric(
		"connectrpc_http_handshake_duration", metrics.Trend, metrics.Time); err != nil {
		return nil, err
	}

	// Payload size metrics
	if m.ConnectRPCReqSize, err = registry.NewMetric(
		"connectrpc_req_size", metrics.Trend, metrics.Data); err != nil {
		return nil, err
	}

	if m.ConnectRPCRespSize, err = registry.NewMetric(
		"connectrpc_resp_size", metrics.Trend, metrics.Data); err != nil {
		return nil, err
	}

	return m, nil
}
