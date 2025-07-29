.PHONY: help
help:
	@echo "Makefile commands:"
	@echo "  tools - Install the Go plugins"
	@echo "  clean - Remove generated files"
	@echo "  generate - Run go generate on the project"

# Install the Go plugins
.PHONY: tools
tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Clean up generated files
.PHONY: clean
clean:
	rm -f tfprotov6/*.pb.go
	rm -f tfprotov5/*.pb.go

.PHONY: generate
generate:
	go generate ./...
