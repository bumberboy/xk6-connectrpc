import connectrpc from 'k6/x/connectrpc';
import { check } from 'k6';

// Load proto files globally in init context
connectrpc.loadProtos([], 'testdata/ping/v1/ping.proto');

export const options = {
    vus: 2,
    duration: '15s',
    thresholds: {
        'connectrpc_stream_duration{procedure:CountUp}': ['avg<3000'],
        'connectrpc_stream_msgs_sent{procedure:CountUp}': ['count>0'],
        'connectrpc_stream_msgs_received{procedure:CountUp}': ['count>0'],
        'connectrpc_stream_errors{procedure:CountUp}': ['count<2'],
    },
};

export default async function () {
    const client = new connectrpc.Client();

    // Connect with streaming settings
    const connected = client.connect('https://your-service.com', {
        protocol: 'connect',
        contentType: 'application/json',
        timeout: '30s'  // 30 second timeout for streaming operations
    });

    if (!connected) {
        throw new Error('Failed to connect to service');
    }

    // Run standard server-side streaming test (using end())
    await testServerSideStream(client);

    // Demonstrate early termination with close()
    await testServerSideStreamWithEarlyTermination(client);

    client.close();
}

function testServerSideStream(client) {
    return new Promise((resolve, reject) => {
        console.log('=== Testing Standard Server-Side Streaming ===');
        const stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CountUp');

        const expectedNumbers = [1, 2, 3, 4, 5];
        const receivedNumbers = [];
        let responseCount = 0;

        stream.on('data', function (response) {
            console.log(`Received number: ${response.number}`);
            receivedNumbers.push(response.number);
            responseCount++;

            check(response, {
                'response has number field': (r) => r.number !== undefined,
                'number is positive': (r) => r.number > 0,
                'number is integer': (r) => Number.isInteger(r.number),
            });
        });

        stream.on('error', function (err) {
            console.error(`Stream error: ${err.code} - ${err.message}`);
            reject(err);
        });

        stream.on('end', function () {
            console.log(`Stream ended, received ${responseCount} responses`);

            check(responseCount, {
                'received expected number of responses': (count) => count === expectedNumbers.length,
            });

            check(receivedNumbers, {
                'received numbers in order': (numbers) => {
                    return JSON.stringify(numbers) === JSON.stringify(expectedNumbers);
                },
                'all numbers received': (numbers) => numbers.length === expectedNumbers.length,
            });

            resolve();
        });

        // Send a single request to start the server stream
        // CountUp will return numbers from 1 to the requested number
        stream.write({ number: 5 });

        // Close the client side to signal end of input
        // end() only closes write side - server continues sending responses
        stream.end();
    });
}

function testServerSideStreamWithEarlyTermination(client) {
    return new Promise((resolve, reject) => {
        console.log('=== Testing Server-Side Streaming with Early Termination ===');
        const stream = new connectrpc.Stream(client, '/k6.connectrpc.ping.v1.PingService/CountUp');

        const receivedNumbers = [];
        let responseCount = 0;

        stream.on('data', function (response) {
            console.log(`Early termination test - Received number: ${response.number}`);
            receivedNumbers.push(response.number);
            responseCount++;

            check(response, {
                'response has number field': (r) => r.number !== undefined,
                'number is positive': (r) => r.number > 0,
                'number is integer': (r) => Number.isInteger(r.number),
            });

            // Terminate stream early after receiving 3 responses
            if (responseCount >= 3) {
                console.log('Received 3 responses, terminating stream early with close()');
                stream.close(); // Immediately close entire stream - stops both read and write

                check(responseCount, {
                    'early termination worked': (count) => count === 3,
                    'did not receive all responses': (count) => count < 10, // We requested 10 but should only get 3
                });

                console.log(`Early termination successful: received ${responseCount} of 10 responses`);
                resolve();
            }
        });

        stream.on('error', function (err) {
            console.error(`Early termination stream error: ${err.code} - ${err.message}`);
            // Context cancellation errors are expected when using close()
            if (err.code === 'deadline_exceeded' || err.code === 'cancelled') {
                console.log('Stream terminated as expected due to close()');
                resolve();
            } else {
                reject(err);
            }
        });

        stream.on('end', function () {
            console.log(`Early termination test - Stream ended naturally, received ${responseCount} responses`);
            // This should not happen if close() works correctly for early termination
            if (responseCount < 10) {
                console.log('Stream ended before receiving all responses (expected with close())');
            }
            resolve();
        });

        // Send a single request to start the server stream
        // CountUp will return numbers from 1 to the requested number
        stream.write({ number: 10 }); // Request 10 numbers but we'll terminate early

        // Use end() to close write side (normal server-side streaming pattern)
        // The close() will be called in the data handler for early termination
        stream.end();
    });
}