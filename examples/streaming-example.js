import connectrpc from 'k6/x/connectrpc';
import { check } from 'k6';

// Load proto files globally in init context
connectrpc.loadProtos([], 'clown.proto');

export const options = {
    vus: 2,
    duration: '20s',
    thresholds: {
        'connectrpc_stream_duration{procedure:ClownGreetStream}': ['avg<2000'],
        'connectrpc_stream_msgs_sent{procedure:ClownGreetStream}': ['count>0'],
        'connectrpc_stream_msgs_received{procedure:ClownGreetStream}': ['count>0'],
        'connectrpc_stream_errors{procedure:ClownGreetStream}': ['count<3'],
    },
};

export default async function () {
    const client = new connectrpc.Client();
    
    // Connect with streaming-optimized settings
    const connected = client.connect('https://your-service.com');

    if (!connected) {
        throw new Error('Failed to connect to service');
    }

    // Run streaming test
    await testBidirectionalStream(client);
    
    client.close();
}

function testBidirectionalStream(client) {
    return new Promise((resolve, reject) => {
        const stream = new connectrpc.Stream(client, '/clown.v1.ClownService/ClownGreetStream');
        
        const names = ['Alice', 'Bob', 'Charlie'];
        let responseCount = 0;

        stream.on('data', function (response) {
            console.log(`Received greeting: ${response.greeting}`);
            responseCount++;
            check(response, {
                'greeting is not empty': (r) => !!r.greeting,
                'greeting is string': (r) => typeof r.greeting === 'string',
            });
        });

        stream.on('error', function (err) {
            console.error(`Stream error: ${err.code} - ${err.message}`);
            reject(err);
        });

        stream.on('end', function () {
            console.log(`Stream ended, received ${responseCount} responses`);
            check(responseCount, {
                'received all expected responses': (count) => count === names.length,
            });
            resolve();
        });

        // Send names to the stream
        names.forEach((name) => {
            stream.write({ name: name });
        });

        // Close the client side of the stream
        stream.end();
    });
} 