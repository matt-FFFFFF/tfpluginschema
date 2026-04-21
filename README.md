# tfpluginschema

A Go library & cli for downloading and retrieving schemas from Terraform/OpenTofu providers using the Terraform Plugin Protocol (v5 and v6).

Sibling project to [tfmoduleschema](https://github.com/matt-FFFFFF/tfmoduleschema), but for providers instead of modules.

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

## CLI

```
tfpluginschema --ns <namespace> -n <name> \
  [--version-constraint VERSION] [--registry opentofu|terraform] \
  <command>
```

Global flags:

| Flag | Alias | Description |
|---|---|---|
| `--namespace` | `--ns` | Provider namespace (required). |
| `--name` | `-n` | Provider name (required). |
| `--version-constraint` | `--vc` | Concrete version or constraint. Empty = latest. |
| `--registry` | `-r` | `opentofu` (default) or `terraform`. |
| `--cache-dir` | | Cache directory. Overrides `$TFPLUGINSCHEMA_CACHE_DIR`. |
| `--force-fetch` | | Always re-download. |
| `--quiet` | | Suppress `cache hit:` / `downloading:` status on stderr. |

Commands:

| Command | Description |
|---|---|
| `provider schema` | Provider configuration schema as JSON. |
| `resource list` | Newline-separated resource type names. |
| `resource schema [name]` | Full schema for one resource, or all. |
| `datasource list` | Newline-separated data source names. |
| `datasource schema [name]` | Full schema for one data source, or all. |
| `function list` | Newline-separated function names. |
| `function schema [name]` | Full schema for one function, or all. |
| `ephemeral list` | Newline-separated ephemeral resource names. |
| `ephemeral schema [name]` | Full schema for one ephemeral resource, or all. |
| `version list` | All versions the registry advertises. |

### Examples

```bash
# List versions (OpenTofu registry by default).
tfpluginschema --ns hashicorp -n aws version list

# Provider configuration schema, pinned version.
tfpluginschema --ns hashicorp -n aws --vc 5.0.0 provider schema

# Just the resource type names for the latest version.
tfpluginschema --ns hashicorp -n aws resource list

# Schema for one resource.
tfpluginschema --ns hashicorp -n aws --vc 5.0.0 resource schema aws_instance

# Schema for one data source.
tfpluginschema --ns hashicorp -n aws --vc 5.0.0 datasource schema aws_ami

# Use the HashiCorp registry.
tfpluginschema -r terraform --ns Azure -n azapi resource list

# Dump every resource schema at once.
tfpluginschema --ns hashicorp -n aws --vc 5.0.0 resource schema
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

The library implements three levels of caching:

1. **Persistent on-disk provider cache**: Downloaded provider binaries are
   stored on disk in a predictable layout and reused across runs.
2. **In-memory download cache**: Prevents redundant work within a single
   `Server` instance.
3. **Schema cache**: Caches retrieved schemas to avoid repeated RPC calls.

The on-disk cache is preserved across runs. The in-memory caches are scoped
to the lifetime of a `Server` instance.

### Provider cache layout

Downloaded providers are extracted into a registry-qualified, namespaced path:

```
<cacheDir>/<registry-type>/<namespace>/terraform-provider-<name>/<version>/<os>_<arch>/
```

Where `<registry-type>` is `opentofu` or `terraform` (from `Request.RegistryType`).
Including the registry type and namespace avoids collisions between providers
with the same name and version published by different namespaces or registries.

The default `<cacheDir>` is `os.UserCacheDir()/tfpluginschema` (for example
`~/.cache/tfpluginschema` on Linux). It can be overridden with:

- The `TFPLUGINSCHEMA_CACHE_DIR` environment variable.
- The `--cache-dir` CLI flag.
- The `tfpluginschema.WithCacheDir("/path")` option to `NewServer`.

### Bypassing the cache

To always re-download providers, use:

- The `--force-fetch` CLI flag.
- The `tfpluginschema.WithForceFetch(true)` option passed to `NewServer`.

Current public API notes:

- `NewServer(l *slog.Logger, opts ...ServerOption) *Server` accepts a logger
  and zero or more `ServerOption` values.
- `Request` includes `RegistryType` in addition to provider-identifying fields
  such as namespace, name, and version.

### Observing cache hits / misses

The CLI prints `cache hit:` or `downloading:` messages to stderr for each
request. Pass `--quiet` to suppress them. Library users can register a
callback to observe the same signal:

```go
server := tfpluginschema.NewServer(nil,
    tfpluginschema.WithCacheStatusFunc(func(req tfpluginschema.Request, status tfpluginschema.CacheStatus) {
        log.Printf("%s: %s/%s %s", status, req.Namespace, req.Name, req.Version)
    }),
)
```

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
- Temporary files and legacy temporary directories are cleaned up when `Server.Cleanup()` is called
- The library handles cross-platform provider downloads automatically
- Base64-encoded type information in schemas is automatically decoded for easier consumption

## Acknowledgements

Thanks to the OpenTofu community for their contributions and the maintainers of the OpenTofu plugin protocol.
This library builds upon their work to provide a seamless experience for Go developers working with Terraform and OpenTofu providers.
