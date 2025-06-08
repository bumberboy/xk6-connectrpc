package connectrpc_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientBasic(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Test client creation in init context
	_, err := ts.Run(`var client = new connectrpc.Client();`)
	require.NoError(t, err)

	ts.ToVUContext()

	// Test basic connection (should succeed)
	_, err = ts.Run(`
		client.connect('example.com:443', {
			protocol: 'connect',
			contentType: 'application/json',
			plaintext: true
		});
		client.close();
		'connected';
	`)
	require.NoError(t, err)
}

func TestClientInvalidHTTPVersion(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	_, err := ts.Run(`var client = new connectrpc.Client();`)
	require.NoError(t, err)

	ts.ToVUContext()

	// Test invalid HTTP version (should fail)
	_, err = ts.Run(`
		client.connect('example.com:443', {
			httpVersion: 'invalid-version'
		});
	`)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid httpVersion")
}

func TestClientInvokeWithoutMethodDescriptor(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	_, err := ts.Run(`var client = new connectrpc.Client();`)
	require.NoError(t, err)

	ts.ToVUContext()

	// Test invoke without loading proto files (should fail)
	_, err = ts.Run(`
		client.connect('example.com:443', {
			protocol: 'connect',
			plaintext: true
		});
		
		client.invoke('hello.HelloService/SayHello', {greeting: 'test'});
	`)
	assert.Error(t, err)
	// The error could be either "no proto files loaded" or "method not found" depending on test execution order
	errorMsg := err.Error()
	assert.True(t,
		strings.Contains(errorMsg, "no proto files loaded") ||
			strings.Contains(errorMsg, "method \"/hello.HelloService/SayHello\" not found in loaded proto files"),
		"Expected proto loading or method not found error, got: %s", errorMsg)
}

func TestClientConnectInInitContext(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Test connecting in init context (should fail)
	_, err := ts.Run(`
		var client = new connectrpc.Client();
		client.connect('example.com:443', {});
	`)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "connecting to a ConnectRPC server in the init context is not supported")
}
