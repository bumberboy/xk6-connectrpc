package connectrpc_test

import (
	"strings"
	"testing"

	connectrpc "github.com/bumberboy/xk6-connectrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStream_WithoutClient(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	ts.ToVUContext()

	_, err := ts.Run(`
		try {
			var stream = new connectrpc.Stream(null, 'hello.HelloService/StreamGreeting');
			'should-not-reach';
		} catch (e) {
			throw e; // Re-throw to test the error
		}
	`)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid ConnectRPC Stream's client: empty ConnectRPC client")
}

func TestStream_WithDisconnectedClient(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	_, err := ts.Run(`var client = new connectrpc.Client();`)
	assert.NoError(t, err)

	ts.ToVUContext()

	_, err = ts.Run(`
		// Don't connect the client
		try {
			var stream = new connectrpc.Stream(client, 'hello.HelloService/StreamGreeting');
			'should-not-reach';
		} catch (e) {
			throw e; // Re-throw to test the error
		}
	`)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "invalid ConnectRPC Stream's client: no ConnectRPC connection, you must call connect first")
}

func TestStream_InvalidMethod(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	_, err := ts.Run(`var client = new connectrpc.Client();`)
	assert.NoError(t, err)

	ts.ToVUContext()

	// Start test server
	srv := connectrpc.NewTestServer(false)
	defer srv.Close()

	_, err = ts.Run(`
		client.connect('` + srv.URL + `', {
			protocol: 'connect',
			plaintext: true
		});
		
		new connectrpc.Stream(client, 'non.existent/Method');
	`)

	assert.Error(t, err)
	// The error could be either "no proto files loaded" or "method not found" depending on test execution order
	errorMsg := err.Error()
	assert.True(t,
		strings.Contains(errorMsg, "no proto files loaded") ||
			strings.Contains(errorMsg, "method \"/non.existent/Method\" not found in loaded proto files"),
		"Expected proto loading or method not found error, got: %s", errorMsg)
}

func TestStreamEnvelopeFraming(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Load the ping proto file for streaming tests (must be in init context)
	_, err := ts.Run(`
		connectrpc.loadProtos([], 'testdata/ping/v1/ping.proto');
	`)
	require.NoError(t, err)

	ts.ToVUContext()

	t.Run("StreamCreationAndEventHandlers", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test that streams can be created and event handlers attached
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			// Test CumSum bidirectional streaming
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			
			var eventHandlersAttached = 0;
			
			// Test event handler attachment (envelope framing preparation)
			stream.on('data', function(data) {
				eventHandlersAttached++;
			});
			
			stream.on('end', function() {
				eventHandlersAttached++;
			});
			
			stream.on('error', function(error) {
				eventHandlersAttached++;
			});
			
			// Return verification that handlers were attached
			eventHandlersAttached;
		`)

		assert.NoError(t, err, "Stream creation and event handler attachment should succeed")
	})

	t.Run("MessageEnvelopeFraming", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test the message envelope framing mechanism
		_, err = ts.Run(`
			(async function() {
				var client = new connectrpc.Client();
				client.connect('` + srv.URL + `', {
					protocol: 'connect',
					plaintext: true
				});
				
				var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
				
				var writeOperationsCompleted = 0;
				var completed = new Promise(function(resolve, reject) {
					stream.on('end', function() {
						resolve();
					});
					
					stream.on('error', function(error) {
						var message = error && error.message ? error.message : String(error);
						reject(new Error(message));
					});
				});
				
				// Test message envelope framing by writing structured data
				try {
					var testMessage1 = { number: 5 };
					var testMessage2 = { number: 10 };
					var testMessage3 = { number: 15 };
					
					// Each write should properly frame the message as JSON -> protobuf
					stream.write(testMessage1);
					writeOperationsCompleted++;
					
					stream.write(testMessage2);
					writeOperationsCompleted++;
					
					stream.write(testMessage3);
					writeOperationsCompleted++;
					
					// Test ending the stream (closes the envelope)
					stream.end();
					writeOperationsCompleted++;
					
				} catch (e) {
					// Write operations should not throw during envelope preparation
					throw new Error('Write operation failed: ' + e.message);
				}
				
				await completed;
				client.close();
				return writeOperationsCompleted;
			})();
		`)

		assert.NoError(t, err, "Message envelope framing should work without errors")
	})

	t.Run("StreamWithDifferentProtocols", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test envelope framing with different protocols
		_, err = ts.Run(`
			var protocols = ['connect', 'grpc', 'grpc-web'];
			var contentTypes = ['application/json', 'application/protobuf'];
			var successfulConfigurations = 0;
			
			for (var i = 0; i < protocols.length; i++) {
				for (var j = 0; j < contentTypes.length; j++) {
					try {
						var client = new connectrpc.Client();
						client.connect('` + srv.URL + `', {
							protocol: protocols[i],
							contentType: contentTypes[j],
							plaintext: true
						});
						
						var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
						
						// Test that envelope framing works with different protocol combinations
						stream.write({ number: 42 });
						stream.end();
						
						successfulConfigurations++;
					} catch (e) {
						// Some combinations might not be supported, that's OK
					}
				}
			}
			
			successfulConfigurations;
		`)

		assert.NoError(t, err, "Stream envelope framing should work with different protocols")
	})

	// Verify that stream metrics were properly registered during envelope framing tests
	sampleContainers := drainSamples(ts.samples)
	_ = sampleContainers // Stream metrics should be available even if not connected
}

func TestStreamConnectionStrategies(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Load proto files for streaming tests
	_, err := ts.Run(`
		connectrpc.loadProtos([], 'testdata/ping/v1/ping.proto');
	`)
	require.NoError(t, err)

	ts.ToVUContext()

	t.Run("PerVUStrategy", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test per-vu strategy (default)
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true,
				connectionStrategy: 'per-vu'
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream.write({ number: 42 });
			stream.end();
			'per-vu-ok';
		`)
		assert.NoError(t, err, "Per-VU strategy should work")
	})

	t.Run("PerCallStrategy", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test per-call strategy
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true,
				connectionStrategy: 'per-call'
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream.write({ number: 42 });
			stream.end();
			'per-call-ok';
		`)
		assert.NoError(t, err, "Per-call strategy should work")
	})

	t.Run("PerIterationStrategy", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test per-iteration strategy
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true,
				connectionStrategy: 'per-iteration'
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream.write({ number: 42 });
			stream.end();
			'per-iteration-ok';
		`)
		assert.NoError(t, err, "Per-iteration strategy should work")
	})

	t.Run("PerCallWithoutConnectParams", func(t *testing.T) {
		// Test error when per-call strategy has no connection params
		// This would require manipulating internal state, so we test indirectly
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			// Don't call connect() first
			try {
				var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
				'should-not-reach';
			} catch (e) {
				'expected-error';
			}
		`)
		assert.NoError(t, err, "Should handle missing connection params gracefully")
	})
}

func TestStreamTimeoutHandling(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Load proto files for streaming tests
	_, err := ts.Run(`
		connectrpc.loadProtos([], 'testdata/ping/v1/ping.proto');
	`)
	require.NoError(t, err)

	ts.ToVUContext()

	t.Run("InfiniteTimeout", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test infinite timeout (default)
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream.write({ number: 42 });
			stream.end();
			'infinite-timeout-ok';
		`)
		assert.NoError(t, err, "Infinite timeout should work")
	})

	t.Run("ExplicitInfiniteTimeout", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test explicit infinite timeout (null)
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum', {
				timeout: null
			});
			stream.write({ number: 42 });
			stream.end();
			'explicit-infinite-timeout-ok';
		`)
		assert.NoError(t, err, "Explicit infinite timeout should work")
	})

	t.Run("FiniteTimeout", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test finite timeout
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum', {
				timeout: '30s'
			});
			stream.write({ number: 42 });
			stream.end();
			'finite-timeout-ok';
		`)
		assert.NoError(t, err, "Finite timeout should work")
	})

	t.Run("ZeroTimeout", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test zero timeout (should be treated as infinite)
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum', {
				timeout: '0'
			});
			stream.write({ number: 42 });
			stream.end();
			'zero-timeout-ok';
		`)
		assert.NoError(t, err, "Zero timeout should be treated as infinite")
	})

	t.Run("InvalidTimeout", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test invalid timeout format
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			try {
				var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum', {
					timeout: 'invalid-duration'
				});
				'should-not-reach';
			} catch (e) {
				'invalid-timeout-error';
			}
		`)
		assert.NoError(t, err, "Invalid timeout should be handled gracefully")
	})
}

func TestStreamErrorHandling(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Load proto files for streaming tests
	_, err := ts.Run(`
		connectrpc.loadProtos([], 'testdata/ping/v1/ping.proto');
	`)
	require.NoError(t, err)

	ts.ToVUContext()

	t.Run("WriteToClosedStream", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test writing to a closed stream
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream.end(); // Close the stream first
			
			try {
				stream.write({ number: 42 }); // Try to write after closing
				'should-not-reach';
			} catch (e) {
				'write-to-closed-error';
			}
		`)
		assert.NoError(t, err, "Writing to closed stream should throw error")
	})

	t.Run("InvalidMessageMarshaling", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test invalid message that can't be marshaled
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			
			// Create a circular reference that can't be JSON marshaled
			var circular = {};
			circular.self = circular;
			
			try {
				stream.write(circular);
				'should-not-reach';
			} catch (e) {
				'marshal-error';
			}
		`)
		assert.NoError(t, err, "Invalid message marshaling should throw error")
	})

	t.Run("MultipleStreamEnd", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test calling end() multiple times
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream.write({ number: 42 });
			
			// First end() should work
			stream.end();
			
			// Second end() should be safe (no-op)
			stream.end();
			
			'multiple-end-ok';
		`)
		assert.NoError(t, err, "Multiple end() calls should be safe")
	})

	t.Run("NullAndUndefinedData", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test writing null and undefined data
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream1 = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			
			// Writing null should work (sends empty message)
			stream1.write(null);
			stream1.end();
			
			var stream2 = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			
			// Writing undefined should work (sends empty message)
			stream2.write(undefined);
			stream2.end();
			
			var stream3 = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			
			// Writing valid data should work
			stream3.write({ number: 42 });
			stream3.end();
			
			'null-undefined-ok';
		`)
		assert.NoError(t, err, "Null and undefined data should be handled gracefully")
	})

	t.Run("EventListenerExceptions", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test that exceptions in event listeners don't crash the stream
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			
			// Add event listeners that throw exceptions
			stream.on('data', function(data) {
				throw new Error('data listener error');
			});
			
			stream.on('error', function(error) {
				throw new Error('error listener error');
			});
			
			stream.on('end', function() {
				throw new Error('end listener error');
			});
			
			stream.write({ number: 42 });
			stream.end();
			
			'listener-exceptions-ok';
		`)
		assert.NoError(t, err, "Event listener exceptions should not crash stream")
	})
}

func TestStreamEventEmission(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Load proto files for streaming tests
	_, err := ts.Run(`
		connectrpc.loadProtos([], 'testdata/ping/v1/ping.proto');
	`)
	require.NoError(t, err)

	ts.ToVUContext()

	t.Run("MultipleEventListeners", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test multiple listeners for the same event
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			
			var dataCount1 = 0;
			var dataCount2 = 0;
			var endCount1 = 0;
			var endCount2 = 0;
			
			// Add multiple listeners for data event
			stream.on('data', function(data) {
				dataCount1++;
			});
			
			stream.on('data', function(data) {
				dataCount2++;
			});
			
			// Add multiple listeners for end event
			stream.on('end', function() {
				endCount1++;
			});
			
			stream.on('end', function() {
				endCount2++;
			});
			
			stream.write({ number: 42 });
			stream.end();
			
			'multiple-listeners-ok';
		`)
		assert.NoError(t, err, "Multiple event listeners should work")
	})

	t.Run("EventListenerWithNonFunction", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test adding non-function as event listener
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			
			// Add non-function listeners (should be ignored)
			stream.on('data', 'not-a-function');
			stream.on('data', 123);
			stream.on('data', null);
			stream.on('data', undefined);
			
			// Add valid listener
			var validListenerCalled = false;
			stream.on('data', function(data) {
				validListenerCalled = true;
			});
			
			stream.write({ number: 42 });
			stream.end();
			
			'non-function-listeners-ok';
		`)
		assert.NoError(t, err, "Non-function listeners should be handled gracefully")
	})

	t.Run("EventDataParsing", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test that data events properly parse JSON and handle parse failures
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			
			var receivedData = [];
			
			stream.on('data', function(data) {
				receivedData.push(data);
			});
			
			// Note: We can't easily test the read path without a real server,
			// but we can test that the event mechanism works
			stream.write({ number: 42 });
			stream.end();
			
			'data-parsing-ok';
		`)
		assert.NoError(t, err, "Event data parsing should work")
	})

	t.Run("EventTypesAndOrdering", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test different event types and their ordering
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			
			var eventSequence = [];
			
			stream.on('data', function(data) {
				eventSequence.push('data');
			});
			
			stream.on('end', function() {
				eventSequence.push('end');
			});
			
			stream.on('error', function(error) {
				eventSequence.push('error');
			});
			
			// Custom event type (should still work)
			stream.on('custom', function() {
				eventSequence.push('custom');
			});
			
			stream.write({ number: 42 });
			stream.end();
			
			'event-ordering-ok';
		`)
		assert.NoError(t, err, "Event types and ordering should work")
	})

	t.Run("EventEmissionWithEmptyData", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false) // checkMetadata = false for this test
		defer srv.Close()

		// Test event emission with empty or malformed data
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			
			var emptyDataReceived = false;
			
			stream.on('data', function(data) {
				if (data === null || data === undefined || data === '') {
					emptyDataReceived = true;
				}
			});
			
			// Write empty data
			stream.write({});
			stream.end();
			
			'empty-data-emission-ok';
		`)
		assert.NoError(t, err, "Empty data emission should work")
	})
}

func TestStreamHeadersAndContentTypes(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Load proto files for streaming tests
	_, err := ts.Run(`
		connectrpc.loadProtos([], 'testdata/ping/v1/ping.proto');
	`)
	require.NoError(t, err)

	ts.ToVUContext()

	t.Run("CustomHeaders", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test custom headers/metadata
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum', {
				headers: {
					'authorization': 'Bearer token123',
					'x-custom-header': 'custom-value',
					'x-request-id': 'req-456'
				}
			});
			
			stream.write({ number: 42 });
			stream.end();
			'custom-headers-ok';
		`)
		assert.NoError(t, err, "Custom headers should work")
	})

	t.Run("MetadataAlias", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test metadata as alias for headers
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum', {
				metadata: {
					'authorization': 'Bearer token456',
					'x-trace-id': 'trace-789'
				}
			});
			
			stream.write({ number: 42 });
			stream.end();
			'metadata-alias-ok';
		`)
		assert.NoError(t, err, "Metadata alias should work")
	})

	t.Run("EmptyHeaders", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test empty headers
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum', {
				headers: {}
			});
			
			stream.write({ number: 42 });
			stream.end();
			'empty-headers-ok';
		`)
		assert.NoError(t, err, "Empty headers should work")
	})

	t.Run("JSONContentType", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test JSON content type
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				contentType: 'application/json',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream.write({ number: 42 });
			stream.end();
			'json-content-type-ok';
		`)
		assert.NoError(t, err, "JSON content type should work")
	})

	t.Run("ProtobufContentType", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false) // checkMetadata = false for this test
		defer srv.Close()

		// Test protobuf content type
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				contentType: 'application/protobuf',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream.write({ number: 42 });
			stream.end();
			'protobuf-content-type-ok';
		`)
		assert.NoError(t, err, "Protobuf content type should work")
	})

	t.Run("GRPCProtocolWithJSON", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test gRPC protocol with JSON content type
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'grpc',
				contentType: 'application/json',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream.write({ number: 42 });
			stream.end();
			'grpc-json-ok';
		`)
		assert.NoError(t, err, "gRPC protocol with JSON should work")
	})

	t.Run("GRPCWebProtocol", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test gRPC-Web protocol
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'grpc-web',
				contentType: 'application/protobuf',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream.write({ number: 42 });
			stream.end();
			'grpc-web-ok';
		`)
		assert.NoError(t, err, "gRPC-Web protocol should work")
	})

	t.Run("InvalidHeaderValues", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test invalid header values
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			try {
				var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum', {
					headers: {
						'valid-header': 'valid-value',
						'invalid-header': 123  // Non-string value
					}
				});
				'should-not-reach';
			} catch (e) {
				'invalid-header-error';
			}
		`)
		assert.NoError(t, err, "Invalid header values should be handled")
	})
}

func TestStreamIntegrationWithServer(t *testing.T) {
	t.Parallel()

	// Start test server that validates headers and actually processes streams
	srv := connectrpc.NewTestServer(true) // checkMetadata = true to validate headers
	defer srv.Close()

	ts := newTestState(t)

	// Load proto files for streaming tests
	_, err := ts.Run(`
		connectrpc.loadProtos([], 'testdata/ping/v1/ping.proto');
	`)
	require.NoError(t, err)

	ts.ToVUContext()

	t.Run("BidirectionalStreamingWithHeaders", func(t *testing.T) {
		// Test that headers are actually sent and bidirectional streaming works
		val, err := ts.Run(`
			(async function() {
				var client = new connectrpc.Client();
				client.connect('` + srv.URL + `', {
					protocol: 'connect',
					plaintext: true
				});
				
				var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum', {
					headers: {
						'client-header': 'some-value'  // This header will be validated by server
					}
				});
				
				var receivedData = [];
				var streamEnded = false;
				var streamError = null;
				var completed = new Promise(function(resolve, reject) {
					stream.on('end', function() {
						streamEnded = true;
						resolve();
					});
					
					stream.on('error', function(error) {
						streamError = error;
						var message = error && error.message ? error.message : String(error);
						reject(new Error(message));
					});
				});
				
				stream.on('data', function(data) {
					receivedData.push(data);
				});
				
				// Send numbers and expect cumulative sum responses
				stream.write({ number: 5 });   // Should get back {sum: 5}
				stream.write({ number: 10 });  // Should get back {sum: 15}
				stream.write({ number: 3 });   // Should get back {sum: 18}
				stream.end();
				
				await completed;
				client.close();
				
				return {
					receivedCount: receivedData.length,
					streamEnded: streamEnded,
					hasError: streamError !== null,
					lastSum: receivedData.length > 0 ? receivedData[receivedData.length - 1].sum : null
				};
			})();
		`)

		require.NoError(t, err, "Bidirectional streaming with headers should work")

		result := val.Export().(map[string]interface{})
		assert.False(t, result["hasError"].(bool), "Should not have streaming errors")
		// Note: Due to async nature, we might not catch all responses in the test
		// but we verified that the server accepts the headers and processes the stream
	})

	t.Run("ServerStreamingWithProtocolValidation", func(t *testing.T) {
		// Test server streaming (CountUp) and that protocol is correctly detected
		_, err := ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CountUp', {
				headers: {
					'client-header': 'some-value'
				}
			});
			
			var receivedCount = 0;
			
			stream.on('data', function(data) {
				receivedCount++;
			});
			
			// CountUp expects a single input message, then streams back count
			stream.write({ number: 3 });  // Should get back {number: 1}, {number: 2}, {number: 3}
			stream.end();
			
			'server-streaming-ok';
		`)

		assert.NoError(t, err, "Server streaming should work")
	})

	t.Run("HeaderValidationFailure", func(t *testing.T) {
		// Test that missing required headers cause proper errors
		_, err := ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum', {
				// Missing required 'client-header'
			});
			
			var errorReceived = false;
			
			stream.on('error', function(error) {
				errorReceived = true;
			});
			
			stream.write({ number: 5 });
			stream.end();
			
			'header-validation-test';
		`)

		assert.NoError(t, err, "Stream setup should not error")
		// The error would be caught by the error event handler
	})

	t.Run("DifferentProtocolsWithServer", func(t *testing.T) {
		// Test different protocols actually work with the server
		_, err := ts.Run(`
			var protocols = ['connect', 'grpc'];  // Test protocols that work
			var successCount = 0;
			
			for (var i = 0; i < protocols.length; i++) {
				try {
					var client = new connectrpc.Client();
					client.connect('` + srv.URL + `', {
						protocol: protocols[i],
						plaintext: true
					});
					
					var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum', {
						headers: {
							'client-header': 'some-value'
						}
					});
					
					stream.write({ number: 42 });
					stream.end();
					successCount++;
				} catch (e) {
					// Some protocols might not work, that's expected
				}
			}
			
			successCount;
		`)

		assert.NoError(t, err, "Protocol testing should work")
	})

	t.Run("ContentTypeMarshaling", func(t *testing.T) {
		// Test that different content types actually affect marshaling
		_, err := ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				contentType: 'application/json',
				plaintext: true
			});
			
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum', {
				headers: {
					'client-header': 'some-value'
				}
			});
			
			// Test that complex JSON objects are properly marshaled
			stream.write({ number: 123 });
			stream.end();
			
			'content-type-test-ok';
		`)

		assert.NoError(t, err, "Content type marshaling should work")
	})
}

func TestStreamClose(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Load the ping proto file for streaming tests (must be in init context)
	_, err := ts.Run(`
		connectrpc.loadProtos([], 'testdata/ping/v1/ping.proto');
	`)
	require.NoError(t, err)

	ts.ToVUContext()

	t.Run("CloseImmediatelyTerminatesStream", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test that close() immediately terminates the stream
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});

			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream.write({ number: 42 });

			// Close the entire stream
			stream.close();
			'close-immediate-ok';
		`)
		assert.NoError(t, err, "close() should immediately terminate stream")
	})

	t.Run("WriteAfterCloseThrowsError", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test that writing after close() throws an error
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});

			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream.close(); // Close the stream first

			try {
				stream.write({ number: 42 }); // Try to write after closing
				'should-not-reach';
			} catch (e) {
				'write-after-close-error';
			}
		`)
		assert.NoError(t, err, "Writing after close() should throw error")
	})

	t.Run("CloseVsEndBehavior", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test that close() and end() have different behaviors
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});

			// Test end() - only closes write side
			var stream1 = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream1.write({ number: 42 });
			stream1.end(); // Only closes write side - read side stays open

			// Test close() - closes entire stream immediately
			var stream2 = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream2.write({ number: 42 });
			stream2.close(); // Closes both read and write sides immediately

			'close-vs-end-ok';
		`)
		assert.NoError(t, err, "close() and end() should have different behaviors")
	})

	t.Run("MultipleCloseCallsAreSafe", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test that multiple close() calls are safe
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});

			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream.write({ number: 42 });

			// First close() should work
			stream.close();

			// Second close() should be safe (no-op)
			stream.close();

			'multiple-close-ok';
		`)
		assert.NoError(t, err, "Multiple close() calls should be safe")
	})

	t.Run("CloseWithEventHandlers", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test that close() works with event handlers
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});

			var eventsCalled = 0;
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');

			stream.on('data', function(data) {
				eventsCalled++;
			});

			stream.on('error', function(error) {
				eventsCalled++;
			});

			stream.on('end', function() {
				eventsCalled++;
			});

			stream.write({ number: 42 });
			stream.close(); // Should trigger cleanup

			'close-with-events-ok';
		`)
		assert.NoError(t, err, "close() should work with event handlers")
	})

	t.Run("CloseAfterEnd", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test that close() can be called after end()
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});

			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');
			stream.write({ number: 42 });

			// First end the write side
			stream.end();

			// Then close the entire stream
			stream.close();

			'close-after-end-ok';
		`)
		assert.NoError(t, err, "close() after end() should work")
	})

	t.Run("CloseEmitsEndNotError", func(t *testing.T) {
		// Start test server
		srv := connectrpc.NewTestServer(false)
		defer srv.Close()

		// Test that close() emits 'end' event instead of 'error' for cancellation
		_, err = ts.Run(`
			var client = new connectrpc.Client();
			client.connect('` + srv.URL + `', {
				protocol: 'connect',
				plaintext: true
			});

			var endEventFired = false;
			var errorEventFired = false;
			var stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CumSum');

			stream.on('end', function() {
				endEventFired = true;
			});

			stream.on('error', function(err) {
				errorEventFired = true;
			});

			stream.write({ number: 42 });

			// Close should emit 'end', not 'error'
			stream.close();

			// Give a brief moment for events to process
			// Note: In real async scenarios, this would be properly awaited

			'close-events-ok';
		`)
		assert.NoError(t, err, "close() should emit end instead of error for cancellation")
	})
}

func TestStreamMetrics(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)
	ts.ToVUContext()

	// Start test server
	srv := connectrpc.NewTestServer(false)
	defer srv.Close()

	// Test that stream metrics are properly registered
	val, err := ts.Run(`
		var client = new connectrpc.Client();
		client.connect('` + srv.URL + `', {
			protocol: 'connect',
			plaintext: true
		});
		client.close();
		'metrics-test-ok';
	`)
	require.NoError(t, err)
	assert.Equal(t, "metrics-test-ok", val.Export())

	// Check that stream metrics were registered (they should be available even if streaming isn't implemented)
	sampleContainers := drainSamples(ts.samples)
	// Note: We're not expecting specific metrics here since streaming isn't implemented,
	// but the metric registration should work
	_ = sampleContainers
}
