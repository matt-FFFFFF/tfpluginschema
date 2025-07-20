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

const (
	// ProviderPluginName is the name used to identify the provider plugin
	ProviderPluginName = "provider"
	MagicCookieKey     = "TF_PLUGIN_MAGIC_COOKIE"
	MagicCookieValue   = "d602bf8f470bc67ca7faa0386276bbdd4330efaf76d1a219cb4d6991ca9872b2"
)

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

// V5Provider is the interface for v5 protocol clients
type V5Provider interface {
	V5Schema(*tfprotov5.GetProviderSchemaRequest) (*tfplugin5.GetProviderSchema_Response, error)
}

// V6Provider is the interface for v6 protocol clients
type V6Provider interface {
	V6Schema(*tfprotov6.GetProviderSchemaRequest) (*tfplugin6.GetProviderSchema_Response, error)
}

type ProviderSchemaProvider[S any, F any] interface {
	GetDataSourceSchemas() map[string]*S
	GetResourceSchemas() map[string]*F
	GetProvider() *S
	GetEphemeralResourceSchemas() map[string]*S
	GetFunctions() map[string]*F
}

type ProviderSchemaProviderV5 interface {
	ProviderSchemaProvider[tfplugin5.Schema, tfplugin5.Function]
}

type ProviderSchemaProviderV6 interface {
	ProviderSchemaProvider[tfplugin6.Schema, tfplugin6.Function]
}

// NewClientV5 creates a new provider client for protocol v5
func NewClientV5(providerPath string) (V5Provider, func(), error) {
	// Create the plugin client
	return newClient[V5Provider](providerPath, 5)
}

// NewClientV6 creates a new provider client for protocol v6
func NewClientV6(providerPath string) (V6Provider, func(), error) {
	// Create the plugin client
	return newClient[V6Provider](providerPath, 6)
}

func newClient[T any](providerPath string, protocolVersion int) (T, func(), error) {
	var ret T

	handshakeConfig := plugin.HandshakeConfig{
		ProtocolVersion:  uint(protocolVersion),
		MagicCookieKey:   MagicCookieKey,
		MagicCookieValue: MagicCookieValue,
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: handshakeConfig,
		Plugins: map[string]plugin.Plugin{
			ProviderPluginName: ProviderGRPCPlugin{ProtocolVersion: protocolVersion},
		},
		Cmd:              exec.Command(providerPath),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           hclog.New(&hclog.LoggerOptions{Level: hclog.Error}),
	})

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return ret, nil, fmt.Errorf("failed to create RPC client: %w", err)
	}

	// Request the plugin
	raw, err := rpcClient.Dispense(ProviderPluginName)
	if err != nil {
		client.Kill()
		return ret, nil, fmt.Errorf("failed to dispense provider: %w", err)
	}

	// Cast to our interface
	provider, ok := raw.(T)
	if !ok {
		client.Kill()
		return ret, nil, fmt.Errorf("plugin does not implement %T interface", ret)
	}

	return provider, client.Kill, nil
}
