package connectrpc_test

import (
	"testing"

	connectrpc "github.com/bumberboy/xk6-connectrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleCreation(t *testing.T) {
	t.Parallel()

	module := connectrpc.New()
	require.NotNil(t, module)
	assert.IsType(t, &connectrpc.RootModule{}, module)
}

func TestModuleInstance(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Test that the module was properly registered
	val, err := ts.Run("typeof connectrpc")
	require.NoError(t, err)
	assert.Equal(t, "object", val.Export())

	// Test Client constructor is available
	val, err = ts.Run("typeof connectrpc.Client")
	require.NoError(t, err)
	assert.Equal(t, "function", val.Export())

	// Test Stream constructor is available
	val, err = ts.Run("typeof connectrpc.Stream")
	require.NoError(t, err)
	assert.Equal(t, "function", val.Export())

	// Test constants are available
	constants := []string{
		"PROTOCOL_CONNECT",
		"PROTOCOL_GRPC",
		"PROTOCOL_GRPC_WEB",
		"CONTENT_TYPE_JSON",
		"CONTENT_TYPE_PROTOBUF",
		"CONTENT_TYPE_GRPC",
		"CONTENT_TYPE_GRPC_WEB",
		"StatusOK",
		"StatusBadRequest",
		"StatusUnauthorized",
		"StatusForbidden",
		"StatusNotFound",
		"StatusInternalServerError",
		"StatusNotImplemented",
		"StatusServiceUnavailable",
	}

	for _, constant := range constants {
		val, err = ts.Run("typeof connectrpc." + constant)
		require.NoError(t, err, "constant %s should be defined", constant)
		assert.Contains(t, []string{"string", "number"}, val.Export(), "constant %s should be string or number", constant)
	}
}

func TestClientCreation(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Test client creation
	val, err := ts.Run(`
		var client = new connectrpc.Client();
		typeof client;
	`)
	require.NoError(t, err)
	assert.Equal(t, "object", val.Export())

	// Test client methods exist
	methods := []string{"connect", "invoke", "asyncInvoke", "close"}

	for _, method := range methods {
		val, err = ts.Run("typeof client." + method)
		require.NoError(t, err, "method %s should exist", method)
		assert.Equal(t, "function", val.Export(), "method %s should be a function", method)
	}
}

func TestModuleExports(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	module := connectrpc.New().NewModuleInstance(ts.VU)

	exports := module.Exports()
	require.NotNil(t, exports.Named)

	// Check that all expected exports are present
	expectedExports := []string{
		"Client",
		"Stream",
		"PROTOCOL_CONNECT",
		"PROTOCOL_GRPC",
		"PROTOCOL_GRPC_WEB",
		"CONTENT_TYPE_JSON",
		"CONTENT_TYPE_PROTOBUF",
		"CONTENT_TYPE_GRPC",
		"CONTENT_TYPE_GRPC_WEB",
		"StatusOK",
		"StatusBadRequest",
		"StatusUnauthorized",
		"StatusForbidden",
		"StatusNotFound",
		"StatusInternalServerError",
		"StatusNotImplemented",
		"StatusServiceUnavailable",
	}

	for _, exportName := range expectedExports {
		_, exists := exports.Named[exportName]
		assert.True(t, exists, "export %s should be present", exportName)
	}
}

func TestSanitizeMethodName(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Initialize client in init context
	_, err := ts.Run(`var client = new connectrpc.Client();`)
	require.NoError(t, err)

	ts.ToVUContext()

	// Test method name sanitization through the module
	val, err := ts.Run(`
		// Test that method names are properly handled
		// This is indirectly tested through error messages when methods don't exist
		client.connect('example.com:443', { protocol: 'connect', plaintext: true });
		
		try {
			client.invoke('TestService/TestMethod', {});
			'should-not-reach';
		} catch (e) {
			client.close();
			// Check for either error message depending on global registry state
			(e.message.includes('no proto files loaded') || e.message.includes('not found in loaded proto files')) ? 'method-name-sanitized' : 'unexpected-error';
		}
	`)
	require.NoError(t, err)
	assert.Equal(t, "method-name-sanitized", val.Export())
}
