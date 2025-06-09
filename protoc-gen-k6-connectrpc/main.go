// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// protoc-gen-k6-connectrpc is a plugin for the Protobuf compiler that generates
// k6-friendly JavaScript and TypeScript clients for Connect-RPC services.
//
// The plugin integrates with xk6-connectrpc to provide type-safe, easy-to-use
// clients for load testing Connect, gRPC, and gRPC-Web services.
//
// To use it, build this program and make it available on your PATH as
// protoc-gen-k6-connectrpc.
//
// With buf, your buf.gen.yaml will look like this:
//
//	version: v2
//	plugins:
//	  - local: protoc-gen-k6-connectrpc
//	    out: k6
//	    opt:
//	      - output_format=js
//
// This generates k6 client code for the services defined in your protobuf files.
// If your service defines the UserService, the invocation above will write output to:
//
//	k6/user.k6.js
//
// The generated code provides type-safe client wrappers that work seamlessly with
// xk6-connectrpc for load testing.
package main

import (
	"embed"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	connect "connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

//go:embed templates/client.js.tmpl
var jsTemplateFS embed.FS

const (
	usage = "See https://connectrpc.com/docs/go/getting-started to learn how to use this plugin.\n\nFlags:\n  -h, --help\tPrint this help and exit.\n      --version\tPrint the version and exit."
)

// Template data structures to match the original template
type TemplateData struct {
	File                  FileInfo
	Services              []ServiceInfo
	Config                *Config
	StreamingWrappersPath string
	EmbeddedProtoset      string
}

type FileInfo struct {
	Path string
}

type ServiceInfo struct {
	Name     string
	FullName string
	Methods  []MethodInfo
}

type MethodInfo struct {
	Name             string
	CamelName        string
	ConstantName     string
	Comment          string
	StreamType       string
	ProcedurePath    string
	InputFields      []FieldInfo
	OutputFields     []FieldInfo
	HasIdempotency   bool
	IdempotencyLevel string
	SuggestedTimeout string
}

type FieldInfo struct {
	Name           string
	Type           string
	Required       bool
	IsOptional     bool
	HasValidation  bool
	ValidationCode string
	MockValue      string
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Fprintln(os.Stdout, connect.Version)
		os.Exit(0)
	}
	if len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Fprintln(os.Stdout, usage)
		os.Exit(0)
	}
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	// Read CodeGeneratorRequest from stdin
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read input: %v\n", err)
		os.Exit(1)
	}

	// Parse the request
	var request pluginpb.CodeGeneratorRequest
	if err := proto.Unmarshal(input, &request); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse CodeGeneratorRequest: %v\n", err)
		os.Exit(1)
	}

	// Parse parameters
	var flagSet flag.FlagSet
	cfg := RegisterFlags(&flagSet)

	// Parse parameters from the request
	if params := request.GetParameter(); params != "" {
		for _, param := range strings.Split(params, ",") {
			if kv := strings.SplitN(param, "=", 2); len(kv) == 2 {
				if err := flagSet.Set(kv[0], kv[1]); err != nil {
					fmt.Fprintf(os.Stderr, "Invalid parameter %s=%s: %v\n", kv[0], kv[1], err)
					os.Exit(1)
				}
			}
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	// Create response
	response := &pluginpb.CodeGeneratorResponse{
		SupportedFeatures: proto.Uint64(uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)),
	}

	// Generate streaming-wrappers.js if external wrappers are enabled and JS is being generated
	if cfg.ExternalWrappers && cfg.StreamingWrappers && cfg.ShouldGenerateJS() {
		if err := generateStreamingWrappersFile(cfg, response); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate streaming-wrappers.js: %v\n", err)
			os.Exit(1)
		}
	}

	// Process files
	for _, fileName := range request.GetFileToGenerate() {
		// Find the file descriptor
		var fileDesc *descriptorpb.FileDescriptorProto
		for _, file := range request.GetProtoFile() {
			if file.GetName() == fileName {
				fileDesc = file
				break
			}
		}

		if fileDesc == nil {
			fmt.Fprintf(os.Stderr, "File %s not found in request\n", fileName)
			continue
		}

		// Check if file has services
		if len(fileDesc.GetService()) == 0 {
			continue
		}

		// Generate JavaScript client if requested
		if cfg.ShouldGenerateJS() {
			if err := generateJavaScriptFile(fileDesc, cfg, &request, response); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to generate JavaScript for %s: %v\n", fileName, err)
				os.Exit(1)
			}
		}

		// Generate TypeScript client if requested
		if cfg.ShouldGenerateTS() {
			if err := generateTypeScriptFile(fileDesc, cfg, response); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to generate TypeScript for %s: %v\n", fileName, err)
				os.Exit(1)
			}
		}
	}

	// Write response to stdout
	output, err := proto.Marshal(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal response: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stdout.Write(output); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write response: %v\n", err)
		os.Exit(1)
	}
}

func generateJavaScriptFile(fileDesc *descriptorpb.FileDescriptorProto, cfg *Config, request *pluginpb.CodeGeneratorRequest, response *pluginpb.CodeGeneratorResponse) error {
	// Generate output filename
	filename := strings.TrimSuffix(fileDesc.GetName(), ".proto") + ".k6.js"

	// Build FileDescriptorSet for embedding
	fdSet := &descriptorpb.FileDescriptorSet{
		File: request.GetProtoFile(),
	}

	// Build template data
	templateData := buildTemplateData(fileDesc, cfg, fdSet)

	// Load and execute template
	content, err := executeJavaScriptTemplate(templateData)
	if err != nil {
		return fmt.Errorf("failed to execute JavaScript template: %w", err)
	}

	// Add to response
	response.File = append(response.File, &pluginpb.CodeGeneratorResponse_File{
		Name:    proto.String(filename),
		Content: proto.String(content),
	})

	return nil
}

func generateTypeScriptFile(fileDesc *descriptorpb.FileDescriptorProto, cfg *Config, response *pluginpb.CodeGeneratorResponse) error {
	// Generate output filename
	filename := strings.TrimSuffix(fileDesc.GetName(), ".proto") + ".k6.d.ts"

	// Generate simple TypeScript definitions
	content := fmt.Sprintf(`// Code generated by protoc-gen-k6-connectrpc. DO NOT EDIT.
//
// Source: %s
// Language: TypeScript

// TypeScript definitions for k6 Connect-RPC client

`, fileDesc.GetName())

	// Add service interface definitions
	for _, service := range fileDesc.GetService() {
		content += fmt.Sprintf(`
// Service: %s
export declare class %sClient {
    constructor(baseURL: string);
`, service.GetName(), service.GetName())

		for _, method := range service.GetMethod() {
			content += fmt.Sprintf(`
    // Method: %s
    %s(request: any): any;
`, method.GetName(), strings.ToLower(method.GetName()))
		}

		content += "}\n"
	}

	// Add to response
	response.File = append(response.File, &pluginpb.CodeGeneratorResponse_File{
		Name:    proto.String(filename),
		Content: proto.String(content),
	})

	return nil
}

func buildTemplateData(fileDesc *descriptorpb.FileDescriptorProto, cfg *Config, fdSet *descriptorpb.FileDescriptorSet) *TemplateData {
	data := &TemplateData{
		File: FileInfo{
			Path: fileDesc.GetName(),
		},
		Config: cfg,
	}

	// Calculate relative path to streaming-wrappers.js if external wrappers are enabled
	if cfg.ExternalWrappers && cfg.StreamingWrappers {
		// Get the output filename for the generated client
		outputFilename := strings.TrimSuffix(fileDesc.GetName(), ".proto") + ".k6.js"

		// Calculate the directory depth of the generated file relative to the output root
		outputDir := filepath.Dir(outputFilename)

		// Calculate how many levels up we need to go to reach the output root directory
		var relativePath string
		if outputDir == "." || outputDir == "" {
			// File is in the root output directory
			relativePath = "./streaming-wrappers.js"
		} else {
			// Count directory separators to determine depth
			// Use forward slashes for consistent counting as proto paths use forward slashes
			normalizedDir := strings.ReplaceAll(outputDir, "\\", "/")
			depth := strings.Count(normalizedDir, "/") + 1 // +1 because depth is one more than separator count

			// Build relative path with appropriate number of "../"
			upLevels := strings.Repeat("../", depth)
			relativePath = upLevels + "streaming-wrappers.js"
		}

		data.StreamingWrappersPath = relativePath
	} else {
		data.StreamingWrappersPath = "./streaming-wrappers.js"
	}

	// Build services
	for _, serviceDesc := range fileDesc.GetService() {
		service := ServiceInfo{
			Name:     serviceDesc.GetName(),
			FullName: fmt.Sprintf("%s.%s", fileDesc.GetPackage(), serviceDesc.GetName()),
		}

		// Build methods
		for _, methodDesc := range serviceDesc.GetMethod() {
			method := MethodInfo{
				Name:          methodDesc.GetName(),
				CamelName:     toCamelCase(methodDesc.GetName()),
				ConstantName:  strings.ToUpper(methodDesc.GetName()),
				Comment:       fmt.Sprintf("Method %s", methodDesc.GetName()),
				StreamType:    getStreamType(methodDesc),
				ProcedurePath: fmt.Sprintf("/%s.%s/%s", fileDesc.GetPackage(), serviceDesc.GetName(), methodDesc.GetName()),
			}

			service.Methods = append(service.Methods, method)
		}

		data.Services = append(data.Services, service)
	}

	// Serialize FileDescriptorSet to base64 for embedding
	fdSetBytes, err := proto.Marshal(fdSet)
	if err != nil {
		// If we can't marshal, just leave it empty - the client will work without auto-loading
		data.EmbeddedProtoset = ""
	} else {
		data.EmbeddedProtoset = base64.StdEncoding.EncodeToString(fdSetBytes)
	}

	return data
}

func getStreamType(method *descriptorpb.MethodDescriptorProto) string {
	if method.GetClientStreaming() && method.GetServerStreaming() {
		return "bidi_stream"
	} else if method.GetClientStreaming() {
		return "client_stream"
	} else if method.GetServerStreaming() {
		return "server_stream"
	}
	return "unary"
}

// toCamelCase converts a PascalCase string to camelCase
func toCamelCase(s string) string {
	if len(s) == 0 {
		return s
	}
	// Convert first letter to lowercase
	return strings.ToLower(s[:1]) + s[1:]
}

func executeJavaScriptTemplate(data *TemplateData) (string, error) {
	// Load template from embedded file system
	templateContent, err := jsTemplateFS.ReadFile("templates/client.js.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to read template: %w", err)
	}

	// Create and parse template
	tmpl, err := template.New("client.js").Parse(string(templateContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func generateStreamingWrappersFile(cfg *Config, response *pluginpb.CodeGeneratorResponse) error {
	content := `// Code generated by protoc-gen-k6-connectrpc. DO NOT EDIT.
//
// Streaming wrapper classes for k6 Connect-RPC

// Base streaming wrapper class
export class StreamWrapper {
  constructor(stream, options = {}) {
    this.stream = stream;
    this.options = options;
    this._setupErrorHandling();
  }

  _setupErrorHandling() {
    this.stream.on('error', (err) => {
      console.error(` + "`Stream error: ${err.code} - ${err.message}`" + `);
      if (this.options.onError) {
        this.options.onError(err);
      }
    });
  }
}

// Server streaming wrapper
export class ServerStreamWrapper extends StreamWrapper {
  constructor(stream, options) {
    super(stream, options);
    this._messageQueue = [];
    this._callbacks = [];
    this._onEndCallbacks = [];
    this._onErrorCallbacks = [];
    this._isEnded = false;
    this._setupHandlers();
  }

  // Event-based pattern (like original xk6-connectrpc)
  on(event, callback) {
    if (event === 'data') {
      this._callbacks.push(callback);
      // Process any queued messages
      while (this._messageQueue.length > 0) {
        callback(this._messageQueue.shift());
      }
    } else if (event === 'end') {
      if (this._isEnded) {
        callback();
      } else {
        this._onEndCallbacks.push(callback);
      }
    } else if (event === 'error') {
      this._onErrorCallbacks.push(callback);
    }
    return this;
  }

  // Callback-based iteration
  forEach(callback) {
    this.on('data', callback);
    return this;
  }

  // Event-style convenience methods
  onEnd(callback) {
    return this.on('end', callback);
  }

  onError(callback) {
    return this.on('error', callback);
  }

  // Promise-based collection of all messages
  collect() {
    return new Promise((resolve, reject) => {
      const messages = [];
      
      this.on('data', (message) => {
        messages.push(message);
      });
      
      this.on('end', () => {
        resolve(messages);
      });
      
      this.on('error', (err) => {
        reject(err);
      });
    });
  }

  _setupHandlers() {
    this.stream.on('data', (message) => {
      if (this._callbacks.length > 0) {
        this._callbacks.forEach(callback => callback(message));
      } else {
        this._messageQueue.push(message);
      }
    });

    this.stream.on('end', () => {
      this._isEnded = true;
      this._onEndCallbacks.forEach(callback => callback());
    });

    this.stream.on('error', (err) => {
      this._isEnded = true;
      this._onErrorCallbacks.forEach(callback => callback(err));
    });
  }
}

// Client streaming wrapper
export class ClientStreamWrapper extends StreamWrapper {
  constructor(stream, options) {
    super(stream, options);
    this._response = null;
    this._responsePromise = new Promise((resolve, reject) => {
      this._resolveResponse = resolve;
      this._rejectResponse = reject;
    });
    this._responseCallbacks = [];
    this._errorCallbacks = [];
    this._setupResponseHandlers();
  }

  write(message) {
    try {
      this.stream.write(message);
    } catch (err) {
      this._errorCallbacks.forEach(callback => callback(err));
      throw err;
    }
    return this;
  }

  close() {
    this.stream.end();
    return this;
  }

  // Event-based response handling
  onResponse(callback) {
    if (this._response) {
      callback(this._response);
    } else {
      this._responseCallbacks.push(callback);
    }
    return this;
  }

  onError(callback) {
    this._errorCallbacks.push(callback);
    return this;
  }

  // Promise-based response (for async/await when supported)
  response() {
    return this._responsePromise;
  }

  _setupResponseHandlers() {
    this.stream.on('data', (response) => {
      this._response = response;
      this._responseCallbacks.forEach(callback => callback(response));
    });

    this.stream.on('end', () => {
      this._resolveResponse(this._response);
    });

    this.stream.on('error', (err) => {
      this._errorCallbacks.forEach(callback => callback(err));
      this._rejectResponse(err);
    });
  }
}

// Bidirectional streaming wrapper
export class BidiStreamWrapper extends StreamWrapper {
  constructor(stream, options) {
    super(stream, options);
    this._messageQueue = [];
    this._callbacks = [];
    this._onEndCallbacks = [];
    this._onErrorCallbacks = [];
    this._isEnded = false;
    this._setupReadHandlers();
  }

  // Event-based pattern for reading
  on(event, callback) {
    if (event === 'data') {
      this._callbacks.push(callback);
      // Process any queued messages
      while (this._messageQueue.length > 0) {
        callback(this._messageQueue.shift());
      }
    } else if (event === 'end') {
      if (this._isEnded) {
        callback();
      } else {
        this._onEndCallbacks.push(callback);
      }
    } else if (event === 'error') {
      this._onErrorCallbacks.push(callback);
    }
    return this;
  }

  // Write to the stream
  write(message) {
    try {
      this.stream.write(message);
    } catch (err) {
      this._onErrorCallbacks.forEach(callback => callback(err));
      throw err;
    }
    return this;
  }

  // Close the stream
  close() {
    this.stream.end();
    return this;
  }

  // Convenience methods
  onEnd(callback) {
    return this.on('end', callback);
  }

  onError(callback) {
    return this.on('error', callback);
  }

  _setupReadHandlers() {
    this.stream.on('data', (message) => {
      if (this._callbacks.length > 0) {
        this._callbacks.forEach(callback => callback(message));
      } else {
        this._messageQueue.push(message);
      }
    });

    this.stream.on('end', () => {
      this._isEnded = true;
      this._onEndCallbacks.forEach(callback => callback());
    });

    this.stream.on('error', (err) => {
      this._isEnded = true;
      this._onErrorCallbacks.forEach(callback => callback(err));
    });
  }
}
`

	// Add to response
	response.File = append(response.File, &pluginpb.CodeGeneratorResponse_File{
		Name:    proto.String("streaming-wrappers.js"),
		Content: proto.String(content),
	})

	return nil
}
