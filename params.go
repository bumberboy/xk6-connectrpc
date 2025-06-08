package connectrpc

import (
	"fmt"
	"time"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"

	"github.com/grafana/sobek"
)

type connectParams struct {
	IsPlaintext        bool
	UseReflection      bool
	Timeout            *time.Duration // Changed to pointer to support nil (infinite timeout)
	MaxReceiveSize     int64
	MaxSendSize        int64
	TLS                map[string]interface{}
	Protocol           string
	ContentType        string
	HTTPVersion        string // New field for HTTP version control
	ConnectionStrategy string // New field for connection reuse strategy
}

type callParams struct {
	Timeout                *time.Duration // Changed to pointer to support nil (infinite timeout)
	DiscardResponseMessage bool
	Metadata               map[string]string
	TagsAndMeta            metrics.TagsAndMeta
}

// newConnectParams creates connection parameters from a sobek.Value
func newConnectParams(vu modules.VU, paramsVal sobek.Value) (*connectParams, error) {
	params := &connectParams{
		IsPlaintext:        false,
		UseReflection:      false,
		Timeout:            nil,                // Default to infinite timeout (protocol compliant)
		Protocol:           "connect",          // Default to Connect protocol
		ContentType:        "application/json", // Default content type
		HTTPVersion:        "2",                // Default to HTTP/2 for best compatibility
		ConnectionStrategy: "per-vu",           // Default to persistent connection per VU
	}

	if paramsVal == nil || sobek.IsUndefined(paramsVal) || sobek.IsNull(paramsVal) {
		return params, nil
	}

	rt := vu.Runtime()
	paramsObj := paramsVal.ToObject(rt)

	for _, k := range paramsObj.Keys() {
		switch k {
		case "plaintext":
			params.IsPlaintext = paramsObj.Get(k).ToBoolean()
		case "reflect":
			params.UseReflection = paramsObj.Get(k).ToBoolean()
		case "timeout":
			timeoutVal := paramsObj.Get(k)
			if sobek.IsNull(timeoutVal) || sobek.IsUndefined(timeoutVal) {
				params.Timeout = nil // Infinite timeout
			} else {
				timeoutStr := timeoutVal.String()
				if timeoutStr == "" || timeoutStr == "0" || timeoutStr == "infinite" {
					params.Timeout = nil // Infinite timeout
				} else {
					timeout, err := time.ParseDuration(timeoutStr)
					if err != nil {
						return nil, fmt.Errorf("invalid timeout value: %w", err)
					}
					params.Timeout = &timeout
				}
			}
		case "maxReceiveSize":
			params.MaxReceiveSize = paramsObj.Get(k).ToInteger()
		case "maxSendSize":
			params.MaxSendSize = paramsObj.Get(k).ToInteger()
		case "tls":
			params.TLS = paramsObj.Get(k).Export().(map[string]interface{})
		case "protocol":
			protocol := paramsObj.Get(k).String()
			// Validate protocol values
			if protocol != "connect" && protocol != "grpc" && protocol != "grpc-web" {
				return nil, fmt.Errorf("invalid protocol: %s. Must be 'connect', 'grpc', or 'grpc-web'", protocol)
			}
			params.Protocol = protocol
		case "contentType":
			contentType := paramsObj.Get(k).String()
			// Validate contentType values
			if contentType != "application/json" && contentType != "application/proto" && contentType != "application/protobuf" {
				return nil, fmt.Errorf("invalid contentType: %s. Must be 'application/json', 'application/proto', or 'application/protobuf'", contentType)
			}
			params.ContentType = contentType
		case "httpVersion":
			httpVersion := paramsObj.Get(k).String()
			if httpVersion != "1.1" && httpVersion != "2" && httpVersion != "auto" {
				return nil, fmt.Errorf("invalid httpVersion: %s. Must be '1.1', '2', or 'auto'", httpVersion)
			}
			params.HTTPVersion = httpVersion
		case "connectionStrategy":
			strategy := paramsObj.Get(k).String()
			if strategy != "per-vu" && strategy != "per-iteration" && strategy != "per-call" {
				return nil, fmt.Errorf("invalid connectionStrategy: %s. Must be 'per-vu', 'per-iteration', or 'per-call'", strategy)
			}
			params.ConnectionStrategy = strategy
		}
	}

	return params, nil
}

// newCallParams creates call parameters from a sobek.Value
func newCallParams(vu modules.VU, paramsVal sobek.Value) (*callParams, error) {
	state := vu.State()
	if state == nil {
		return nil, common.NewInitContextError("getting call parameters in the init context is not supported")
	}

	params := &callParams{
		Timeout:     nil, // Default to infinite timeout (protocol compliant)
		Metadata:    make(map[string]string),
		TagsAndMeta: state.Tags.GetCurrentValues(),
	}

	if paramsVal == nil || sobek.IsUndefined(paramsVal) || sobek.IsNull(paramsVal) {
		return params, nil
	}

	rt := vu.Runtime()
	paramsObj := paramsVal.ToObject(rt)

	for _, k := range paramsObj.Keys() {
		switch k {
		case "headers", "metadata":
			metadata := paramsObj.Get(k)
			if !sobek.IsUndefined(metadata) && !sobek.IsNull(metadata) {
				err := processMetadata(metadata, params.Metadata, rt)
				if err != nil {
					return nil, fmt.Errorf("invalid %s object: %w", k, err)
				}
			}
		case "timeout":
			timeoutVal := paramsObj.Get(k)
			if sobek.IsNull(timeoutVal) || sobek.IsUndefined(timeoutVal) {
				params.Timeout = nil // Infinite timeout
			} else {
				timeoutStr := timeoutVal.String()
				if timeoutStr == "" || timeoutStr == "0" || timeoutStr == "infinite" {
					params.Timeout = nil // Infinite timeout
				} else {
					timeout, err := time.ParseDuration(timeoutStr)
					if err != nil {
						return nil, fmt.Errorf("invalid timeout value: %w", err)
					}
					params.Timeout = &timeout
				}
			}
		case "discardResponse":
			params.DiscardResponseMessage = paramsObj.Get(k).ToBoolean()
		case "tags":
			if err := common.ApplyCustomUserTags(rt, &params.TagsAndMeta, paramsObj.Get(k)); err != nil {
				return nil, fmt.Errorf("invalid tags object: %w", err)
			}
		}
	}

	return params, nil
}

// processMetadata processes metadata/headers from JavaScript object
func processMetadata(metadata sobek.Value, dest map[string]string, rt *sobek.Runtime) error {
	v := metadata.Export()

	rawHeaders, ok := v.(map[string]interface{})
	if !ok {
		return fmt.Errorf("must be an object with key-value pairs")
	}

	for hk, kv := range rawHeaders {
		var val string
		if val, ok = kv.(string); !ok {
			return fmt.Errorf("%q value must be a string", hk)
		}
		dest[hk] = val
	}

	return nil
}

// SetSystemTags sets system tags for metrics
func (p *callParams) SetSystemTags(state *lib.State, addr, method string) {
	if state.Options.SystemTags.Has(metrics.TagURL) {
		p.TagsAndMeta.SetSystemTagOrMeta(metrics.TagURL, fmt.Sprintf("connectrpc://%s%s", addr, method))
	}

	if state.Options.SystemTags.Has(metrics.TagMethod) {
		p.TagsAndMeta.SetSystemTagOrMeta(metrics.TagMethod, method)
	}

	if state.Options.SystemTags.Has(metrics.TagName) {
		p.TagsAndMeta.SetSystemTagOrMeta(metrics.TagName, fmt.Sprintf("connectrpc://%s%s", addr, method))
	}
}
