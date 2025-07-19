package tfpluginschema

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/matt-FFFFFF/tfpluginschema/tfplugin5"
	"github.com/matt-FFFFFF/tfpluginschema/tfplugin6"
	"google.golang.org/grpc"
)

// Handshake config for go-plugin
var handshakeConfigV5 = plugin.HandshakeConfig{
	ProtocolVersion:  5,
	MagicCookieKey:   "TF_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "d602bf8f470bc67ca7faa0386276bbdd4330efaf76d1a219cb4d6991ca9872b2",
}

var handshakeConfigV6 = plugin.HandshakeConfig{
	ProtocolVersion:  6,
	MagicCookieKey:   "TF_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "d602bf8f470bc67ca7faa0386276bbdd4330efaf76d1a219cb4d6991ca9872b2",
}

// ProviderGRPCPlugin implements the plugin.GRPCPlugin interface for connecting to provider binaries
type ProviderGRPCPlugin struct {
	plugin.Plugin
	ProtocolVersion int // 5 or 6
}

// GRPCClient returns the client implementation using the gRPC connection
func (p ProviderGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	if p.ProtocolVersion == 5 {
		return &providerGRPCClientV5{
			grpcClient: tfplugin5.NewProviderClient(c),
		}, nil
	}
	return &providerGRPCClientV6{
		grpcClient: tfplugin6.NewProviderClient(c),
	}, nil
}

// GRPCServer is not implemented as we're only acting as a client
func (p ProviderGRPCPlugin) GRPCServer(*plugin.GRPCBroker, *grpc.Server) error {
	return fmt.Errorf("provider plugin only supports client mode")
}

// providerGRPCClientV5 wraps the gRPC client for protocol v5
type providerGRPCClientV5 struct {
	grpcClient tfplugin5.ProviderClient
}

// V5Schema calls GetSchema on the provider and returns the protobuf response
func (c *providerGRPCClientV5) V5Schema(req *tfprotov5.GetProviderSchemaRequest) (*tfplugin5.GetProviderSchema_Response, error) {
	protoReq := &tfplugin5.GetProviderSchema_Request{} // Empty request
	protoResp, err := c.grpcClient.GetSchema(context.Background(), protoReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider schema: %w", err)
	}
	return protoResp, nil
}

// providerGRPCClientV6 wraps the gRPC client for protocol v6
type providerGRPCClientV6 struct {
	grpcClient tfplugin6.ProviderClient
}

// V6Schema calls GetProviderSchema on the provider and returns the protobuf response
func (c *providerGRPCClientV6) V6Schema(req *tfprotov6.GetProviderSchemaRequest) (*tfplugin6.GetProviderSchema_Response, error) {
	protoReq := &tfplugin6.GetProviderSchema_Request{} // Empty request
	protoResp, err := c.grpcClient.GetProviderSchema(context.Background(), protoReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider schema: %w", err)
	}
	return protoResp, nil
}

// convertSchemaFromProto converts a protobuf Schema to tfprotov6.Schema
func convertSchemaFromProto(protoSchema *tfplugin6.Schema) *tfprotov6.Schema {
	if protoSchema == nil {
		return nil
	}

	result := &tfprotov6.Schema{
		Version: protoSchema.Version,
	}

	if protoSchema.Block != nil {
		result.Block = convertSchemaBlockFromProto(protoSchema.Block)
	}

	return result
}

// convertSchemaBlockFromProto converts a protobuf Schema_Block to tfprotov6.SchemaBlock
func convertSchemaBlockFromProto(protoBlock *tfplugin6.Schema_Block) *tfprotov6.SchemaBlock {
	if protoBlock == nil {
		return nil
	}

	result := &tfprotov6.SchemaBlock{
		Version:     protoBlock.Version,
		Description: protoBlock.Description,
		Deprecated:  protoBlock.Deprecated,
	}

	// Convert attributes
	if len(protoBlock.Attributes) > 0 {
		result.Attributes = make([]*tfprotov6.SchemaAttribute, 0, len(protoBlock.Attributes))
		for _, attr := range protoBlock.Attributes {
			converted := convertSchemaAttributeFromProto(attr)
			if converted != nil {
				result.Attributes = append(result.Attributes, converted)
			}
		}
	}

	// Convert block types
	if len(protoBlock.BlockTypes) > 0 {
		result.BlockTypes = make([]*tfprotov6.SchemaNestedBlock, 0, len(protoBlock.BlockTypes))
		for _, blockType := range protoBlock.BlockTypes {
			converted := convertSchemaNestedBlockFromProto(blockType)
			if converted != nil {
				result.BlockTypes = append(result.BlockTypes, converted)
			}
		}
	}

	return result
}

// convertSchemaAttributeFromProto converts a protobuf Schema_Attribute to tfprotov6.SchemaAttribute
func convertSchemaAttributeFromProto(protoAttr *tfplugin6.Schema_Attribute) *tfprotov6.SchemaAttribute {
	if protoAttr == nil {
		return nil
	}

	// For now, we'll create a basic attribute without the type
	// The type field requires tftypes which is complex to handle here
	return &tfprotov6.SchemaAttribute{
		Name:        protoAttr.Name,
		Description: protoAttr.Description,
		Required:    protoAttr.Required,
		Optional:    protoAttr.Optional,
		Computed:    protoAttr.Computed,
		Sensitive:   protoAttr.Sensitive,
		Deprecated:  protoAttr.Deprecated,
		// Type would need proper tftypes.Type conversion from protoAttr.Type
	}
}

// convertSchemaNestedBlockFromProto converts a protobuf Schema_NestedBlock to tfprotov6.SchemaNestedBlock
func convertSchemaNestedBlockFromProto(protoBlock *tfplugin6.Schema_NestedBlock) *tfprotov6.SchemaNestedBlock {
	if protoBlock == nil {
		return nil
	}

	var nesting tfprotov6.SchemaNestedBlockNestingMode
	switch protoBlock.Nesting {
	case tfplugin6.Schema_NestedBlock_INVALID:
		nesting = tfprotov6.SchemaNestedBlockNestingModeInvalid
	case tfplugin6.Schema_NestedBlock_SINGLE:
		nesting = tfprotov6.SchemaNestedBlockNestingModeSingle
	case tfplugin6.Schema_NestedBlock_LIST:
		nesting = tfprotov6.SchemaNestedBlockNestingModeList
	case tfplugin6.Schema_NestedBlock_SET:
		nesting = tfprotov6.SchemaNestedBlockNestingModeSet
	case tfplugin6.Schema_NestedBlock_MAP:
		nesting = tfprotov6.SchemaNestedBlockNestingModeMap
	case tfplugin6.Schema_NestedBlock_GROUP:
		nesting = tfprotov6.SchemaNestedBlockNestingModeGroup
	default:
		nesting = tfprotov6.SchemaNestedBlockNestingModeInvalid
	}

	result := &tfprotov6.SchemaNestedBlock{
		TypeName: protoBlock.TypeName,
		Nesting:  nesting,
	}

	if protoBlock.Block != nil {
		result.Block = convertSchemaBlockFromProto(protoBlock.Block)
	}

	return result
}

// V5ProviderSchema is the interface for v5 protocol clients
type V5ProviderSchema interface {
	V5Schema(*tfprotov5.GetProviderSchemaRequest) (*tfplugin5.GetProviderSchema_Response, error)
}

// V6ProviderSchema is the interface for v6 protocol clients
type V6ProviderSchema interface {
	V6Schema(*tfprotov6.GetProviderSchemaRequest) (*tfplugin6.GetProviderSchema_Response, error)
}

// NewClientV5 creates a new provider client for protocol v5
func NewClientV5(providerPath string) (V5ProviderSchema, error) {
	// Create the plugin client
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: handshakeConfigV5,
		Plugins: map[string]plugin.Plugin{
			"provider": ProviderGRPCPlugin{ProtocolVersion: 5},
		},
		Cmd:              exec.Command(providerPath),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           hclog.New(&hclog.LoggerOptions{Level: hclog.Error}),
	})

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to create RPC client: %w", err)
	}

	// Request the plugin
	raw, err := rpcClient.Dispense("provider")
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to dispense provider: %w", err)
	}

	// Cast to our interface
	provider, ok := raw.(V5ProviderSchema)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("plugin does not implement V5ProviderSchema interface")
	}

	return provider, nil
}

// NewClientV6 creates a new provider client for protocol v6
func NewClientV6(providerPath string) (V6ProviderSchema, error) {
	// Create the plugin client
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: handshakeConfigV6,
		Plugins: map[string]plugin.Plugin{
			"provider": ProviderGRPCPlugin{ProtocolVersion: 6},
		},
		Cmd:              exec.Command(providerPath),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           hclog.New(&hclog.LoggerOptions{Level: hclog.Error}),
	})

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to create RPC client: %w", err)
	}

	// Request the plugin
	raw, err := rpcClient.Dispense("provider")
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to dispense provider: %w", err)
	}

	// Cast to our interface
	provider, ok := raw.(V6ProviderSchema)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("plugin does not implement V6ProviderSchema interface")
	}

	return provider, nil
}
