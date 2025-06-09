import connectrpc from 'k6/x/connectrpc';
import { check, group } from 'k6';
import { ElizaServiceClient } from './gen/connectrpc/eliza/v1/eliza.k6.js';

export let options = {
    vus: 1,
    // duration: '10s',
    iterations: 1,
    thresholds: {
        // Per-procedure request duration thresholds
        'connectrpc_req_duration{procedure:Say}': ['avg<2000'],
        'connectrpc_req_duration{procedure:Converse}': ['avg<3000'],
        'connectrpc_req_duration{procedure:Introduce}': ['avg<2000'],

        // Per-procedure request counts
        'connectrpc_reqs{procedure:Say}': ['count>0'],
        'connectrpc_reqs{procedure:Converse}': ['count>=0'],
        'connectrpc_reqs{procedure:Introduce}': ['count>=0'],

        // Per-procedure stream metrics
        'connectrpc_stream_duration{procedure:Converse}': ['avg>0'],
        'connectrpc_stream_duration{procedure:Introduce}': ['avg>0'],
        'connectrpc_stream_msgs_sent{procedure:Converse}': ['count>0'],
        'connectrpc_stream_msgs_received{procedure:Converse}': ['count>0'],
        'connectrpc_stream_msgs_received{procedure:Introduce}': ['count>0'],

        // Per-procedure error rates
        'connectrpc_req_errors{procedure:Say}': ['count<5'],
        'connectrpc_stream_errors{procedure:Converse}': ['count<5'],
        'connectrpc_stream_errors{procedure:Introduce}': ['count<5'],

        // HTTP connection tracking
        'connectrpc_http_connections_new': ['count>0'],
        'connectrpc_http_handshake_duration': ['avg<500'],
    },
};

// Create client once per VU (outside iteration function)
let client;
let elizaClient;

export function setup() {
    console.log('Setup phase completed - testing Eliza psychotherapist service');
}

// The default export function must be 'async' to use 'await' for the stream tests
export default async function () {
    // Create and connect client only if not already done
    if (!client) {
        // Create client in VU context (once per VU)
        client = new connectrpc.Client();

        const connected = client.connect('https://demo.connectrpc.com');

        if (!connected) {
            throw new Error('Failed to connect to Eliza demo service');
        }
        
        // Create the generated client wrapper
        elizaClient = new ElizaServiceClient(client);
    }

    // Test the unary Say method
    testSay(elizaClient);

    // Test the bidirectional Converse streaming method
    await testConverse(elizaClient);

    // Test the server streaming Introduce method
    await testIntroduce(elizaClient);
}

// Unary test function for the Say method
function testSay(elizaClient) {
    group('unary Say call', () => {
        try {
            // Use the generated client method
            const response = elizaClient.say({
                sentence: 'Hello Eliza, I am feeling anxious about my work.',
            });

            check(response, {
                'Say status is 200': (r) => r.status === 200,
                'Say has sentence in response': (r) => !!r.message && !!r.message.sentence,
                'Say response is not empty': (r) => r.message && r.message.sentence.length > 0,
            });

            if (response.message && response.message.sentence) {
                console.log(`Eliza responded: ${response.message.sentence}`);
            }

        } catch (error) {
            console.error(`Error in Say test: ${error.message}`);
            check(false, { 'Say should not throw': () => true });
        }
    });
}

// Bidirectional streaming test for the Converse method
function testConverse(elizaClient) {
    return new Promise((resolve, reject) => {
        group('bidirectional Converse stream', () => {
            try {
                // Use the generated client method that returns BidiStreamWrapper
                const streamWrapper = elizaClient.converseStream();

                const messages = [
                    'Hello Eliza, how are you?',
                    'I have been feeling stressed lately.',
                    'Work has been overwhelming me.',
                    'What do you think I should do?'
                ];
                let responseCount = 0;

                // Use the event-based pattern
                streamWrapper.on('data', function (response) {
                    console.log(`Eliza replied: ${response.sentence}`);
                    responseCount++;
                    check(response, {
                        'converse response has sentence': (r) => !!r.sentence,
                        'converse sentence is not empty': (r) => r.sentence.length > 0,
                    });
                });

                streamWrapper.on('error', function (err) {
                    console.error(`Converse stream error: ${err.code} - ${err.message}`);
                    reject(err);
                });

                streamWrapper.on('end', function () {
                    check(responseCount, {
                        'received responses from converse': (count) => count > 0,
                        'received reasonable number of responses': (count) => count <= messages.length,
                    });
                    resolve();
                });

                // Send all messages to the stream
                messages.forEach((message, index) => {
                    setTimeout(() => {
                        console.log(`Sending: ${message}`);
                        streamWrapper.write({ sentence: message });
                        
                        // Close the stream after the last message
                        if (index === messages.length - 1) {
                            setTimeout(() => {
                                streamWrapper.close();
                            }, 100);
                        }
                    }, index * 200); // Stagger messages slightly
                });

            } catch (error) {
                console.error(`Error setting up Converse stream test: ${error.message}`);
                reject(error);
            }
        });
    });
}

// Server streaming test for the Introduce method
function testIntroduce(elizaClient) {
    return new Promise((resolve, reject) => {
        group('server stream Introduce', () => {
            try {
                // Use the generated client method that returns ServerStreamWrapper
                const streamWrapper = elizaClient.introduce({
                    name: 'k6 Load Tester'
                });

                let responseCount = 0;

                // Use the event-based pattern
                streamWrapper.on('data', function (response) {
                    console.log(`Introduction response: ${response.sentence}`);
                    responseCount++;
                    check(response, {
                        'introduce response has sentence': (r) => !!r.sentence,
                        'introduce sentence is not empty': (r) => r.sentence.length > 0,
                    });
                });

                streamWrapper.on('error', function (err) {
                    console.error(`Introduce stream error: ${err.code} - ${err.message}`);
                    reject(err);
                });

                streamWrapper.on('end', function () {
                    check(responseCount, {
                        'received introduction responses': (count) => count > 0,
                        'received reasonable number of introductions': (count) => count <= 10, // Reasonable upper bound
                    });
                    console.log(`Received ${responseCount} introduction responses`);
                    resolve();
                });

            } catch (error) {
                console.error(`Error setting up Introduce stream test: ${error.message}`);
                reject(error);
            }
        });
    });
}

export function teardown() {
    console.log('Eliza service test completed');
} 