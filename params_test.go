package connectrpc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"gopkg.in/guregu/null.v3"
)

// Helper function to create *time.Duration for tests
func durationPtr(d time.Duration) *time.Duration {
	return &d
}

func TestConnectParamsValidInput(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		Name     string
		JSON     string
		Expected connectParams
	}{
		{
			Name: "Default",
			JSON: `{}`,
			Expected: connectParams{
				IsPlaintext:   false,
				UseReflection: false,
				Timeout:       nil, // Default to infinite timeout
				Protocol:      "connect",
				ContentType:   "application/json",
				HTTPVersion:   "2",
			},
		},
		{
			Name: "Plaintext",
			JSON: `{ plaintext: true }`,
			Expected: connectParams{
				IsPlaintext:   true,
				UseReflection: false,
				Timeout:       nil,
				Protocol:      "connect",
				ContentType:   "application/json",
				HTTPVersion:   "2",
			},
		},
		{
			Name: "Reflection",
			JSON: `{ reflect: true }`,
			Expected: connectParams{
				IsPlaintext:   false,
				UseReflection: true,
				Timeout:       nil,
				Protocol:      "connect",
				ContentType:   "application/json",
				HTTPVersion:   "2",
			},
		},
		{
			Name: "CustomTimeout",
			JSON: `{ timeout: "30s" }`,
			Expected: connectParams{
				IsPlaintext:   false,
				UseReflection: false,
				Timeout:       durationPtr(30 * time.Second),
				Protocol:      "connect",
				ContentType:   "application/json",
				HTTPVersion:   "2",
			},
		},
		{
			Name: "GRPCProtocol",
			JSON: `{ protocol: "grpc" }`,
			Expected: connectParams{
				IsPlaintext:   false,
				UseReflection: false,
				Timeout:       nil,
				Protocol:      "grpc",
				ContentType:   "application/json",
				HTTPVersion:   "2",
			},
		},
		{
			Name: "ProtobufContentType",
			JSON: `{ contentType: "application/protobuf" }`,
			Expected: connectParams{
				IsPlaintext:   false,
				UseReflection: false,
				Timeout:       nil,
				Protocol:      "connect",
				ContentType:   "application/protobuf",
				HTTPVersion:   "2",
			},
		},
		{
			Name: "MaxSizes",
			JSON: `{ maxReceiveSize: 1024, maxSendSize: 512 }`,
			Expected: connectParams{
				IsPlaintext:    false,
				UseReflection:  false,
				Timeout:        nil,
				Protocol:       "connect",
				ContentType:    "application/json",
				HTTPVersion:    "2",
				MaxReceiveSize: 1024,
				MaxSendSize:    512,
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			testRuntime := modulestest.NewRuntime(t)

			val, err := testRuntime.VU.Runtime().RunString("(" + tc.JSON + ")")
			require.NoError(t, err)

			params, err := newConnectParams(testRuntime.VU, val)
			require.NoError(t, err)

			assert.Equal(t, tc.Expected.IsPlaintext, params.IsPlaintext)
			assert.Equal(t, tc.Expected.UseReflection, params.UseReflection)
			assert.Equal(t, tc.Expected.Timeout, params.Timeout)
			assert.Equal(t, tc.Expected.Protocol, params.Protocol)
			assert.Equal(t, tc.Expected.ContentType, params.ContentType)
			assert.Equal(t, tc.Expected.MaxReceiveSize, params.MaxReceiveSize)
			assert.Equal(t, tc.Expected.MaxSendSize, params.MaxSendSize)
		})
	}
}

func TestConnectParamsInvalidInput(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		Name        string
		JSON        string
		ErrContains string
	}{
		{
			Name:        "InvalidTimeout",
			JSON:        `{ timeout: "invalid" }`,
			ErrContains: "invalid timeout value",
		},
		{
			Name:        "InvalidHTTPVersion",
			JSON:        `{ httpVersion: "invalid" }`,
			ErrContains: "invalid httpVersion: invalid",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			testRuntime := modulestest.NewRuntime(t)

			val, err := testRuntime.VU.Runtime().RunString("(" + tc.JSON + ")")
			require.NoError(t, err)

			_, err = newConnectParams(testRuntime.VU, val)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.ErrContains)
		})
	}
}

func TestCallParamsValidInput(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		Name     string
		JSON     string
		Expected callParams
	}{
		{
			Name: "Default",
			JSON: `{}`,
			Expected: callParams{
				Timeout:  nil,
				Metadata: map[string]string{},
			},
		},
		{
			Name: "WithTimeout",
			JSON: `{ timeout: "30s" }`,
			Expected: callParams{
				Timeout:  durationPtr(30 * time.Second),
				Metadata: map[string]string{},
			},
		},
		{
			Name: "WithMetadata",
			JSON: `{ metadata: { "authorization": "Bearer token", "x-custom": "value" } }`,
			Expected: callParams{
				Timeout: nil,
				Metadata: map[string]string{
					"authorization": "Bearer token",
					"x-custom":      "value",
				},
			},
		},
		{
			Name: "WithHeaders",
			JSON: `{ headers: { "content-type": "application/json" } }`,
			Expected: callParams{
				Timeout: nil,
				Metadata: map[string]string{
					"content-type": "application/json",
				},
			},
		},
		{
			Name: "WithDiscardResponse",
			JSON: `{ discardResponse: true }`,
			Expected: callParams{
				Timeout:                nil,
				Metadata:               map[string]string{},
				DiscardResponseMessage: true,
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			testRuntime := modulestest.NewRuntime(t)
			registry := metrics.NewRegistry()

			state := &lib.State{
				Options: lib.Options{
					SystemTags: metrics.NewSystemTagSet(
						metrics.TagName,
						metrics.TagURL,
					),
					UserAgent: null.StringFrom("k6-test"),
				},
				BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
				Tags:           lib.NewVUStateTags(registry.RootTagSet()),
			}

			testRuntime.MoveToVUContext(state)

			val, err := testRuntime.VU.Runtime().RunString("(" + tc.JSON + ")")
			require.NoError(t, err)

			params, err := newCallParams(testRuntime.VU, val)
			require.NoError(t, err)

			assert.Equal(t, tc.Expected.Timeout, params.Timeout)
			assert.Equal(t, tc.Expected.Metadata, params.Metadata)
			assert.Equal(t, tc.Expected.DiscardResponseMessage, params.DiscardResponseMessage)
		})
	}
}

func TestCallParamsInvalidInput(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		Name        string
		JSON        string
		ErrContains string
	}{
		{
			Name:        "InvalidTimeout",
			JSON:        `{ timeout: "invalid" }`,
			ErrContains: "invalid timeout value",
		},
		{
			Name:        "InvalidMetadata",
			JSON:        `{ metadata: "invalid" }`,
			ErrContains: "invalid metadata object",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			testRuntime := modulestest.NewRuntime(t)
			registry := metrics.NewRegistry()

			state := &lib.State{
				Options: lib.Options{
					SystemTags: metrics.NewSystemTagSet(
						metrics.TagName,
						metrics.TagURL,
					),
					UserAgent: null.StringFrom("k6-test"),
				},
				BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
				Tags:           lib.NewVUStateTags(registry.RootTagSet()),
			}

			testRuntime.MoveToVUContext(state)

			val, err := testRuntime.VU.Runtime().RunString("(" + tc.JSON + ")")
			require.NoError(t, err)

			_, err = newCallParams(testRuntime.VU, val)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.ErrContains)
		})
	}
}

func TestCallParamsInInitContext(t *testing.T) {
	t.Parallel()

	testRuntime := modulestest.NewRuntime(t)

	val, err := testRuntime.VU.Runtime().RunString("({})")
	require.NoError(t, err)

	_, err = newCallParams(testRuntime.VU, val)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "getting call parameters in the init context is not supported")
}
