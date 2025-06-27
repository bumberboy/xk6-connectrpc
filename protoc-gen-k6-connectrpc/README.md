# protoc-gen-k6-connectrpc

## ⚠️ Important Notice

**This plugin is currently not recommended for production use and requires more comprehensive testing.** There are known issues that need to be resolved before it can be considered stable. We are actively looking for contributors to help improve and test this plugin.

**For reliable k6 Connect-RPC testing, please use the raw [xk6-connectrpc](https://github.com/bumberboy/xk6-connectrpc) API instead.**

---

A Protocol Buffer compiler plugin that generates k6-friendly JavaScript clients for Connect-RPC services. This plugin integrates with [xk6-connectrpc](https://github.com/bumberboy/xk6-connectrpc) to provide easy-to-use clients for load testing Connect, gRPC, and gRPC-Web services.

## Current Status

⚠️ **Under Development** - This plugin currently has reliability issues and needs more comprehensive testing across different protobuf schemas and service configurations.

## Intended Features (When Stable)

✨ **Zero Setup Required** - Proto definitions are automatically embedded in generated clients  
🌐 **Buf Schema Registry Support** - Works seamlessly with external modules like `buf.build/connectrpc/eliza`  
🔄 **k6-Compatible Streaming** - Full support for all RPC patterns (unary, server/client/bidi streaming)  
📝 **JavaScript Client Generation** - Generates clean, well-structured JavaScript clients  
🎯 **Load Testing Optimized** - Designed specifically for k6's JavaScript engine limitations  

> **Note**: Currently only JavaScript generation is supported. TypeScript support is planned for a future release.  

## Overview

Instead of manually constructing service method paths, loading proto files, and handling streaming boilerplate in your k6 tests, this plugin is intended to generate:

- **Self-contained client classes** with embedded proto definitions (no manual loading required!)
- **k6-compatible streaming abstractions** that work with k6's JavaScript engine limitations
- **Protocol constants** for service and method names
- **Request/response validation** helpers
- **Mock generators** for testing
- **Automatic buf schema registry integration**

## Installation (Not Recommended for Production Use)

Pull repo and build the plugin:

```bash
go build -o protoc-gen-k6-connectrpc ./cmd/protoc-gen-k6-connectrpc
```

Or install directly:

```bash
go install github.com/bumberboy/xk6-connectrpc/protoc-gen-k6-connectrpc@latest
```

Make sure `protoc-gen-k6-connectrpc` is available in your `$PATH`.

## Usage (Experimental)

⚠️ **Warning**: The following usage examples may not work reliably due to current plugin issues.

### 1. Configure buf.gen.yaml

**Using local proto files:**
```yaml
version: v2
plugins:
  - local: protoc-gen-go
    out: gen
  - local: protoc-gen-connect-go
    out: gen
  - local: protoc-gen-k6-connectrpc
    out: k6
    opt:
      - output_format=js
```

**Using buf schema registry:**
```yaml
version: v2
inputs:
  - directory: .
  - module: buf.build/connectrpc/eliza  # External module from schema registry
plugins:
  - local: protoc-gen-k6-connectrpc
    out: k6
    opt:
      - output_format=js
      - external_wrappers=true
```

### 2. Generate clients

```bash
buf generate
```

### 3. Use in k6 tests (May Not Work)

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
        sentence: 'Hello Eliza, I am feeling anxious about work.' 
    });
    
    check(response, {
        'eliza responded': (r) => r.status === 200,
        'has response': (r) => !!r.message?.sentence
    });
    
    client.close();
}
```

## Configuration Options

Configure the plugin using the `opt` parameter in `buf.gen.yaml`:

| Option               | Values          | Default  | Description                                         |
|----------------------|-----------------|----------|-----------------------------------------------------|
| `output_format`      | `js`            | `js`     | Output format (only JavaScript currently supported) |
| `client_suffix`      | string          | `Client` | Suffix for generated client class names             |
| `include_mocks`      | `true`, `false` | `false`  | Generate mock response helpers                      |
| `include_validation` | `true`, `false` | `true`   | Generate request validation                         |
| `streaming_wrappers` | `true`, `false` | `true`   | Generate streaming wrapper classes                  |
| `external_wrappers`  | `true`, `false` | `false`  | Import streaming wrappers from external file        |

## Development

### Building

```bash
go build -o protoc-gen-k6-connectrpc ./cmd/protoc-gen-k6-connectrpc
```

### Testing

```bash
go test ./cmd/protoc-gen-k6-connectrpc/...
```

## Contributing

**Help Wanted!** This plugin needs significant work to be production-ready. We're looking for contributors with experience in:

- Protocol Buffer compiler plugins
- JavaScript/TypeScript code generation  
- k6 extension development
- Comprehensive testing strategies

Areas that need attention:
- Reliability across different protobuf schemas
- Error handling and edge cases
- Streaming functionality
- Integration testing
- Documentation improvements

### Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

Licensed under the Apache License, Version 2.0. See LICENSE for details.