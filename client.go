package connectrpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"time"

	"connectrpc.com/connect"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"

	"github.com/grafana/sobek"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Client represents a ConnectRPC client that can be used to make RPC requests
type Client struct {
	httpClient         *http.Client
	vu                 modules.VU
	addr               string
	baseURL            string
	metrics            *instanceMetrics
	connectionStrategy string
	connectParams      *connectParams // Store connection params for per-call strategy

	// Connection tracking
	lastIterationID int64 // Track iteration for per-iteration strategy
}

// Connect establishes a connection to the ConnectRPC server at the given address
func (c *Client) Connect(addr string, params sobek.Value) (bool, error) {
	state := c.vu.State()
	if state == nil {
		return false, common.NewInitContextError("connecting to a ConnectRPC server in the init context is not supported")
	}

	p, err := newConnectParams(c.vu, params)
	if err != nil {
		return false, fmt.Errorf("invalid connectrpc.connect() parameters: %w", err)
	}

	// Parse address first to get hostname for TLS ServerName
	var hostname string
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		parsedURL, err := url.Parse(addr)
		if err != nil {
			return false, fmt.Errorf("invalid URL: %w", err)
		}
		hostname = parsedURL.Host
		c.baseURL = strings.TrimRight(addr, "/")
	} else {
		// This case is for when only a hostname is provided
		hostname = addr
		scheme := "https"
		if p.IsPlaintext {
			scheme = "http"
		}
		c.baseURL = fmt.Sprintf("%s://%s", scheme, addr)
	}
	c.addr = hostname

	// Store connection strategy for use in connection management
	c.connectionStrategy = p.ConnectionStrategy

	// Store connection parameters for potential per-call use
	c.connectParams = p

	// For per-call strategy, we don't create the HTTP client here
	if c.connectionStrategy == "per-call" {
		return true, nil
	}

	// Create HTTP client for per-vu and per-iteration strategies
	httpClient, err := c.createHTTPClient(p, hostname)
	if err != nil {
		return false, err
	}
	c.httpClient = httpClient

	return true, nil
}

// createHTTPClient creates an HTTP client with the specified parameters
func (c *Client) createHTTPClient(p *connectParams, hostname string) (*http.Client, error) {
	// Create HTTP transport with configurable HTTP version
	transport := &http.Transport{}

	// Configure HTTP version based on user preference
	switch p.HTTPVersion {
	case "1.1":
		transport.ForceAttemptHTTP2 = false
		transport.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
	case "2", "auto":
		transport.ForceAttemptHTTP2 = true
	default:
		transport.ForceAttemptHTTP2 = true // Default to HTTP/2
	}

	if !p.IsPlaintext {
		// Configure TLS with proper security defaults
		tlsCfg := &tls.Config{
			InsecureSkipVerify: false, // Default to secure verification
			ServerName:         hostname,
		}

		// Allow user to override TLS settings
		if len(p.TLS) > 0 {
			var err error
			if tlsCfg, err = buildTLSConfigFromMap(tlsCfg, p.TLS); err != nil {
				return nil, err
			}
			if tlsCfg.ServerName == "" {
				tlsCfg.ServerName = hostname
			}
		}
		transport.TLSClientConfig = tlsCfg
	} else {
		// For plaintext HTTP/2 (h2c)
		transport.TLSClientConfig = nil
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		}
	}

	// Create HTTP client with configurable timeout
	timeout := time.Duration(0) // No timeout by default
	if p.Timeout != nil {
		timeout = *p.Timeout
	}

	// Wrap transport with connection tracking
	trackingTransport := &connectionTrackingTransport{
		base:    transport,
		client:  c,
		baseURL: c.baseURL,
	}

	return &http.Client{
		Transport: trackingTransport,
		Timeout:   timeout,
	}, nil
}

// Invoke creates and calls a unary RPC by fully qualified method name
func (c *Client) Invoke(
	method string,
	reqJS sobek.Value,
	params sobek.Value,
) (*sobek.Object, error) {
	state := c.vu.State()
	if state == nil {
		return nil, common.NewInitContextError("invoking RPC methods is not supported in the init context")
	}

	// Check if Connect() was called successfully
	if c.httpClient == nil && c.connectParams == nil {
		return nil, errors.New("client not connected: call connect() first")
	}

	// Get or create HTTP client based on connection strategy
	var httpClient *http.Client
	var err error

	if c.connectionStrategy == "per-call" {
		// Create a fresh HTTP client for this call
		httpClient, err = c.createHTTPClient(c.connectParams, c.addr)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP client for per-call strategy: %w", err)
		}
	} else if c.connectionStrategy == "per-iteration" {
		// Check if we're in a new iteration
		currentIterationID := state.Iteration
		if c.httpClient == nil || c.lastIterationID != currentIterationID {
			// Close the existing connection if any
			if c.httpClient != nil {
				c.httpClient.CloseIdleConnections()
			}
			// Create a fresh HTTP client for this iteration
			httpClient, err = c.createHTTPClient(c.connectParams, c.addr)
			if err != nil {
				return nil, fmt.Errorf("failed to create HTTP client for per-iteration strategy: %w", err)
			}
			c.httpClient = httpClient
			c.lastIterationID = currentIterationID
		} else {
			// Reuse existing client for same iteration
			httpClient = c.httpClient
		}
	} else {
		// Use the existing HTTP client for per-vu strategy
		httpClient = c.httpClient
	}

	methodDesc, err := c.getMethodDescriptor(method)
	if err != nil {
		// Debug logging
		c.vu.State().Logger.WithField("method", method).WithError(err).Error("Failed to get method descriptor")
		return nil, err
	}

	p, err := newCallParams(c.vu, params)
	if err != nil {
		return nil, err
	}

	// Use infinite timeout by default (protocol compliant)
	callTimeout := time.Duration(0) // No timeout
	if p.Timeout != nil {
		callTimeout = *p.Timeout
	}

	// Create the dynamic client just-in-time
	// The full procedure string is just the method path
	procedureString := method // e.g., "/clown.v1.ClownService/TellJoke"
	url := c.baseURL + procedureString

	// Prepare the dynamic request message from JavaScript object
	reqJSON, err := reqJS.ToObject(c.vu.Runtime()).MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request object: %w", err)
	}

	requestMessage := dynamicpb.NewMessage(methodDesc.Input())
	if err := protojson.Unmarshal(reqJSON, requestMessage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON into dynamic protobuf message: %w", err)
	}

	// Prepare client options based on connection parameters
	clientOptions := []connect.ClientOption{
		connect.WithSchema(methodDesc),
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
	connParams := c.connectParams

	switch connParams.Protocol {
	case "grpc":
		clientOptions = append(clientOptions, connect.WithGRPC())
	case "grpc-web":
		clientOptions = append(clientOptions, connect.WithGRPCWeb())
	case "connect":
		// "connect" protocol is the default, no additional option needed
	}

	// Add JSON codec if JSON content type is specified
	if connParams.ContentType == "application/json" {
		clientOptions = append(clientOptions, connect.WithProtoJSON())
	}

	// Create the client with the baseURL and the full procedure string
	dynamicClient := connect.NewClient[dynamicpb.Message, dynamicpb.Message](
		httpClient,
		url,
		clientOptions...,
	)

	connectReq := connect.NewRequest(requestMessage)

	// First, set connection-level headers from connectParams
	if connParams.Headers != nil {
		for key, value := range connParams.Headers {
			connectReq.Header().Set(key, value)
		}
	}

	// Then, set call-level headers from p.Metadata (these can override connection-level headers)
	for key, value := range p.Metadata {
		connectReq.Header().Set(key, value)
	}

	// Make the call with configurable timeout
	var ctx context.Context
	var cancel context.CancelFunc
	if callTimeout > 0 {
		ctx, cancel = context.WithTimeout(c.vu.Context(), callTimeout)
		defer cancel()
	} else {
		// No timeout - use base context
		ctx = c.vu.Context()
	}

	// Record request start time for metrics
	requestStart := time.Now()

	resp, err := dynamicClient.CallUnary(ctx, connectReq)

	// Calculate duration and payload sizes for metrics
	requestDuration := time.Since(requestStart)
	var reqSize, respSize int64
	if reqJSON != nil {
		reqSize = int64(len(reqJSON))
	}

	// Create response object for k6
	rt := c.vu.Runtime()
	responseObject := rt.NewObject()

	if err != nil {
		// Handle Connect RPC errors by converting them to HTTP-like status codes
		var connectErr *connect.Error
		var httpStatus int
		var message string

		if errors.As(err, &connectErr) {
			// Convert Connect error codes to HTTP status codes
			httpStatus = connectCodeToHTTPStatus(connectErr.Code())
			message = connectErr.Error() // Use full error message like streaming code

			// Create error response object
			errorObj := rt.NewObject()
			must(rt, errorObj.Set("code", rt.ToValue(connectErr.Code().String())))
			must(rt, errorObj.Set("message", rt.ToValue(message)))

			// Serialize error details properly
			details := connectErr.Details()
			serializedDetails := make([]map[string]interface{}, len(details))
			for i, detail := range details {
				serializedDetail := make(map[string]interface{})
				serializedDetail["type"] = detail.Type()
				serializedDetail["bytes"] = detail.Bytes()

				// Try to get the protobuf value
				if value, err := detail.Value(); err == nil {
					serializedDetail["value"] = value
				}

				serializedDetails[i] = serializedDetail
			}
			must(rt, errorObj.Set("details", rt.ToValue(serializedDetails)))

			must(rt, responseObject.Set("message", errorObj))
			must(rt, responseObject.Set("status", rt.ToValue(httpStatus)))
			must(rt, responseObject.Set("headers", rt.ToValue(connectErr.Meta())))
			must(rt, responseObject.Set("trailers", rt.ToValue(connectErr.Meta()))) // Connect errors use Meta for both
		} else {
			// Non-Connect errors (network, timeout, etc.)
			httpStatus = 500 // Internal Server Error
			message = err.Error()

			errorObj := rt.NewObject()
			must(rt, errorObj.Set("message", rt.ToValue(message)))
			// No details for non-Connect errors

			must(rt, responseObject.Set("message", errorObj))
			must(rt, responseObject.Set("status", rt.ToValue(httpStatus)))
			must(rt, responseObject.Set("headers", rt.ToValue(map[string]string{})))
			must(rt, responseObject.Set("trailers", rt.ToValue(map[string]string{})))
		}

		// Record error metrics
		if c.metrics != nil {
			tags := c.createMetricTags(method, connParams.Protocol, connParams.ContentType)
			tags.Type = "unary"
			c.metrics.recordUnaryRequest(c.vu.Context(), c.vu, requestDuration, reqSize, 0, tags, err)
		}

		return responseObject, nil // Return response object instead of error for k6
	}

	// Marshal the dynamic response back to a JS-friendly format
	responseJSON, err := protojson.Marshal(resp.Msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal dynamic response to JSON: %w", err)
	}

	// Calculate response size
	if responseJSON != nil {
		respSize = int64(len(responseJSON))
	}

	// Create a message object from the JSON response
	messageVal, err := rt.RunString("(" + string(responseJSON) + ")")
	if err != nil {
		return nil, err
	}

	must(rt, responseObject.Set("message", messageVal))
	must(rt, responseObject.Set("status", rt.ToValue(200))) // HTTP OK status for successful RPC
	must(rt, responseObject.Set("headers", rt.ToValue(resp.Header())))
	must(rt, responseObject.Set("trailers", rt.ToValue(resp.Trailer())))

	// Record successful unary request metrics
	if c.metrics != nil {
		tags := c.createMetricTags(method, connParams.Protocol, connParams.ContentType)
		tags.Type = "unary"
		c.metrics.recordUnaryRequest(c.vu.Context(), c.vu, requestDuration, reqSize, respSize, tags, nil)
	}

	return responseObject, nil
}

// rpcResult holds the raw result of an RPC call without sobek objects
type rpcResult struct {
	responseJSON []byte
	httpStatus   int
	headers      map[string][]string
	trailers     map[string][]string
	err          error
	connectErr   *connect.Error
	reqSize      int64
	respSize     int64
	duration     time.Duration
}

// AsyncInvoke creates and calls a unary RPC by fully qualified method name asynchronously
func (c *Client) AsyncInvoke(
	method string,
	req sobek.Value,
	params sobek.Value,
) (*sobek.Promise, error) {
	rt := c.vu.Runtime()
	promise, resolve, _ := rt.NewPromise()

	// IMPORTANT: Extract all data from sobek Values BEFORE spawning goroutine
	// The sobek runtime is NOT thread-safe and cannot be accessed from other goroutines

	// Pre-process parameters in the main VU goroutine
	state := c.vu.State()
	if state == nil {
		return nil, common.NewInitContextError("invoking a ConnectRPC method in the init context is not supported")
	}

	method = sanitizeMethodName(method)

	methodDesc, err := c.getMethodDescriptor(method)
	if err != nil {
		return nil, err
	}

	p, err := newCallParams(c.vu, params)
	if err != nil {
		return nil, fmt.Errorf("invalid connectrpc.invoke() parameters: %w", err)
	}

	// Marshal the request to JSON in the main goroutine
	reqJSON, err := req.ToObject(rt).MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request object: %w", err)
	}

	// Set tags for metrics
	p.SetSystemTags(state, c.addr, method)

	// Store connection params protocol and content type for metrics (accessed in goroutine)
	connParams := c.connectParams

	callback := c.vu.RegisterCallback()
	go func() {
		// Do the RPC call in the goroutine without touching the runtime
		result := c.doUnaryRPC(method, methodDesc, reqJSON, p)

		// Record metrics in the goroutine (doesn't touch runtime)
		if c.metrics != nil {
			tags := c.createMetricTags(method, connParams.Protocol, connParams.ContentType)
			tags.Type = "unary"
			c.metrics.recordUnaryRequest(c.vu.Context(), c.vu, result.duration, result.reqSize, result.respSize, tags, result.err)
		}

		// Convert the raw result to a sobek object in the callback (main goroutine)
		callback(func() error {
			responseObj := c.convertRPCResultToObject(result)

			if result.err != nil && result.connectErr == nil {
				// For non-Connect errors, we still return the response object (k6 pattern)
				// but we could also reject the promise
				return resolve(responseObj)
			}

			return resolve(responseObj)
		})
	}()

	return promise, nil
}

// Close will close the client HTTP connection
func (c *Client) Close() error {
	if c.httpClient == nil {
		return nil
	}

	// Close idle connections
	c.httpClient.CloseIdleConnections()
	c.httpClient = nil

	return nil
}

// doUnaryRPC performs the actual RPC call without touching the sobek runtime
// This method is safe to call from a goroutine
func (c *Client) doUnaryRPC(
	method string,
	methodDesc protoreflect.MethodDescriptor,
	reqJSON []byte,
	p *callParams,
) *rpcResult {
	result := &rpcResult{
		reqSize: int64(len(reqJSON)),
	}

	// Get or create HTTP client based on connection strategy
	var httpClient *http.Client
	var err error

	if c.connectionStrategy == "per-call" {
		httpClient, err = c.createHTTPClient(c.connectParams, c.addr)
		if err != nil {
			result.err = fmt.Errorf("failed to create HTTP client for per-call strategy: %w", err)
			result.httpStatus = 500
			return result
		}
		defer httpClient.CloseIdleConnections()
	} else if c.connectionStrategy == "per-iteration" {
		state := c.vu.State()
		currentIterationID := state.Iteration
		if c.lastIterationID != currentIterationID {
			if c.httpClient != nil {
				c.httpClient.CloseIdleConnections()
			}
			httpClient, err = c.createHTTPClient(c.connectParams, c.addr)
			if err != nil {
				result.err = fmt.Errorf("failed to create HTTP client for per-iteration strategy: %w", err)
				result.httpStatus = 500
				return result
			}
			c.httpClient = httpClient
			c.lastIterationID = currentIterationID
		} else {
			httpClient = c.httpClient
		}
	} else {
		httpClient = c.httpClient
	}

	// Prepare the dynamic request message from JSON
	requestMessage := dynamicpb.NewMessage(methodDesc.Input())
	if err := protojson.Unmarshal(reqJSON, requestMessage); err != nil {
		result.err = fmt.Errorf("failed to unmarshal JSON into dynamic protobuf message: %w", err)
		result.httpStatus = 500
		return result
	}

	// Prepare client options
	clientOptions := []connect.ClientOption{
		connect.WithSchema(methodDesc),
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

	// Add protocol-specific options
	connParams := c.connectParams
	switch connParams.Protocol {
	case "grpc":
		clientOptions = append(clientOptions, connect.WithGRPC())
	case "grpc-web":
		clientOptions = append(clientOptions, connect.WithGRPCWeb())
	case "connect":
		// Default, no additional option needed
	}

	// Add JSON codec if needed
	if connParams.ContentType == "application/json" {
		clientOptions = append(clientOptions, connect.WithProtoJSON())
	}

	// Create client
	procedureString := method
	url := c.baseURL + procedureString
	dynamicClient := connect.NewClient[dynamicpb.Message, dynamicpb.Message](
		httpClient,
		url,
		clientOptions...,
	)

	connectReq := connect.NewRequest(requestMessage)

	// Set connection-level headers
	if connParams.Headers != nil {
		for key, value := range connParams.Headers {
			connectReq.Header().Set(key, value)
		}
	}

	// Set call-level headers (can override connection-level)
	for key, value := range p.Metadata {
		connectReq.Header().Set(key, value)
	}

	// Make the call with timeout
	var ctx context.Context
	var cancel context.CancelFunc
	callTimeout := time.Duration(0)
	if p.Timeout != nil {
		callTimeout = *p.Timeout
	}

	if callTimeout > 0 {
		ctx, cancel = context.WithTimeout(c.vu.Context(), callTimeout)
		defer cancel()
	} else {
		ctx = c.vu.Context()
	}

	// Record start time
	requestStart := time.Now()
	resp, err := dynamicClient.CallUnary(ctx, connectReq)
	result.duration = time.Since(requestStart)

	if err != nil {
		result.err = err
		// Check if it's a Connect error
		var connectErr *connect.Error
		if errors.As(err, &connectErr) {
			result.connectErr = connectErr
			result.httpStatus = connectCodeToHTTPStatus(connectErr.Code())
			result.headers = connectErr.Meta()
			result.trailers = connectErr.Meta()
		} else {
			// Non-Connect error (network, timeout, etc.)
			result.httpStatus = 500
			result.headers = make(map[string][]string)
			result.trailers = make(map[string][]string)
		}
		return result
	}

	// Marshal successful response
	responseJSON, err := protojson.Marshal(resp.Msg)
	if err != nil {
		result.err = fmt.Errorf("failed to marshal dynamic response to JSON: %w", err)
		result.httpStatus = 500
		return result
	}

	result.responseJSON = responseJSON
	result.respSize = int64(len(responseJSON))
	result.httpStatus = 200
	result.headers = resp.Header()
	result.trailers = resp.Trailer()

	return result
}

// convertRPCResultToObject converts an rpcResult to a sobek Object
// This method must be called from the main VU goroutine as it accesses the runtime
func (c *Client) convertRPCResultToObject(result *rpcResult) *sobek.Object {
	rt := c.vu.Runtime()
	responseObject := rt.NewObject()

	if result.err != nil {
		// Handle error case
		var message string
		if result.connectErr != nil {
			// Connect RPC error
			message = result.connectErr.Error()

			errorObj := rt.NewObject()
			must(rt, errorObj.Set("code", rt.ToValue(result.connectErr.Code().String())))
			must(rt, errorObj.Set("message", rt.ToValue(message)))

			// Serialize error details
			details := result.connectErr.Details()
			serializedDetails := make([]map[string]interface{}, len(details))
			for i, detail := range details {
				serializedDetail := make(map[string]interface{})
				serializedDetail["type"] = detail.Type()
				serializedDetail["bytes"] = detail.Bytes()

				if value, err := detail.Value(); err == nil {
					serializedDetail["value"] = value
				}

				serializedDetails[i] = serializedDetail
			}
			must(rt, errorObj.Set("details", rt.ToValue(serializedDetails)))

			must(rt, responseObject.Set("message", errorObj))
		} else {
			// Non-Connect error
			message = result.err.Error()

			errorObj := rt.NewObject()
			must(rt, errorObj.Set("message", rt.ToValue(message)))

			must(rt, responseObject.Set("message", errorObj))
		}

		must(rt, responseObject.Set("status", rt.ToValue(result.httpStatus)))
		must(rt, responseObject.Set("headers", rt.ToValue(result.headers)))
		must(rt, responseObject.Set("trailers", rt.ToValue(result.trailers)))

		return responseObject
	}

	// Success case
	messageVal, err := rt.RunString("(" + string(result.responseJSON) + ")")
	if err != nil {
		// If we can't parse the JSON, create an error response
		errorObj := rt.NewObject()
		must(rt, errorObj.Set("message", rt.ToValue(fmt.Sprintf("Failed to parse response JSON: %v", err))))

		must(rt, responseObject.Set("message", errorObj))
		must(rt, responseObject.Set("status", rt.ToValue(500)))
		must(rt, responseObject.Set("headers", rt.ToValue(result.headers)))
		must(rt, responseObject.Set("trailers", rt.ToValue(result.trailers)))

		return responseObject
	}

	must(rt, responseObject.Set("message", messageVal))
	must(rt, responseObject.Set("status", rt.ToValue(result.httpStatus)))
	must(rt, responseObject.Set("headers", rt.ToValue(result.headers)))
	must(rt, responseObject.Set("trailers", rt.ToValue(result.trailers)))

	return responseObject
}

// MethodInfo holds information on any parsed method descriptors that can be used by the Sobek VM
type MethodInfo struct {
	Package        string
	Service        string
	FullMethod     string
	IsClientStream bool `json:"isClientStream"`
	IsServerStream bool `json:"isServerStream"`
}

// getMethodDescriptor sanitizes and gets ConnectRPC method descriptor or an error if not found
func (c *Client) getMethodDescriptor(method string) (protoreflect.MethodDescriptor, error) {
	method = sanitizeMethodName(method)

	if method == "" {
		return nil, errors.New("method to invoke cannot be empty")
	}

	return globalProtoRegistry.getMethodDescriptor(method)
}

// extractMethodInfo extracts service and procedure names from a method path
func extractMethodInfo(method string) (service, procedure string) {
	// Method format: "/package.service/procedure" or "/service/procedure"
	method = strings.TrimPrefix(method, "/")
	parts := strings.Split(method, "/")
	if len(parts) >= 2 {
		service = parts[0]
		procedure = parts[1]
	}
	return service, procedure
}

// createMetricTags creates standardized tags for metrics
func (c *Client) createMetricTags(method, protocol, contentType string) MetricTags {
	service, procedure := extractMethodInfo(method)
	return MetricTags{
		Method:      method,
		Service:     service,
		Procedure:   procedure,
		Protocol:    protocol,
		ContentType: contentType,
	}
}

// connectCodeToHTTPStatus converts Connect error codes to HTTP status codes
// Based on the Connect protocol specification
func connectCodeToHTTPStatus(code connect.Code) int {
	switch code {
	case 0: // OK
		return 200
	case connect.CodeCanceled:
		return 408 // Request Timeout
	case connect.CodeUnknown:
		return 500 // Internal Server Error
	case connect.CodeInvalidArgument:
		return 400 // Bad Request
	case connect.CodeDeadlineExceeded:
		return 504 // Gateway Timeout
	case connect.CodeNotFound:
		return 404 // Not Found
	case connect.CodeAlreadyExists:
		return 409 // Conflict
	case connect.CodePermissionDenied:
		return 403 // Forbidden
	case connect.CodeResourceExhausted:
		return 429 // Too Many Requests
	case connect.CodeFailedPrecondition:
		return 412 // Precondition Failed
	case connect.CodeAborted:
		return 409 // Conflict
	case connect.CodeOutOfRange:
		return 400 // Bad Request
	case connect.CodeUnimplemented:
		return 501 // Not Implemented
	case connect.CodeInternal:
		return 500 // Internal Server Error
	case connect.CodeUnavailable:
		return 503 // Service Unavailable
	case connect.CodeDataLoss:
		return 500 // Internal Server Error
	case connect.CodeUnauthenticated:
		return 401 // Unauthorized
	default:
		return 500 // Internal Server Error
	}
}

// TLS helper functions (adapted from gRPC extension)

func decryptPrivateKey(key, password []byte) ([]byte, error) {
	block, _ := pem.Decode(key)
	if block == nil {
		return nil, errors.New("failed to decode PEM key")
	}

	blockType := block.Type
	if blockType == "ENCRYPTED PRIVATE KEY" {
		return nil, errors.New("encrypted pkcs8 formatted key is not supported")
	}

	decryptedKey, err := x509.DecryptPEMBlock(block, password) //nolint:staticcheck
	if err != nil {
		return nil, err
	}
	key = pem.EncodeToMemory(&pem.Block{
		Type:  blockType,
		Bytes: decryptedKey,
	})
	return key, nil
}

func buildTLSConfig(parentConfig *tls.Config, certificate, key []byte, caCertificates [][]byte) (*tls.Config, error) {
	var cp *x509.CertPool
	if len(caCertificates) > 0 {
		cp, _ = x509.SystemCertPool()
		for i, caCert := range caCertificates {
			if ok := cp.AppendCertsFromPEM(caCert); !ok {
				return nil, fmt.Errorf("failed to append ca certificate [%d] from PEM", i)
			}
		}
	}

	//nolint:gosec
	tlsCfg := &tls.Config{
		CipherSuites:       parentConfig.CipherSuites,
		InsecureSkipVerify: parentConfig.InsecureSkipVerify,
		MinVersion:         parentConfig.MinVersion,
		MaxVersion:         parentConfig.MaxVersion,
		Renegotiation:      parentConfig.Renegotiation,
		RootCAs:            cp,
	}
	if len(certificate) > 0 && len(key) > 0 {
		cert, err := tls.X509KeyPair(certificate, key)
		if err != nil {
			return nil, fmt.Errorf("failed to append certificate from PEM: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return tlsCfg, nil
}

func buildTLSConfigFromMap(parentConfig *tls.Config, tlsConfigMap map[string]interface{}) (*tls.Config, error) {
	var cert, key, pass []byte
	var ca [][]byte
	var err error

	// Handle insecureSkipVerify option
	if insecure, ok := tlsConfigMap["insecureSkipVerify"].(bool); ok {
		parentConfig.InsecureSkipVerify = insecure
	}

	if certstr, ok := tlsConfigMap["cert"].(string); ok {
		cert = []byte(certstr)
	}
	if keystr, ok := tlsConfigMap["key"].(string); ok {
		key = []byte(keystr)
	}
	if passwordStr, ok := tlsConfigMap["password"].(string); ok {
		pass = []byte(passwordStr)
		if len(pass) > 0 {
			if key, err = decryptPrivateKey(key, pass); err != nil {
				return nil, err
			}
		}
	}
	if cas, ok := tlsConfigMap["cacerts"]; ok {
		var caCertsArray []interface{}
		if caCertsArray, ok = cas.([]interface{}); ok {
			ca = make([][]byte, len(caCertsArray))
			for i, entry := range caCertsArray {
				var entryStr string
				if entryStr, ok = entry.(string); ok {
					ca[i] = []byte(entryStr)
				}
			}
		} else if caCertStr, caCertStrOk := cas.(string); caCertStrOk {
			ca = [][]byte{[]byte(caCertStr)}
		}
	}
	return buildTLSConfig(parentConfig, cert, key, ca)
}

// connectionTrackingTransport wraps http.RoundTripper to track connection reuse
type connectionTrackingTransport struct {
	base    http.RoundTripper
	client  *Client
	baseURL string
}

func (t *connectionTrackingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var handshakeStart time.Time
	var connectionRecorded bool

	// Add httptrace to detect new connections
	trace := &httptrace.ClientTrace{
		ConnectStart: func(network, addr string) {
			handshakeStart = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			if !connectionRecorded && t.client.metrics != nil {
				handshakeDuration := time.Since(handshakeStart)
				t.client.metrics.recordHTTPConnection(
					t.client.vu.Context(),
					t.client.vu,
					t.baseURL,
					true, // new connection
					handshakeDuration,
				)
				connectionRecorded = true
			}
		},
		GotConn: func(info httptrace.GotConnInfo) {
			if !connectionRecorded && t.client.metrics != nil {
				if info.Reused {
					// Connection was reused
					t.client.metrics.recordHTTPConnection(
						t.client.vu.Context(),
						t.client.vu,
						t.baseURL,
						false, // reused connection
						0,
					)
					connectionRecorded = true
				}
				// Note: We don't record "new" connections here because ConnectDone should handle that
				// The only case where GotConn with !info.Reused happens without ConnectStart/ConnectDone
				// is when using existing idle connections, but those are still "reuse" from a logical perspective
			}
		},
	}

	// Add trace to request context
	ctx := httptrace.WithClientTrace(req.Context(), trace)
	req = req.WithContext(ctx)

	return t.base.RoundTrip(req)
}
