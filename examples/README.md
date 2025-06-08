# xk6-connectrpc Examples

This folder contains example k6 scripts demonstrating how to use the xk6-connectrpc extension.

## Prerequisites

1. Build k6 with the connectrpc extension:
   ```bash
   xk6 build --with github.com/bumberboy/xk6-connectrpc@latest
   ```

2. Have a Connect-RPC service running that implements the `clown.proto` interface, or modify the examples to use your own service and proto files.

## Examples

### 1. Unary RPC Calls (`unary-example.js`)

Demonstrates basic unary RPC calls using the Connect protocol with JSON encoding.

**Run:**
```bash
./k6 run examples/unary-example.js
```

**Features:**
- Connection establishment
- Single request-response calls
- Response validation
- Metrics collection

### 2. Bidirectional Streaming (`streaming-example.js`)

Shows how to use bidirectional streaming with proper async handling.

**Run:**
```bash
./k6 run examples/streaming-example.js
```

**Features:**
- Bidirectional streaming
- Event-driven message handling
- Stream lifecycle management
- Async/await patterns

### 3. Full Integration Test (`test-connectrpc-clown.js`)

Comprehensive example that tests both unary and streaming operations with detailed metrics.

**Run:**
```bash
./k6 run examples/test-connectrpc-clown.js
```

**Features:**
- Multiple test scenarios
- Comprehensive metrics and thresholds
- Connection reuse patterns
- Error handling

## Customizing Examples

To use these examples with your own service:

1. Replace `clown.proto` with your own proto file
2. Update the service URLs in the scripts
3. Modify the RPC method names and message structures
4. Adjust the test thresholds based on your performance requirements

## Protocol and Content Type Combinations

These examples can be easily modified to test different protocol combinations:

- **Connect + JSON**: `protocol: 'connect', contentType: 'application/json'`
- **Connect + Protobuf**: `protocol: 'connect', contentType: 'application/proto'`
- **gRPC + Protobuf**: `protocol: 'grpc', contentType: 'application/protobuf'`
- **gRPC-Web + JSON**: `protocol: 'grpc-web', contentType: 'application/json'` 