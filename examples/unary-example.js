import connectrpc from 'k6/x/connectrpc';
import { check } from 'k6';

// Load proto files globally in init context
connectrpc.loadProtos([], 'clown.proto');

export const options = {
    vus: 5,
    duration: '30s',
    thresholds: {
        'connectrpc_req_duration{procedure:TellJoke}': ['avg<1000'],
        'connectrpc_reqs{procedure:TellJoke}': ['count>0'],
        'connectrpc_req_errors{procedure:TellJoke}': ['count<5'],
    },
};

export default function () {
    // Create client
    const client = new connectrpc.Client();
    
    // Connect to service
    const connected = client.connect('https://your-service.com', {
        protocol: 'connect',
        contentType: 'application/json',
        timeout: '30s'
    });

    if (!connected) {
        throw new Error('Failed to connect to service');
    }

    // Make unary call
    const response = client.invoke('/clown.v1.ClownService/TellJoke', {
        audienceType: 'programmers'
    });

    // Validate response
    check(response, {
        'status is 200': (r) => r.status === 200,
        'has joke': (r) => !!r.message && !!r.message.joke,
    });

    client.close();
} 