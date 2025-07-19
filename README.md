# tfpluginschema

A ***work in progress*** Go library for interacting with Terraform provider plugins and retrieving their schemas via gRPC. This library supports both Terraform plugin protocol versions 5 and 6, providing a unified interface for downloading, communicating with, and extracting schema information from Terraform providers.

## Features

- **Multi-protocol support**: Works with both Terraform plugin protocol v5 and v6
- **Provider download**: Automatically download providers from the Terraform Registry
- **Schema extraction**: Retrieve provider, resource, and data source schemas as JSON
- **Universal client**: Automatically selects the best protocol version
- **gRPC communication**: Direct gRPC communication with provider binaries
- **Protobuf definitions**: Includes generated protobuf code for both protocol versions

## Installation

```bash
go get github.com/matt-FFFFFF/tfpluginschema
```

## License

This project is licensed under the Mozilla Public License Version 2.0. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please ensure that:
1. Code follows Go conventions
2. Tests are included for new functionality
3. Protocol buffer changes are properly generated
4. Documentation is updated accordingly

## Acknowledgements

Thanks to the OpenTofu community for their contributions and the maintainers of the OpenTofu plugin protocol. This library builds upon their work to provide a seamless experience for Go developers working with Terraform and OpenTofu providers.
