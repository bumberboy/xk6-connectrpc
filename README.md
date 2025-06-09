# xk6-connectrpc

A k6 extension that enables load testing of Connect-RPC services using the Connect protocol over HTTP.

## What's Included

This project contains two main components:

1. **xk6-connectrpc** - The k6 extension for Connect-RPC load testing
2. **protoc-gen-k6-connectrpc** - A Protocol Buffer compiler plugin that generates k6-friendly JavaScript clients

**🎯 Recommended Approach**: Use `protoc-gen-k6-connectrpc` to generate JavaScript clients for your services. This provides the best developer experience with zero setup required.

## Installation

Build k6 with the connectrpc extension:

```bash
xk6 build --with github.com/bumberboy/xk6-connectrpc@latest
```

## Quick Start (Recommended: Generated Clients)

### 1. Generate k6 clients from your proto files

See [protoc-gen-k6-connectrpc/README.md](./protoc-gen-k6-connectrpc/README.md) for detailed setup instructions.

```bash
# Configure buf.gen.yaml and run:
buf generate
```

### 2. Use generated clients in your k6 tests

```javascript
import connectrpc from 'k6/x/connectrpc';
import { ElizaServiceClient } from './k6/connectrpc/eliza/v1/eliza.k6.js';
import { check } from 'k6';

export default function () {
    // Create xk6-connectrpc client
    const client = new connectrpc.Client();
    client.connect('https://demo.connectrpc.com');
    
    // Use generated client wrapper (proto definitions auto-loaded!)
    const elizaClient = new ElizaServiceClient(client);
    
    const response = elizaClient.say({ 
        sentence: 'Hello Eliza!' 
    });
    
    check(response, {
        'eliza responded': (r) => r.status === 200,
        'has response': (r) => !!r.message?.sentence
    });
    
    client.close();
}
```

## Alternative: Raw xk6-connectrpc API

If you prefer to use the extension directly without generated clients:

```javascript
import connectrpc from 'k6/x/connectrpc';
import { check } from 'k6';

// Load proto files manually (init context only)
connectrpc.loadProtos([], 'your-service.proto');

export default function () {
    const client = new connectrpc.Client();
    
    const connected = client.connect('https://your-service.com', {
        protocol: 'connect',
        contentType: 'application/json',
    });

    if (!connected) {
        throw new Error('Failed to connect');
    }

    // Manual method path construction
    const response = client.invoke('/package.Service/Method', {
        field1: 'value1',
        field2: 'value2'
    });

    check(response, {
        'status is 200': (r) => r.status === 200,
        'has expected data': (r) => !!r.message,
    });

    client.close();
}
```

## Examples

See the [examples/](./examples/) directory for complete working examples:

- **[generated-client-example.js](./examples/generated-client-example.js)** - Using generated clients (recommended)
- **[unary-example.js](./examples/unary-example.js)** - Basic unary RPC calls
- **[streaming-example.js](./examples/streaming-example.js)** - Bidirectional streaming

## API Reference

### Global Functions

- **`connectrpc.loadProtos(importPaths, ...filenames)`**: Load `.proto` files (init context only)
- **`connectrpc.loadProtoset(protosetPath)`**: Load protoset file (init context only)  
- **`connectrpc.loadEmbeddedProtoset(base64Data)`**: Load embedded proto definitions (init context only)

> **Note**: Generated clients automatically embed and load proto definitions - no manual loading required!

### connectrpc.Client

- **Constructor**: `new connectrpc.Client()` - Creates a new client instance
- **`connect(url, options)`**: Establishes connection to a Connect-RPC service
- **`invoke(method, request, params?)`**: Makes unary RPC calls
- **`close()`**: Closes the client connection

### connectrpc.Stream

- **Constructor**: `new connectrpc.Stream(client, method)` - Creates a bidirectional stream
- **Event Handlers**: `stream.on('data'|'error'|'end', callback)`
- **Methods**: `stream.write(data)`, `stream.end()`

## Configuration

### Connection Options

```javascript
client.connect(url, {
    protocol: 'connect',                    // 'connect', 'grpc', or 'grpc-web'
    contentType: 'application/json',        // 'application/json', 'application/proto', or 'application/protobuf'
    plaintext: false,                       // true for HTTP, false for HTTPS
    httpVersion: '2',                       // '1.1', '2', or 'auto'
    timeout: '30s',                         // duration string, null, '0', or 'infinite'
    connectionStrategy: 'per-vu',           // 'per-vu', 'per-iteration', or 'per-call'
    tls: {
        insecureSkipVerify: false           // skip TLS verification (testing only)
    }
});
```

### Protocol Support

| Protocol   | Description                | Content Types                    |
|------------|----------------------------|----------------------------------|
| `connect`  | Connect protocol (default) | JSON, protobuf                   |
| `grpc`     | gRPC protocol over HTTP/2  | JSON, protobuf                   |
| `grpc-web` | gRPC-Web protocol          | JSON, protobuf                   |

### Connection Strategies

| Strategy        | Description                     | Use Case                    |
|-----------------|---------------------------------|-----------------------------|
| `per-vu`        | One connection per Virtual User | Realistic load testing      |
| `per-iteration` | New connection each iteration   | Connection overhead testing |
| `per-call`      | New connection each RPC call    | Individual call testing     |

## Best Practices

1. **Use generated clients** for the best developer experience
2. **Choose appropriate protocols**: `connect` + JSON for modern APIs, `grpc` + protobuf for traditional gRPC
3. **Use `per-vu` connection strategy** for realistic load testing
4. **Set `timeout: null`** for streaming connections
5. **Always validate responses** with k6's `check()` function

## Code Generation

For detailed information about generating k6-compatible JavaScript clients from your proto files, see:

**[protoc-gen-k6-connectrpc/README.md](./protoc-gen-k6-connectrpc/README.md)**

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

Contributions welcome! Please ensure:
- Compatibility with all supported protocols and content types
- Comprehensive error handling and test coverage
- Updated documentation for new features