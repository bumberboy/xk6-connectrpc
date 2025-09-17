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

    // Run server-side streaming test
    await testServerSideStream(client);

    client.close();
}

function testServerSideStream(client) {
    return new Promise((resolve, reject) => {
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
        stream.end();
    });
}