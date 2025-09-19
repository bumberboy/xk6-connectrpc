# xk6-connectrpc

A k6 extension that enables load testing of Connect-RPC services using the Connect protocol over HTTP.

## What's Included

This project contains the **xk6-connectrpc** k6 extension for Connect-RPC load testing.

## Installation

Build k6 with the connectrpc extension:

```bash
xk6 build --with github.com/bumberboy/xk6-connectrpc@latest
```

## Quick Start

### Load proto files and use the xk6-connectrpc API

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

## Working with Buf Schema Registry

For services published to the buf schema registry, you'll need to export the proto definitions locally first:

### 1. Export proto files from buf schema registry

```bash
# Export proto definitions to a local directory
buf export buf.build/your-org/your-module -o ./proto-definitions
```

This will create a directory structure with all the proto files and their dependencies.

### 2. Load multiple proto files with import paths

```javascript
import connectrpc from 'k6/x/connectrpc';

// Load proto files with import path and multiple files
connectrpc.loadProtos(['/path/to/proto-definitions'], 
    'auth/v2/auth.proto',
    'session/v2/session.proto', 
    'verification/v2/verification.proto'
);

export default function () {
    const client = new connectrpc.Client();
    
    // Reusable connection settings
    const connectionSettings = {
        protocol: 'connect',
        contentType: 'application/proto', // or 'application/json'
        timeout: '30s'
    };
    
    client.connect('https://your-service.com', connectionSettings);
    
    // Use full service method paths
    const response = client.invoke('/package.v2.ServiceName/MethodName', {
        // your request data
    }, {
        headers: {
            'Authorization': 'Bearer your-token',
            'X-Custom-Header': 'value'
        }
    });
    
    client.close();
}
```

## Examples

See the [examples/](./examples/) directory for complete working examples:

- **[unary-example.js](./examples/unary-example.js)** - Basic unary RPC calls
- **[streaming-example.js](./examples/streaming-example.js)** - Bidirectional streaming
- **[server-side-streaming.js](./examples/server-side-streaming.js)** - Server-side streaming with response validation

## API Reference

### Global Functions

- **`connectrpc.loadProtos(importPaths, ...filenames)`**: Load `.proto` files (init context only)
- **`connectrpc.loadProtoset(protosetPath)`**: Load protoset file (init context only)  
- **`connectrpc.loadEmbeddedProtoset(base64Data)`**: Load embedded proto definitions (init context only)

#### Loading Proto Files

```javascript
// Single proto file
connectrpc.loadProtos([], 'service.proto');

// Multiple proto files with import paths
connectrpc.loadProtos(['/path/to/proto/root'], 
    'auth/v2/auth.proto',
    'session/v2/session.proto'
);

// Using protoset file (compiled proto definitions)
connectrpc.loadProtoset('path/to/compiled.protoset');
```

### connectrpc.Client

- **Constructor**: `new connectrpc.Client()` - Creates a new client instance
- **`connect(url, options)`**: Establishes connection to a Connect-RPC service
- **`invoke(method, request, params?)`**: Makes unary RPC calls
- **`close()`**: Closes the client connection

#### Making Requests with Headers

```javascript
const response = client.invoke('/package.Service/Method', requestData, {
    headers: {
        'Authorization': 'Bearer token',
        'X-Custom-Header': 'value'
    }
});
```

### connectrpc.Stream

- **Constructor**: `new connectrpc.Stream(client, method)` - Creates a bidirectional stream
- **Event Handlers**: `stream.on('data'|'error'|'end', callback)`
- **Methods**:
  - `stream.write(data)` - Send data to the stream
  - `stream.end()` - Close the write side of the stream (server continues sending)
  - `stream.close()` - Immediately terminate the entire stream (both read and write)

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

> **Note**: HTTP GET requests are not supported in k6 extensions due to Connect library limitations with dynamic protobuf clients. All requests use HTTP POST regardless of method idempotency.

### Connection Strategies

| Strategy        | Description                     | Use Case                    |
|-----------------|---------------------------------|-----------------------------|
| `per-vu`        | One connection per Virtual User | Realistic load testing      |
| `per-iteration` | New connection each iteration   | Connection overhead testing |
| `per-call`      | New connection each RPC call    | Individual call testing     |

## Advanced Patterns

### Authentication Flows

For authentication flows, use a single client and pass headers per request:

```javascript
export default function () {
    const client = new connectrpc.Client();
    client.connect(baseUrl, connectionSettings);
    
    // Step 1: Login without authentication
    const loginResponse = client.invoke('/auth.Service/Login', credentials);
    const token = loginResponse.message.accessToken;
    
    // Step 2: Use token for authenticated requests
    const dataResponse = client.invoke('/api.Service/GetData', {}, {
        headers: {
            'Authorization': `Bearer ${token}`
        }
    });
    
    // Step 3: More authenticated requests
    const userResponse = client.invoke('/user.Service/GetProfile', {}, {
        headers: {
            'Authorization': `Bearer ${token}`
        }
    });
    
    client.close();
}
```

### Reusable Connection Settings

Define connection settings once and reuse them:

```javascript
const connectionSettings = {
    protocol: 'connect',
    contentType: 'application/proto',
    timeout: '3s'
};

export default function () {
    const client = new connectrpc.Client();
    client.connect('https://your-service.com', connectionSettings);
    // ... rest of your test
}
```

## Error Handling

xk6-connectrpc provides comprehensive error information for debugging Connect RPC failures:

### Error Response Structure

When RPC calls fail, the response includes detailed error information:

```javascript
const response = client.invoke('/service.Service/Method', request);

if (response.status !== 200) {
    console.log('Error details:', {
        status: response.status,           // HTTP status (400, 404, 500, etc.)
        code: response.message.code,       // Connect error code ('invalid_argument', 'not_found', etc.)  
        message: response.message.message, // Full error message
        details: response.message.details  // Structured error details (array)
    });
}
```

### Error Details

Error details provide structured information about failures:

```javascript
// Example error response
{
    "message": {
        "code": "invalid_argument",
        "message": "invalid_argument: validation failed for field 'email'",
        "details": [
            {
                "type": "google.rpc.BadRequest",
                "value": {
                    "fieldViolations": [
                        {
                            "field": "email", 
                            "description": "must be a valid email address"
                        }
                    ]
                },
                "bytes": [8, 1, 18, 5, ...]  // Raw protobuf bytes
            }
        ]
    },
    "status": 400,
    "headers": {...},
    "trailers": {...}
}
```

### Common Error Handling Pattern

```javascript
export default function () {
    const client = new connectrpc.Client();
    client.connect(baseUrl, connectionSettings);
    
    const response = client.invoke('/auth.Service/Login', credentials);
    
    // Check for errors
    if (response.status !== 200) {
        console.error('RPC failed:', {
            httpStatus: response.status,
            errorCode: response.message.code,
            errorMessage: response.message.message
        });
        
        // Process structured error details if available
        if (response.message.details && response.message.details.length > 0) {
            response.message.details.forEach((detail, i) => {
                console.error(`Error detail ${i + 1}:`, {
                    type: detail.type,
                    value: detail.value
                });
            });
        }
        
        return; // Skip rest of test
    }
    
    // Success case
    check(response, {
        'login successful': (r) => r.status === 200,
        'has access token': (r) => !!r.message.accessToken
    });
    
    client.close();
}
```

## Best Practices

1. **Load proto files** in the init context using `connectrpc.loadProtos()`
2. **Export buf modules locally** using `buf export` before testing
3. **Use import paths** when loading multiple related proto files
4. **Choose appropriate protocols**: `connect` + JSON for modern APIs, `grpc` + protobuf for traditional gRPC
5. **Use `per-vu` connection strategy** for realistic load testing
6. **Set `timeout: null`** for streaming connections
7. **Always validate responses** with k6's `check()` function
8. **Clean up connections** with `client.close()` at the end of your test

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

## AI Use
Most of the code and documentation in this repository were vibe coded.