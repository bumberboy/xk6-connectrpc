.PHONY: build build-buf-plugin install-buf-plugin clean help

# Variables
PLUGIN_NAME = protoc-gen-k6-connectrpc
PLUGIN_DIR = ./protoc-gen-k6-connectrpc
K6_BINARY = ./k6

# Default target
all: build build-buf-plugin

# Build k6 with xk6-connectrpc extension
build:
	@echo "Building k6 with xk6-connectrpc extension..."
	xk6 build --with github.com/bumberboy/xk6-connectrpc=.

# Build the protoc-gen-k6-connectrpc plugin
build-buf-plugin:
	@echo "Building protoc-gen-k6-connectrpc plugin..."
	cd $(PLUGIN_DIR) && go build -o ../$(PLUGIN_NAME) .

# Install the protoc plugin to GOPATH/bin
install-buf-plugin: build-buf-plugin
	@echo "Installing protoc-gen-k6-connectrpc to GOPATH/bin..."
	cd $(PLUGIN_DIR) && go install .

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(K6_BINARY)
	rm -f $(PLUGIN_NAME)

# Show help
help:
	@echo "Available targets:"
	@echo "  build              - Build k6 with xk6-connectrpc extension"
	@echo "  build-buf-plugin   - Build the protoc-gen-k6-connectrpc plugin"
	@echo "  install-buf-plugin - Install the protoc plugin to GOPATH/bin"
	@echo "  clean              - Clean build artifacts"
	@echo "  all                - Build both k6 extension and protoc plugin"
	@echo "  help               - Show this help message" 