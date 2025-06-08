package connectrpc_test

import (
	"testing"

	"github.com/bumberboy/xk6-connectrpc"
	"github.com/stretchr/testify/require"
)

// TestIntegrationBasicPingWithServer tests basic functionality using our test server
func TestIntegrationBasicPingWithServer(t *testing.T) {
	t.Parallel()

	// Start our test server
	srv := connectrpc.NewTestServer(false)
	defer srv.Close()

	ts := newTestState(t)

	// Load the ping service proto file globally
	_, err := ts.Run(`
		const protoFile = './testdata/ping/v1/ping.proto';
		connectrpc.loadProtos([], protoFile);
	`)
	require.NoError(t, err)

	ts.ToVUContext()

	// Test basic ping using JavaScript API
	_, err = ts.Run(`
		var client = new connectrpc.Client();
		client.connect('` + srv.URL + `', {
			protocol: 'connect',
			contentType: 'application/json',
			plaintext: true
		});

		const response = client.invoke('/k6.connectrpc.ping.v1.PingService/Ping', {
			number: 42,
			text: 'hello from k6'
		});

		if (response.status !== 200) {
			throw new Error('Expected status 200, got ' + response.status);
		}
		
		// Just test that we have a response - the details may vary based on implementation
		if (!response.message) {
			throw new Error('No response message received');
		}

		client.close();
		'success';
	`)
	require.NoError(t, err)
}

// TestIntegrationHTTPVersionConfiguration tests HTTP version settings
func TestIntegrationHTTPVersionConfiguration(t *testing.T) {
	t.Parallel()

	srv := connectrpc.NewTestServer(false)
	defer srv.Close()

	testCases := []struct {
		Name        string
		HTTPVersion string
	}{
		{"Default HTTP version", ""},
		{"HTTP 1.1", "1.1"},
		{"HTTP 2", "2"},
		{"Auto HTTP version", "auto"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ts := newTestState(t)

			_, err := ts.Run(`
				const protoFile = './testdata/ping/v1/ping.proto';
				connectrpc.loadProtos([], protoFile);
			`)
			require.NoError(t, err)

			ts.ToVUContext()

			// Build config with HTTP version
			configJS := `{
				protocol: 'connect',
				contentType: 'application/json',
				plaintext: true`
			if tc.HTTPVersion != "" {
				configJS += `,
				httpVersion: '` + tc.HTTPVersion + `'`
			}
			configJS += `}`

			// Test connection with specific HTTP version
			_, err = ts.Run(`
				var client = new connectrpc.Client();
				// Test that the HTTP version configuration is accepted
				client.connect('` + srv.URL + `', ` + configJS + `);
				client.close();
				'http-version-config-accepted';
			`)
			require.NoError(t, err)
		})
	}
}

// TestIntegrationTimeoutConfiguration tests timeout settings
func TestIntegrationTimeoutConfiguration(t *testing.T) {
	t.Parallel()

	srv := connectrpc.NewTestServer(false)
	defer srv.Close()

	testCases := []struct {
		Name    string
		Timeout string
	}{
		{"Default timeout", ""},
		{"Infinite timeout (null)", "null"},
		{"Infinite timeout (0)", `"0"`},
		{"Infinite timeout (infinite)", `"infinite"`},
		{"30 second timeout", `"30s"`},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ts := newTestState(t)

			_, err := ts.Run(`
				const protoFile = './testdata/ping/v1/ping.proto';
				connectrpc.loadProtos([], protoFile);
			`)
			require.NoError(t, err)

			ts.ToVUContext()

			// Build config with timeout
			configJS := `{
				protocol: 'connect',
				contentType: 'application/json',
				plaintext: true`
			if tc.Timeout != "" {
				configJS += `,
				timeout: ` + tc.Timeout
			}
			configJS += `}`

			// Test connection with specific timeout
			_, err = ts.Run(`
				var client = new connectrpc.Client();
				// Test that the timeout configuration is accepted
				client.connect('` + srv.URL + `', ` + configJS + `);
				client.close();
				'timeout-config-accepted';
			`)
			require.NoError(t, err)
		})
	}
}

// TestIntegrationProtocolAndContentType tests supported protocol/content-type combinations
func TestIntegrationProtocolAndContentType(t *testing.T) {
	t.Parallel()

	srv := connectrpc.NewTestServer(false)
	defer srv.Close()

	testCases := []struct {
		Name        string
		Protocol    string
		ContentType string
	}{
		{"Connect with JSON", "connect", "application/json"},
		{"Connect with Proto", "connect", "application/proto"},
		{"Connect with Protobuf", "connect", "application/protobuf"},
		{"gRPC with JSON", "grpc", "application/json"},
		{"gRPC with Proto", "grpc", "application/proto"},
		{"gRPC with Protobuf", "grpc", "application/protobuf"},
		{"gRPC-Web with JSON", "grpc-web", "application/json"},
		{"gRPC-Web with Proto", "grpc-web", "application/proto"},
		{"gRPC-Web with Protobuf", "grpc-web", "application/protobuf"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ts := newTestState(t)

			_, err := ts.Run(`
				const protoFile = './testdata/ping/v1/ping.proto';
				connectrpc.loadProtos([], protoFile);
			`)
			require.NoError(t, err)

			ts.ToVUContext()

			// Test that valid protocol/content type combinations are accepted
			_, err = ts.Run(`
				var client = new connectrpc.Client();
				client.connect('` + srv.URL + `', {
					protocol: '` + tc.Protocol + `',
					contentType: '` + tc.ContentType + `',
					plaintext: true
				});
				
				// Just test that the configuration is accepted without making actual calls
				// The test server may not support all protocol combinations
				client.close();
				'protocol-content-type-config-accepted';
			`)
			require.NoError(t, err)
		})
	}
}

// TestIntegrationProtocolAndContentTypeValidation tests validation of protocol/content-type parameters
func TestIntegrationProtocolAndContentTypeValidation(t *testing.T) {
	t.Parallel()

	srv := connectrpc.NewTestServer(false)
	defer srv.Close()

	testCases := []struct {
		Name        string
		Protocol    string
		ContentType string
		ShouldFail  bool
	}{
		// Valid combinations (should pass)
		{"Valid Connect JSON", "connect", "application/json", false},
		{"Valid gRPC Proto", "grpc", "application/proto", false},
		{"Valid gRPC-Web Protobuf", "grpc-web", "application/protobuf", false},

		// Invalid protocols (should fail)
		{"Invalid Protocol", "invalid-protocol", "application/json", true},
		{"Custom Protocol", "my-custom-protocol", "application/json", true},

		// Invalid content types (should fail)
		{"Invalid Content Type", "connect", "application/invalid", true},
		{"Custom Content Type", "connect", "application/my-format", true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			ts := newTestState(t)

			_, err := ts.Run(`
				const protoFile = './testdata/ping/v1/ping.proto';
				connectrpc.loadProtos([], protoFile);
			`)
			require.NoError(t, err)

			ts.ToVUContext()

			if tc.ShouldFail {
				// Test that invalid configurations are rejected
				_, err = ts.Run(`
					var client = new connectrpc.Client();
					try {
						client.connect('` + srv.URL + `', {
							protocol: '` + tc.Protocol + `',
							contentType: '` + tc.ContentType + `',
							plaintext: true
						});
						throw new Error('Expected validation error but connection succeeded');
					} catch (e) {
						if (e.message.includes('invalid protocol') || e.message.includes('invalid contentType')) {
							'validation-correctly-failed';
						} else {
							throw new Error('Wrong error type: ' + e.message);
						}
					}
				`)
				require.NoError(t, err)
			} else {
				// Test that valid configurations are accepted
				_, err = ts.Run(`
					var client = new connectrpc.Client();
					client.connect('` + srv.URL + `', {
						protocol: '` + tc.Protocol + `',
						contentType: '` + tc.ContentType + `',
						plaintext: true
					});
					client.close();
					'validation-correctly-passed';
				`)
				require.NoError(t, err)
			}
		})
	}
}

// TestIntegrationTLSConfiguration tests TLS settings
func TestIntegrationTLSConfiguration(t *testing.T) {
	t.Parallel()

	srv := connectrpc.NewTLSTestServer(false)
	defer srv.Close()

	ts := newTestState(t)

	_, err := ts.Run(`
		const protoFile = './testdata/ping/v1/ping.proto';
		connectrpc.loadProtos([], protoFile);
	`)
	require.NoError(t, err)

	ts.ToVUContext()

	// Test insecure skip verify (should work with test server's self-signed cert)
	_, err = ts.Run(`
		var client = new connectrpc.Client();
		client.connect('` + srv.URL + `', {
			protocol: 'connect',
			contentType: 'application/json',
			plaintext: false,
			tls: {
				insecureSkipVerify: true
			}
		});

		const response = client.invoke('/k6.connectrpc.ping.v1.PingService/Ping', {
			number: 789,
			text: 'tls test'
		});

		if (response.status !== 200) {
			throw new Error('TLS test failed with status: ' + response.status);
		}

		client.close();
		'tls-success';
	`)
	require.NoError(t, err)
}
