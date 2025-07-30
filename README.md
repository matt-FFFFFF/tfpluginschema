# tfpluginschema

A Go library for downloading and retrieving schemas from Terraform/OpenTofu providers using the Terraform Plugin Protocol (v5 and v6).

## Overview

`tfpluginschema` provides a unified interface to interact with Terraform provider plugins, supporting both protocol v5 and v6. It can automatically download providers from the OpenTofu registry, extract them, and retrieve their schemas including provider configuration, resources, data sources, and functions.

## Features

- **Multi-protocol support**: Works with both Terraform Plugin Protocol v5 and v6
- **Automatic provider download**: Downloads and extracts providers from the OpenTofu registry
- **Schema retrieval**: Get complete schemas or individual resource/data source/function/ephemeral schemas
- **Caching**: Built-in caching for both downloads and schemas
- **Cross-platform**: Supports multiple operating systems and architectures

## Installation

```bash
go get github.com/matt-FFFFFF/tfpluginschema
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    "github.com/matt-FFFFFF/tfpluginschema"
)

func main() {
    // Create a new server instance
    server := tfpluginschema.NewServer(nil)
    defer server.Cleanup()

    // Define a provider request
    request := tfpluginschema.Request{
        Namespace: "hashicorp",
        Name:      "azurerm",
        Version:   "4.36.0",
    }

    // Download the provider (optional - automatically done when getting schema)
    if err := server.Get(request); err != nil {
        log.Fatal(err)
    }

    // Get the complete provider schema
    schema, err := server.GetProviderSchema(request)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(string(schema))
}
```

## Exported Types

### Request

Represents a provider request with namespace, name, and version information.

```go
type Request struct {
    Namespace string // Provider namespace (e.g., "Azure")
    Name      string // Provider name (e.g., "azapi")
    Version   string // Provider version (e.g., "2.5.0")
}
```

**Methods:**
- `String() string` - Returns the OpenTofu registry download URL for the provider

### Server

The main server struct that manages provider downloads and schema caching.

```go
type Server struct {
    // private fields
}
```

**Constructor:**
- `NewServer(l *slog.Logger) *Server` - Creates a new server instance with optional logger

**Methods:**
- `Get(request Request) error` - Downloads and extracts the specified provider
- `GetResourceSchema(request Request, resource string) ([]byte, error)` - Retrieves schema for a specific resource
- `GetDataSourceSchema(request Request, dataSource string) ([]byte, error)` - Retrieves schema for a specific data source
- `GetFunctionSchema(request Request, function string) ([]byte, error)` - Retrieves schema for a specific function
- `GetEphemeralResourceSchema(request Request, resource string) ([]byte, error)` - Retrieves schema for an ephemeral resource
- `GetProviderSchema(request Request) ([]byte, error)` - Retrieves the complete provider schema
- `Cleanup()` - Removes temporary directories and cleans up resources

## Usage Examples

### Getting a Resource Schema

```go
server := tfpluginschema.NewServer(nil)
defer server.Cleanup()

request := tfpluginschema.Request{
    Namespace: "hashicorp",
    Name:      "aws",
    Version:   "5.0.0",
}

// Get schema for aws_instance resource
resourceSchema, err := server.GetResourceSchema(request, "aws_instance")
if err != nil {
    log.Fatal(err)
}

fmt.Println(string(resourceSchema))
```

### Getting a Data Source Schema

```go
server := tfpluginschema.NewServer(nil)
defer server.Cleanup()

request := tfpluginschema.Request{
    Namespace: "hashicorp",
    Name:      "aws",
    Version:   "5.0.0",
}

// Get schema for aws_ami data source
dataSourceSchema, err := server.GetDataSourceSchema(request, "aws_ami")
if err != nil {
    log.Fatal(err)
}

fmt.Println(string(dataSourceSchema))
```

### Getting a Function Schema

```go
server := tfpluginschema.NewServer(nil)
defer server.Cleanup()

request := tfpluginschema.Request{
    Namespace: "hashicorp",
    Name:      "aws",
    Version:   "5.0.0",
}

// Get schema for a provider function (if available)
functionSchema, err := server.GetFunctionSchema(request, "some_function")
if err != nil {
    log.Fatal(err)
}

fmt.Println(string(functionSchema))
```

### Custom Logging

```go
import "log/slog"

// Create a custom logger
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

server := tfpluginschema.NewServer(logger)
defer server.Cleanup()

// Server will now use custom logger for all operations
```

## Architecture

The library consists of several key components:

1. **Server**: Main orchestrator that handles downloads, caching, and schema retrieval
2. **RPC Client**: Handles communication with provider plugins using gRPC
3. **Protocol Support**: Supports both Terraform Plugin Protocol v5 and v6
4. **Schema Processing**: Automatically decodes base64-encoded type information
5. **Caching**: In-memory caching of both downloaded providers and retrieved schemas

## Protocol Support

The library automatically detects and supports both Terraform Plugin Protocol versions:

- **Protocol v5**: Legacy protocol used by older providers
- **Protocol v6**: Current protocol with enhanced features

The universal client interface abstracts away the protocol differences, providing a consistent API regardless of the underlying protocol version.

## Caching

The library implements two levels of caching:

1. **Download Cache**: Prevents re-downloading the same provider version
2. **Schema Cache**: Caches retrieved schemas to avoid repeated RPC calls

Caches are automatically managed and cleared when the server is cleaned up.

## Error Handling

The library defines specific error types for different failure scenarios:

- `ErrPluginNotFound`: Provider not found in registry
- `ErrPluginApi`: API communication errors
- `ErrNotImplemented`: Unimplemented functionality

## Dependencies

- `github.com/hashicorp/go-plugin` - Plugin framework
- `github.com/hashicorp/go-hclog` - Logging
- `google.golang.org/grpc` - gRPC communication
- `google.golang.org/protobuf` - Protocol buffer support

## License

This project follows the same license as specified in the source code.

## Contributing

Contributions are welcome! Please ensure that:

1. All exported types and methods are properly documented
2. Tests are included for new functionality
3. Code follows Go best practices and conventions
4. Integration tests pass with real providers

## Notes

- The library uses the OpenTofu registry (`https://registry.opentofu.org`) by default
- Temporary files are automatically cleaned up when `Server.Cleanup()` is called
- The library handles cross-platform provider downloads automatically
- Base64-encoded type information in schemas is automatically decoded for easier consumption

## Acknowledgements

Thanks to the OpenTofu community for their contributions and the maintainers of the OpenTofu plugin protocol.
This library builds upon their work to provide a seamless experience for Go developers working with Terraform and OpenTofu providers.
