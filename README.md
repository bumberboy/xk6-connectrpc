# xk6-connectrpc

A k6 extension that enables load testing of Connect-RPC services using the Connect protocol over HTTP.

## Overview

Connect-RPC is a protocol that allows calling gRPC services over HTTP/1.1 and HTTP/2 with JSON or binary payloads. This k6 extension provides native support for testing Connect-RPC services, including both unary calls and streaming operations.

## Installation

Build k6 with the connectrpc extension:

```bash
xk6 build --with github.com/bumberboy/xk6-connectrpc@latest
```

## Quick Start

```javascript
import connectrpc from 'k6/x/connectrpc';
import { check } from 'k6';

// Load proto files (init context only)
connectrpc.loadProtos([], 'your-service.proto');

export default function () {
    // Create client
    const client = new connectrpc.Client();
    
    // Connect to service
    const connected = client.connect('https://your-service.com', {
        protocol: 'connect',
        contentType: 'application/json',
    });

    if (!connected) {
        throw new Error('Failed to connect');
    }

    // Make unary call
    const response = client.invoke('/package.Service/Method', {
        field1: 'value1',
        field2: 'value2'
    });

    // Validate response
    check(response, {
        'status is 200': (r) => r.status === 200,
        'has expected data': (r) => !!r.message,
    });

    client.close();
}
```

## API Reference

### Global Functions

- **`connectrpc.loadProtos(importPaths, ...filenames)`**: Loads protocol buffer definitions from `.proto` files (init context only)
- **`connectrpc.loadProtoset(protosetPath)`**: Loads protocol buffer definitions from a protoset file (init context only)

### connectrpc.Client

- **Constructor**: `new connectrpc.Client()` - Creates a new Connect-RPC client instance
- **`connect(url, options)`**: Establishes connection to a Connect-RPC service
- **`invoke(method, request, params?)`**: Makes unary RPC calls
- **`close()`**: Closes the client connection

### connectrpc.Stream

- **Constructor**: `new connectrpc.Stream(client, method)` - Creates a bidirectional stream
- **Event Handlers**: 
  - `stream.on('data', callback)` - Handles incoming messages
  - `stream.on('error', callback)` - Handles stream errors
  - `stream.on('end', callback)` - Handles stream completion
- **Methods**:
  - `stream.write(data)` - Sends data to the stream
  - `stream.end()` - Closes the client-side of the stream

## Configuration Options

### Connection Options

```javascript
client.connect(url, {
    protocol: 'connect',                    // Required: 'connect', 'grpc', or 'grpc-web'
    contentType: 'application/json',        // Required: 'application/json', 'application/proto', or 'application/protobuf'
    plaintext: false,                       // Optional: true for HTTP, false for HTTPS (default: false)
    httpVersion: '2',                       // Optional: '1.1', '2', or 'auto' (default: '2')
    timeout: '30s',                         // Optional: duration string, null, '0', or 'infinite' (default: null)
    connectionStrategy: 'per-vu',           // Optional: 'per-vu', 'per-iteration', or 'per-call' (default: 'per-vu')
    tls: {                                  // Optional: TLS configuration
        insecureSkipVerify: false           // Optional: skip TLS verification (default: false)
    }
});
```

### Protocol Options

| Protocol   | Description                | Use Case                  |
|------------|----------------------------|---------------------------|
| `connect`  | Connect protocol (default) | Modern HTTP/JSON APIs     |
| `grpc`     | gRPC protocol over HTTP/2  | Traditional gRPC services |
| `grpc-web` | gRPC-Web protocol          | Browser-compatible gRPC   |

**Example:**
```javascript
// Connect protocol with JSON
client.connect(url, {
    protocol: 'connect',
    contentType: 'application/json'
});

// gRPC protocol with protobuf
client.connect(url, {
    protocol: 'grpc',
    contentType: 'application/protobuf'
});

// gRPC-Web protocol with JSON
client.connect(url, {
    protocol: 'grpc-web',
    contentType: 'application/json'
});
```

### Content Type Options

| Content Type           | Description                    | Protocol Compatibility |
|------------------------|--------------------------------|------------------------|
| `application/json`     | JSON encoding (human-readable) | All protocols          |
| `application/proto`    | Binary protobuf encoding       | All protocols          |
| `application/protobuf` | Alternative binary protobuf    | All protocols          |

**Example:**
```javascript
// JSON encoding (easier debugging)
client.connect(url, {
    protocol: 'connect',
    contentType: 'application/json'  // Request/response bodies are JSON
});

// Binary protobuf encoding (more efficient)
client.connect(url, {
    protocol: 'connect',
    contentType: 'application/proto'  // Request/response bodies are binary
});
```

### Connection Strategy Options

| Strategy               | Description                     | Connection Lifetime  | Use Case                    |
|------------------------|---------------------------------|----------------------|-----------------------------|
| `per-vu` (default)     | One connection per Virtual User | Entire test duration | Realistic load testing      |
| `per-iteration` | New connection each iteration   | Single iteration     | Connection overhead testing |
| `per-call`             | New connection each RPC call    | Single call          | Individual call testing     |

**Example:**
```javascript
// Persistent connections (recommended)
client.connect(url, {
    connectionStrategy: 'per-vu'  // Connection reused across all iterations
});

// Fresh connection each iteration
client.connect(url, {
    connectionStrategy: 'per-iteration'  // Test connection establishment overhead
});

// Fresh connection each call (unary only)
client.connect(url, {
    connectionStrategy: 'per-call'  // Maximum connection overhead
});
```

### Timeout Configuration

```javascript
// Connection-level timeout (applies to all calls)
client.connect(url, {
    timeout: '60s'          // 60 second timeout
});

// Infinite timeout (no timeout) - useful for persistent connections
client.connect(url, {
    timeout: null           // No timeout (recommended for streaming)
    // timeout: '0'         // Alternative: string '0' 
    // timeout: 'infinite'  // Alternative: string 'infinite'
});

// Call-level timeout (overrides connection timeout)
const response = client.invoke('/Service/Method', request, {
    timeout: '10s'          // This call gets 10s timeout
});
```

### HTTP Version Control

```javascript
// Force HTTP/1.1 (compatibility with older infrastructure)
client.connect(url, {
    httpVersion: '1.1'
});

// Force HTTP/2 (default, best performance)
client.connect(url, {
    httpVersion: '2'         // Default value
});

// Auto-negotiate HTTP version
client.connect(url, {
    httpVersion: 'auto'      // Let client choose the best version
});
```

### TLS Configuration

```javascript
// Secure by default (recommended for production)
client.connect('https://service.com', {
    // TLS certificate verification enabled by default
});

// Skip TLS verification (testing only)
client.connect('https://service.com', {
    tls: {
        insecureSkipVerify: true    // ⚠️ Only use for testing!
    }
});
```

## Examples

### Unary RPC Calls

```javascript
import connectrpc from 'k6/x/connectrpc';
import { check } from 'k6';

connectrpc.loadProtos([], 'service.proto');

export default function () {
    const client = new connectrpc.Client();
    
    const connected = client.connect('https://api.service.com', {
        protocol: 'connect',
        contentType: 'application/json',
        timeout: '30s'
    });

    if (!connected) {
        throw new Error('Failed to connect');
    }

    const response = client.invoke('/api.v1.UserService/GetUser', {
        userId: '12345'
    }, {
        timeout: '10s'  // Call-specific timeout
    });

    check(response, {
        'status is 200': (r) => r.status === 200,
        'user data exists': (r) => !!r.message.user,
    });

    client.close();
}
```

### Bidirectional Streaming

```javascript
import connectrpc from 'k6/x/connectrpc';
import { check } from 'k6';

connectrpc.loadProtos([], 'chat.proto');

export default async function () {
    const client = new connectrpc.Client();
    
    const connected = client.connect('https://chat.service.com', {
        protocol: 'connect',
        contentType: 'application/json',
        timeout: null,  // No timeout for streaming
        httpVersion: '2'  // HTTP/2 optimal for streaming
    });

    if (!connected) return;

    await testChatStream(client);
    client.close();
}

function testChatStream(client) {
    return new Promise((resolve, reject) => {
        const stream = new connectrpc.Stream(client, '/chat.v1.ChatService/Chat');
        
        let messageCount = 0;
        
        stream.on('data', function (response) {
            console.log('Received message:', response.message);
            messageCount++;
            check(response, {
                'message not empty': (r) => !!r.message,
            });
        });
        
        stream.on('error', function (err) {
            console.error(`Stream error: ${err.code} - ${err.message}`);
            reject(err);
        });
        
        stream.on('end', function () {
            console.log('Chat stream ended');
            check(messageCount, {
                'received messages': (count) => count > 0,
            });
            resolve();
        });
        
        // Send messages
        ['Hello', 'How are you?', 'Goodbye'].forEach((text) => {
            stream.write({ 
                user: 'test-user',
                message: text 
            });
        });
        
        stream.end();
    });
}
```

### Multiple Protocols Example

```javascript
import connectrpc from 'k6/x/connectrpc';
import { check } from 'k6';

connectrpc.loadProtos([], 'service.proto');

export default function () {
    // Test different protocol combinations
    testConnectJSON();
    testGRPCProtobuf();
    testGRPCWebJSON();
}

function testConnectJSON() {
    const client = new connectrpc.Client();
    
    client.connect('https://api.service.com', {
        protocol: 'connect',
        contentType: 'application/json'
    });
    
    const response = client.invoke('/service.API/Method', { data: 'test' });
    check(response, { 'connect+json works': (r) => r.status === 200 });
    
    client.close();
}

function testGRPCProtobuf() {
    const client = new connectrpc.Client();
    
    client.connect('https://grpc.service.com', {
        protocol: 'grpc',
        contentType: 'application/protobuf'
    });
    
    const response = client.invoke('/service.API/Method', { data: 'test' });
    check(response, { 'grpc+protobuf works': (r) => r.status === 200 });
    
    client.close();
}

function testGRPCWebJSON() {
    const client = new connectrpc.Client();
    
    client.connect('https://web.service.com', {
        protocol: 'grpc-web',
        contentType: 'application/json'
    });
    
    const response = client.invoke('/service.API/Method', { data: 'test' });
    check(response, { 'grpc-web+json works': (r) => r.status === 200 });
    
    client.close();
}
```

## Best Practices

### 1. Choose the Right Protocol and Content Type
- Use `connect` protocol with `application/json` for modern APIs (easier debugging)
- Use `grpc` protocol with `application/protobuf` for traditional gRPC services (better performance)
- Use `grpc-web` protocol for browser-compatible services

### 2. Connection Strategy Selection
- Use `per-vu` (default) for realistic load testing
- Use `per-iteration` only when testing connection establishment overhead
- Use `per-call` for testing individual call performance (unary calls only)

### 3. Timeout Configuration
- Set `timeout: null` for persistent streaming connections
- Use reasonable timeouts for unary calls (e.g., `timeout: '30s'`)
- Use call-specific timeouts for operations with different requirements

### 4. HTTP Version Selection
- Use `httpVersion: '2'` (default) for best performance, especially with persistent connections
- Use `httpVersion: '1.1'` when testing compatibility with older infrastructure
- Use `httpVersion: 'auto'` to test HTTP version negotiation

### 5. Error Handling
- Always check connection status before making calls
- Handle stream errors properly with `stream.on('error', callback)`
- Use k6's `check()` function to validate responses

### 6. Streaming Best Practices
- Make your default function `async` for streaming tests
- Use `await` to ensure streams complete before test ends
- Set `timeout: null` to avoid unexpected stream disconnections

## Validation

The extension validates configuration parameters at connection time and only accepts standard values:

- **protocol**: Must be `'connect'`, `'grpc'`, or `'grpc-web'` (custom protocols not supported)
- **contentType**: Must be `'application/json'`, `'application/proto'`, or `'application/protobuf'` (custom content types not supported)
- **httpVersion**: Must be `'1.1'`, `'2'`, or `'auto'`
- **connectionStrategy**: Must be `'per-vu'`, `'per-iteration'`, or `'per-call'`

Invalid values will throw descriptive errors with valid options listed. This ensures compatibility and prevents configuration mistakes.

## Metrics

The extension automatically collects metrics for:

- Request duration and counts per procedure
- Stream duration and message counts
- HTTP connection establishment and reuse
- Error rates per procedure and operation type

Use k6's built-in thresholds to set performance criteria based on these metrics.

## Development

### Running Tests

```bash
go test ./...
```

### Building

```bash
xk6 build --with github.com/bumberboy/xk6-connectrpc@latest
```

## Contributing

When contributing:
- Follow k6 extension development patterns
- Ensure compatibility with all supported protocols and content types
- Add comprehensive error handling
- Include test cases and examples
- Update documentation for new features