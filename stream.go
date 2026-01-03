package connectrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"

	"github.com/grafana/sobek"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// message represents a message in the stream queue
type message struct {
	isClosing bool
	msg       []byte
}

const (
	opened = iota + 1
	closed
)

type stream struct {
	vu     modules.VU
	client *Client

	logger logrus.FieldLogger

	methodDescriptor protoreflect.MethodDescriptor

	method        string
	connectStream *connect.BidiStreamForClient[dynamicpb.Message, dynamicpb.Message]

	tagsAndMeta *metrics.TagsAndMeta
	tq          *taskqueue.TaskQueue

	instanceMetrics *instanceMetrics
	builtinMetrics  *metrics.BuiltinMetrics

	obj *sobek.Object // the object that is given to js to interact with the stream

	writingState int8
	done         chan struct{}

	writeQueueCh chan message

	eventListeners *eventListeners

	timeoutCancel context.CancelFunc

	// Timing for metrics
	streamStartTime time.Time

	// Track if stream was explicitly closed
	explicitlyClosed bool

	// Ensure readLoop starts only once, after the first successful send
	startReadLoopOnce sync.Once

	// Track read loop lifecycle for safe shutdown ordering
	readLoopStarted atomic.Bool
	readLoopDone    chan struct{}
	closeQueueOnce  sync.Once
}

// defineStream defines the sobek.Object that is given to js to interact with the Stream
func defineStream(rt *sobek.Runtime, s *stream) {
	must(rt, s.obj.DefineDataProperty(
		"on", rt.ToValue(s.on), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE))

	must(rt, s.obj.DefineDataProperty(
		"write", rt.ToValue(s.write), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE))

	must(rt, s.obj.DefineDataProperty(
		"end", rt.ToValue(s.end), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE))

	must(rt, s.obj.DefineDataProperty(
		"close", rt.ToValue(s.close), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE))
}

func (s *stream) beginStream(p *callParams) error {
	// Record stream start time for metrics
	s.streamStartTime = time.Now()

	// Get or create HTTP client based on connection strategy
	var httpClient *http.Client
	var err error

	if s.client.connectionStrategy == "per-call" {
		// For per-call strategy, create a fresh HTTP client for this stream
		if s.client.connectParams == nil {
			return errors.New("invalid ConnectRPC Stream's client: no connection parameters available for per-call strategy")
		}
		httpClient, err = s.client.createHTTPClient(s.client.connectParams, s.client.addr)
		if err != nil {
			return fmt.Errorf("failed to create HTTP client for per-call stream: %w", err)
		}
	} else {
		// For per-vu and per-iteration strategies, use existing client
		if s.client.httpClient == nil {
			return errors.New("invalid ConnectRPC Stream's client: no ConnectRPC connection, you must call connect first")
		}
		httpClient = s.client.httpClient
	}

	// Create the dynamic client just-in-time
	procedureString := s.method

	// Prepare client options based on connection parameters
	clientOptions := []connect.ClientOption{
		connect.WithSchema(s.methodDescriptor),
		connect.WithResponseInitializer(func(spec connect.Spec, msg any) error {
			dynamic, ok := msg.(*dynamicpb.Message)
			if !ok {
				return nil
			}
			desc, ok := spec.Schema.(protoreflect.MethodDescriptor)
			if !ok {
				return fmt.Errorf("invalid schema type %T for %T message", spec.Schema, dynamic)
			}
			if spec.IsClient {
				*dynamic = *dynamicpb.NewMessage(desc.Output())
			} else {
				*dynamic = *dynamicpb.NewMessage(desc.Input())
			}
			return nil
		}),
	}

	// Add protocol-specific options based on connection parameters
	if s.client.connectParams != nil {
		switch s.client.connectParams.Protocol {
		case "grpc":
			clientOptions = append(clientOptions, connect.WithGRPC())
		case "grpc-web":
			clientOptions = append(clientOptions, connect.WithGRPCWeb())
		case "connect":
			// "connect" protocol is the default, no additional option needed
		}

		// Add JSON codec if JSON content type is specified
		if s.client.connectParams.ContentType == "application/json" {
			clientOptions = append(clientOptions, connect.WithProtoJSON())
		}
	}

	dynamicClient := connect.NewClient[dynamicpb.Message, dynamicpb.Message](
		httpClient,
		s.client.baseURL+procedureString,
		clientOptions...,
	)

	// This call is non-blocking. It just prepares the stream object.
	// Configure timeout for streaming (support infinite timeout)
	var ctx context.Context
	var cancel context.CancelFunc
	if p.Timeout != nil && *p.Timeout > 0 {
		ctx, cancel = context.WithTimeout(s.vu.Context(), *p.Timeout)
		s.timeoutCancel = cancel // Store cancel to be called on shutdown
	} else {
		// No timeout - use base context for infinite streams
		ctx = s.vu.Context()
		s.timeoutCancel = nil
	}
	s.connectStream = dynamicClient.CallBidiStream(ctx)

	// Apply headers before the first write
	for key, value := range p.Metadata {
		s.connectStream.RequestHeader().Set(key, value)
	}

	// Start writeLoop goroutine - the connection will be initiated on the first s.connectStream.Send()
	// Note: readLoop is started after the first successful Send() to avoid race conditions
	// where readLoop fails before any writes happen (the connection isn't established until first Send)
	go s.writeLoop()

	// Record stream start metrics
	if s.instanceMetrics != nil {
		protocol := "connect"
		contentType := "application/json"
		if s.client.connectParams != nil {
			protocol = s.client.connectParams.Protocol
			contentType = s.client.connectParams.ContentType
		}
		tags := s.client.createMetricTags(s.method, protocol, contentType)
		tags.Type = "stream"
		s.instanceMetrics.recordStreamStart(s.vu.Context(), s.vu, tags)
	}

	return nil
}

// on attaches an event listener to the stream
func (s *stream) on(event string, listener sobek.Value) {
	if s.eventListeners == nil {
		s.eventListeners = newEventListeners()
	}

	s.eventListeners.add(event, listener)
}

// write sends a message to the stream
func (s *stream) write(data sobek.Value) {
	if s.writingState == closed {
		if rt := s.vu.Runtime(); rt != nil {
			common.Throw(rt, errors.New("cannot write to a closed stream"))
		}
		return
	}

	// Convert the data to bytes
	var msgBytes []byte
	if data != nil && !sobek.IsUndefined(data) && !sobek.IsNull(data) {
		rt := s.vu.Runtime()
		if rt == nil {
			return
		}
		obj := data.ToObject(rt)
		jsonBytes, err := obj.MarshalJSON()
		if err != nil {
			common.Throw(rt, fmt.Errorf("failed to marshal message: %w", err))
			return
		}
		msgBytes = jsonBytes
	}

	// Send message through the write queue
	select {
	case s.writeQueueCh <- message{msg: msgBytes}:
	case <-s.done:
		// Check if runtime is available before throwing
		if rt := s.vu.Runtime(); rt != nil {
			common.Throw(rt, errors.New("stream is closed"))
		}
		return
	}
}

// end closes the client side of the stream
func (s *stream) end() {
	if s.writingState == closed {
		return
	}

	s.writingState = closed

	// Send closing message
	select {
	case s.writeQueueCh <- message{isClosing: true}:
	case <-s.done:
	}
}

// close terminates the entire stream immediately (both read and write sides)
func (s *stream) close() {
	// Mark that this was an explicit close
	s.explicitlyClosed = true

	// Cancel the timeout context if it exists
	if s.timeoutCancel != nil {
		s.timeoutCancel()
	}

	// Force close the stream by calling shutdown directly
	s.shutdown()
}

// writeLoop handles writing messages to the stream
func (s *stream) writeLoop() {
	for {
		select {
		case msg := <-s.writeQueueCh:
			if msg.isClosing {
				// Process any remaining messages before closing
				for {
					select {
					case pendingMsg := <-s.writeQueueCh:
						if pendingMsg.isClosing {
							continue // Skip additional closing messages
						}
						s.processMessage(pendingMsg)
					default:
						// No more pending messages
						if s.connectStream != nil {
							_ = s.connectStream.CloseRequest() // Use the official API
						}
						s.shutdown()
						return
					}
				}
			}
			s.processMessage(msg)

		case <-s.done:
			return
		}
	}
}

// processMessage handles the actual sending of a message
func (s *stream) processMessage(msg message) {
	requestMessage := dynamicpb.NewMessage(s.methodDescriptor.Input())
	if err := protojson.Unmarshal(msg.msg, requestMessage); err != nil {
		s.logger.WithError(err).Error("Failed to unmarshal message for sending")
		s.emitError(err)
		s.shutdown() // Assuming a shutdown function exists
		return
	}

	if err := s.connectStream.Send(requestMessage); err != nil {
		s.logger.WithError(err).Error("Failed to write to stream")
		s.emitError(err)
		s.shutdown()
		return
	}

	// Start readLoop after the first successful send - this ensures the connection
	// is established before we try to receive (avoiding race conditions)
	s.startReadLoopOnce.Do(func() {
		s.readLoopStarted.Store(true)
		go s.readLoop()
	})

	// Record sent message metrics
	if s.instanceMetrics != nil {
		protocol := "connect"
		contentType := "application/json"
		if s.client.connectParams != nil {
			protocol = s.client.connectParams.Protocol
			contentType = s.client.connectParams.ContentType
		}
		tags := s.client.createMetricTags(s.method, protocol, contentType)
		tags.Type = "stream"
		messageSize := int64(len(msg.msg))
		s.instanceMetrics.recordStreamMessage(s.vu.Context(), s.vu, tags, "sent", messageSize)
	}
}

// readLoop handles reading messages from the stream
func (s *stream) readLoop() {
	defer close(s.readLoopDone)
	// Note: We don't defer s.shutdown() here because read errors shouldn't
	// prevent writes from continuing. Shutdown is called from writeLoop
	// when the write side is intentionally closed (via end()), or from
	// close() when the entire stream is terminated.

	for {
		res, err := s.connectStream.Receive()
		if err != nil {
			// Check for normal EOF (direct or Connect-wrapped)
			if errors.Is(err, io.EOF) {
				s.emitEnd()
				return
			}

			// Check for Connect-wrapped EOF errors
			if connectErr := new(connect.Error); errors.As(err, &connectErr) {
				if strings.Contains(connectErr.Message(), "EOF") {
					s.emitEnd()
					return
				}
			}

			// Check for context cancellation (from explicit close())
			if errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "context canceled") {
				if s.explicitlyClosed {
					// This was an explicit close(), emit end instead of error
					s.emitEnd()
					return
				}
			}

			// Check for Connect-wrapped context cancellation
			if connectErr := new(connect.Error); errors.As(err, &connectErr) {
				if connectErr.Code().String() == "canceled" || strings.Contains(connectErr.Message(), "context canceled") {
					if s.explicitlyClosed {
						// This was an explicit close(), emit end instead of error
						s.emitEnd()
						return
					}
				}
			}

			s.logger.WithError(err).Error("Failed to read from stream")
			s.emitError(err) // This will now be a connect.Error
			return
		}

		// res is already a dynamicpb.Message, marshal it directly.
		jsonBytes, err := protojson.Marshal(res)
		if err != nil {
			s.emitError(err)
			return
		}

		// Record received message metrics
		if s.instanceMetrics != nil {
			protocol := "connect"
			contentType := "application/json"
			if s.client.connectParams != nil {
				protocol = s.client.connectParams.Protocol
				contentType = s.client.connectParams.ContentType
			}
			tags := s.client.createMetricTags(s.method, protocol, contentType)
			tags.Type = "stream"
			messageSize := int64(len(jsonBytes))
			s.instanceMetrics.recordStreamMessage(s.vu.Context(), s.vu, tags, "received", messageSize)
		}

		// emitData now receives JSON bytes instead of raw bytes
		s.emitData(jsonBytes)
	}
}

// emitData emits a 'data' event with the received data
func (s *stream) emitData(data []byte) {
	s.tq.Queue(func() error {
		rt := s.vu.Runtime()
		if rt == nil {
			return nil
		}
		// Try to parse as JSON and convert to JS object
		var result interface{}
		if len(data) > 0 {
			if err := json.Unmarshal(data, &result); err != nil {
				// If JSON parsing fails, emit as string
				s.eventListeners.emit("data", rt.ToValue(string(data)))
			} else {
				// Emit as parsed object
				s.eventListeners.emit("data", rt.ToValue(result))
			}
		}
		return nil
	})
}

// emitEnd emits an 'end' event
func (s *stream) emitEnd() {
	s.tq.Queue(func() error {
		s.eventListeners.emit("end", sobek.Undefined())
		return nil
	})
}

// emitError emits an 'error' event
func (s *stream) emitError(err error) {
	// Record stream error metrics
	if s.instanceMetrics != nil && !s.streamStartTime.IsZero() {
		duration := time.Since(s.streamStartTime)
		protocol := "connect"
		contentType := "application/json"
		if s.client.connectParams != nil {
			protocol = s.client.connectParams.Protocol
			contentType = s.client.connectParams.ContentType
		}
		tags := s.client.createMetricTags(s.method, protocol, contentType)
		tags.Type = "stream"
		s.instanceMetrics.recordStreamEnd(s.vu.Context(), s.vu, duration, tags, err)
	}

	s.tq.Queue(func() error {
		rt := s.vu.Runtime()
		if rt == nil {
			return nil
		}
		var errValue sobek.Value

		// Check if it's a connect.Error
		if connectErr := new(connect.Error); errors.As(err, &connectErr) {
			// Create error object for connect.Error
			errorObj := rt.NewObject()
			must(rt, errorObj.Set("code", rt.ToValue(connectErr.Code().String())))
			must(rt, errorObj.Set("message", rt.ToValue(connectErr.Error())))
			must(rt, errorObj.Set("details", rt.ToValue(connectErr.Details())))
			errValue = errorObj
		} else {
			// Fallback for generic errors
			errorObj := rt.NewObject()
			must(rt, errorObj.Set("message", err.Error()))
			errValue = errorObj
		}

		s.eventListeners.emit("error", errValue)
		return nil
	})
}

// shutdown closes the stream and cleans up resources
func (s *stream) shutdown() {
	// Record stream end metrics
	if s.instanceMetrics != nil && !s.streamStartTime.IsZero() {
		duration := time.Since(s.streamStartTime)
		protocol := "connect"
		contentType := "application/json"
		if s.client.connectParams != nil {
			protocol = s.client.connectParams.Protocol
			contentType = s.client.connectParams.ContentType
		}
		tags := s.client.createMetricTags(s.method, protocol, contentType)
		tags.Type = "stream"
		s.instanceMetrics.recordStreamEnd(s.vu.Context(), s.vu, duration, tags, nil)
	}

	// Close the done channel and task queue when shutdown is called
	select {
	case <-s.done:
		// Already closed
	default:
		close(s.done)
	}

	if s.readLoopStarted.Load() {
		go func() {
			<-s.readLoopDone
			s.closeTaskQueue()
		}()
		return
	}

	s.closeTaskQueue()
}

func (s *stream) closeTaskQueue() {
	s.closeQueueOnce.Do(func() {
		if s.tq != nil {
			s.tq.Close()
		}
	})
}

// eventListeners manages event listeners for the stream
type eventListeners struct {
	mu        sync.RWMutex
	listeners map[string][]sobek.Value
}

func newEventListeners() *eventListeners {
	return &eventListeners{
		listeners: make(map[string][]sobek.Value),
	}
}

func (el *eventListeners) add(event string, listener sobek.Value) {
	el.mu.Lock()
	defer el.mu.Unlock()

	if el.listeners[event] == nil {
		el.listeners[event] = make([]sobek.Value, 0)
	}
	el.listeners[event] = append(el.listeners[event], listener)
}

func (el *eventListeners) emit(event string, data sobek.Value) {
	el.mu.RLock()
	listeners := el.listeners[event]
	el.mu.RUnlock()

	for _, listener := range listeners {
		if fn, ok := sobek.AssertFunction(listener); ok {
			if data != nil {
				_, _ = fn(sobek.Undefined(), data)
			} else {
				_, _ = fn(sobek.Undefined())
			}
		}
	}
}

// must is a helper function for handling errors in Sobek operations
func must(rt *sobek.Runtime, err error) {
	if err != nil {
		panic(rt.NewGoError(err))
	}
}
