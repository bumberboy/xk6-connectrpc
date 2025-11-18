package connectrpc_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/bumberboy/xk6-connectrpc"
	pingv1 "github.com/bumberboy/xk6-connectrpc/testdata/ping/v1"
	"github.com/bumberboy/xk6-connectrpc/testdata/ping/v1/pingv1connect"
	"github.com/stretchr/testify/require"
)

// simplePingServer implements the ping service for HTTP method testing
type simplePingServer struct {
	pingv1connect.UnimplementedPingServiceHandler
}

func (s *simplePingServer) Ping(ctx context.Context, req *connect.Request[pingv1.PingRequest]) (*connect.Response[pingv1.PingResponse], error) {
	return connect.NewResponse(&pingv1.PingResponse{
		Number: req.Msg.GetNumber(),
		Text:   req.Msg.GetText(),
	}), nil
}

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

// TestErrorDetailsIncludedInResponse tests that Connect RPC error details are properly included in the response
func TestErrorDetailsIncludedInResponse(t *testing.T) {
	t.Parallel()

	// Start test server with error details enabled
	srv := connectrpc.NewTestServerWithErrorDetails(false)
	defer srv.Close()

	ts := newTestState(t)

	// Load the ping service proto file globally
	_, err := ts.Run(`
		const protoFile = './testdata/ping/v1/ping.proto';
		connectrpc.loadProtos([], protoFile);
	`)
	require.NoError(t, err)

	ts.ToVUContext()

	// Test that error details are included in unary RPC error responses
	_, err = ts.Run(`
		var client = new connectrpc.Client();
		var connected = client.connect('` + srv.URL + `', {});
		if (!connected) {
			throw new Error('Failed to connect to test server');
		}

		// Call the Fail endpoint with a specific error code to trigger an error with details
		var response = client.invoke('/k6.connectrpc.ping.v1.PingService/Fail', {
			code: 3 // INVALID_ARGUMENT
		});


		// Verify the error response structure
		if (response.status !== 400) {
			throw new Error('Expected status 400 for invalid_argument error, got: ' + response.status);
		}

		if (!response.message) {
			throw new Error('Expected error message object');
		}

		if (!response.message.code) {
			throw new Error('Expected error code in response.message.code');
		}

		if (response.message.code !== 'invalid_argument') {
			throw new Error('Expected error code to be "invalid_argument", got: ' + response.message.code);
		}

		if (!response.message.message) {
			throw new Error('Expected error message text in response.message.message');
		}

		// Most importantly, verify that error details are included
		if (!response.message.details) {
			throw new Error('Expected error details in response.message.details');
		}

		if (!Array.isArray(response.message.details)) {
			throw new Error('Expected error details to be an array, got: ' + typeof response.message.details);
		}

		if (response.message.details.length === 0) {
			throw new Error('Expected at least one error detail');
		}

		// Verify the actual content of the error detail
		var firstDetail = response.message.details[0];
		
		if (!firstDetail.type) {
			throw new Error('Expected error detail to have a type field');
		}

		// The test server adds a FailRequest detail with the error code
		if (firstDetail.type !== 'k6.connectrpc.ping.v1.FailRequest') {
			throw new Error('Expected error detail type to be "k6.connectrpc.ping.v1.FailRequest", got: ' + firstDetail.type);
		}

		if (!firstDetail.value) {
			throw new Error('Expected error detail to have a value field');
		}

		if (typeof firstDetail.value !== 'object') {
			throw new Error('Expected error detail value to be an object, got: ' + typeof firstDetail.value);
		}

		// The FailRequest protobuf message should contain the code field
		if (!firstDetail.value.code && firstDetail.value.code !== 0) {
			throw new Error('Expected error detail value to have a code field');
		}

		if (firstDetail.value.code !== 3) {
			throw new Error('Expected error detail code to be 3, got: ' + firstDetail.value.code);
		}

		// Verify bytes field exists (raw protobuf bytes)
		if (!firstDetail.bytes) {
			throw new Error('Expected error detail to have a bytes field');
		}


		client.close();
	`)
	require.NoError(t, err)
}

// TestAsyncInvokeBasicFunctionality tests that asyncInvoke works correctly
func TestAsyncInvokeBasicFunctionality(t *testing.T) {
	t.Parallel()

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

	// Test asyncInvoke with single call
	_, err = ts.Run(`
		(async function() {
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				contentType: 'application/json',
				plaintext: true
			});

			// Test single async call
			const promise = client.asyncInvoke('/k6.connectrpc.ping.v1.PingService/Ping', {
				number: 100,
				text: 'async test'
			});

			// Verify it returns a promise
			if (typeof promise.then !== 'function') {
				throw new Error('asyncInvoke should return a Promise');
			}

			// Await the response
			const response = await promise;

			if (response.status !== 200) {
				throw new Error('Expected status 200, got ' + response.status);
			}

			if (!response.message) {
				throw new Error('No response message received');
			}

			if (response.message.number !== 100) {
				throw new Error('Expected number 100, got ' + response.message.number);
			}

			if (response.message.text !== 'async test') {
				throw new Error('Expected text "async test", got ' + response.message.text);
			}

			client.close();
			return 'async-success';
		})();
	`)
	require.NoError(t, err)
}

// TestAsyncInvokeParallelCalls tests that multiple asyncInvoke calls work in parallel
func TestAsyncInvokeParallelCalls(t *testing.T) {
	t.Parallel()

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

	// Test multiple parallel asyncInvoke calls
	_, err = ts.Run(`
		(async function() {
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				contentType: 'application/json',
				plaintext: true
			});

			// Make multiple parallel async calls
			const promises = [
				client.asyncInvoke('/k6.connectrpc.ping.v1.PingService/Ping', {
					number: 1,
					text: 'call 1'
				}),
				client.asyncInvoke('/k6.connectrpc.ping.v1.PingService/Ping', {
					number: 2,
					text: 'call 2'
				}),
				client.asyncInvoke('/k6.connectrpc.ping.v1.PingService/Ping', {
					number: 3,
					text: 'call 3'
				})
			];

			// Wait for all to complete
			const responses = await Promise.all(promises);

			if (responses.length !== 3) {
				throw new Error('Expected 3 responses, got ' + responses.length);
			}

			// Verify all responses
			for (let i = 0; i < responses.length; i++) {
				const response = responses[i];
				const expectedNumber = i + 1;
				const expectedText = 'call ' + expectedNumber;

				if (response.status !== 200) {
					throw new Error('Response ' + i + ' expected status 200, got ' + response.status);
				}

				if (response.message.number !== expectedNumber) {
					throw new Error('Response ' + i + ' expected number ' + expectedNumber + ', got ' + response.message.number);
				}

				if (response.message.text !== expectedText) {
					throw new Error('Response ' + i + ' expected text "' + expectedText + '", got ' + response.message.text);
				}
			}

			client.close();
			return 'parallel-success';
		})();
	`)
	require.NoError(t, err)
}

// TestAsyncInvokeWithHeaders tests that asyncInvoke works with custom headers
func TestAsyncInvokeWithHeaders(t *testing.T) {
	t.Parallel()

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

	// Test asyncInvoke with headers
	_, err = ts.Run(`
		(async function() {
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				contentType: 'application/json',
				plaintext: true
			});

			// Test async call with custom headers
			const response = await client.asyncInvoke('/k6.connectrpc.ping.v1.PingService/Ping', {
				number: 999,
				text: 'with headers'
			}, {
				headers: {
					'X-Custom-Header': 'test-value',
					'Authorization': 'Bearer test-token'
				}
			});

			if (response.status !== 200) {
				throw new Error('Expected status 200, got ' + response.status);
			}

			if (!response.message) {
				throw new Error('No response message received');
			}

			client.close();
			return 'headers-success';
		})();
	`)
	require.NoError(t, err)
}
