package tfpluginschema

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/matt-FFFFFF/tfpluginschema/tfplugin5"
	"github.com/matt-FFFFFF/tfpluginschema/tfplugin6"
	"google.golang.org/grpc"

	// terraform-json provides the unified ProviderSchema type we use
	tfjson "github.com/hashicorp/terraform-json"
	// cty is the target type system used by terraform-json
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

const (
	// providerPluginName is the name used to identify the provider plugin
	providerPluginName = "provider"
	// magicCookieKey is the key used for the magic cookie in the plugin handshake
	magicCookieKey = "TF_PLUGIN_MAGIC_COOKIE"
	// magicCookieValue is the value used for the magic cookie in the plugin handshake
	magicCookieValue = "d602bf8f470bc67ca7faa0386276bbdd4330efaf76d1a219cb4d6991ca9872b2"
)

var (
	// ErrNotImplemented is returned when a method is not implemented
	ErrNotImplemented = errors.New("not implemented")
)

// providerGRPCPlugin implements the plugin.GRPCPlugin interface for connecting to provider binaries
type providerGRPCPlugin struct {
	plugin.Plugin
	protocolVersion int // 5 or 6
}

// GRPCClient returns the client implementation using the gRPC connection.
// Must be exported for the plugin framework to use it.
func (p providerGRPCPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	if p.protocolVersion == 5 {
		return &providerGRPCClientV5{
			providerGRPCClient: &providerGRPCClient[*tfplugin5.GetProviderSchema_Request, *tfplugin5.GetProviderSchema_Response]{
				grpcClient: v5SchemaClient{client: tfplugin5.NewProviderClient(c)},
			},
		}, nil
	}
	return &providerGRPCClientV6{
		providerGRPCClient: &providerGRPCClient[*tfplugin6.GetProviderSchema_Request, *tfplugin6.GetProviderSchema_Response]{
			grpcClient: v6SchemaClient{client: tfplugin6.NewProviderClient(c)},
		},
	}, nil
}

// GRPCServer is not implemented as we're only acting as a client
func (p providerGRPCPlugin) GRPCServer(*plugin.GRPCBroker, *grpc.Server) error {
	return ErrNotImplemented
}

// schemaClient defines the interface for clients that can retrieve schemas.
// Required as the v5 and v6 clients have different method signatures.
type schemaClient[TReq, TResp any] interface {
	getSchema(ctx context.Context, req TReq, opts ...grpc.CallOption) (TResp, error)
}

// v5SchemaClient adapts tfplugin5.ProviderClient to the schemaClient interface.
type v5SchemaClient struct {
	client tfplugin5.ProviderClient
}

// getSchema calls GetSchema on the V5 client and implements the schemaClient interface.
func (c v5SchemaClient) getSchema(ctx context.Context, req *tfplugin5.GetProviderSchema_Request, opts ...grpc.CallOption) (*tfplugin5.GetProviderSchema_Response, error) {
	return c.client.GetSchema(ctx, req, opts...)
}

// v6SchemaClient adapts tfplugin6.ProviderClient to the schemaClient interface.
type v6SchemaClient struct {
	client tfplugin6.ProviderClient
}

// getSchema calls GetProviderSchema on the V6 client and implements the schemaClient interface.
func (c v6SchemaClient) getSchema(ctx context.Context, req *tfplugin6.GetProviderSchema_Request, opts ...grpc.CallOption) (*tfplugin6.GetProviderSchema_Response, error) {
	return c.client.GetProviderSchema(ctx, req, opts...)
}

// providerGRPCClient is a generic wrapper for gRPC clients
type providerGRPCClient[TReq, TResp any] struct {
	grpcClient schemaClient[TReq, TResp]
}

// Schema calls GetSchema on the provider and returns the protobuf response
func (c *providerGRPCClient[TReq, TResp]) Schema(req TReq) (TResp, error) {
	var zeroResp TResp
	protoResp, err := c.grpcClient.getSchema(context.Background(), req)
	if err != nil {
		return zeroResp, fmt.Errorf("failed to get provider schema: %w", err)
	}
	return protoResp, nil
}

// providerGRPCClientV5 wraps the gRPC client for protocol v5
type providerGRPCClientV5 struct {
	*providerGRPCClient[*tfplugin5.GetProviderSchema_Request, *tfplugin5.GetProviderSchema_Response]
}

// v5Schema calls GetSchema on the provider and returns the protobuf response
func (c *providerGRPCClientV5) v5Schema() (*tfplugin5.GetProviderSchema_Response, error) {
	protoReq := &tfplugin5.GetProviderSchema_Request{} // Empty request
	return c.Schema(protoReq)
}

// providerGRPCClientV6 wraps the gRPC client for protocol v6
type providerGRPCClientV6 struct {
	*providerGRPCClient[*tfplugin6.GetProviderSchema_Request, *tfplugin6.GetProviderSchema_Response]
}

// v6Schema calls GetProviderSchema on the provider and returns the protobuf response
func (c *providerGRPCClientV6) v6Schema() (*tfplugin6.GetProviderSchema_Response, error) {
	protoReq := &tfplugin6.GetProviderSchema_Request{} // Empty request
	return c.Schema(protoReq)
}

// universalProvider provides a unified interface that works with both V5 and V6 protocols
type universalProvider interface {
	v5Schema() (*tfplugin5.GetProviderSchema_Response, error)
	v6Schema() (*tfplugin6.GetProviderSchema_Response, error)
	// schema returns a unified terraform-json ProviderSchema representation for either protocol
	schema() (*tfjson.ProviderSchema, error)
	close()
}

// newGrpcClient creates a provider client that supports both V5 and V6 protocols.
func newGrpcClient(providerPath string) (universalProvider, error) {
	// No need for ProtocolVersion here as we are using VersionedPlugins
	handshakeConfig := plugin.HandshakeConfig{
		MagicCookieKey:   magicCookieKey,
		MagicCookieValue: magicCookieValue,
	}

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: handshakeConfig,
		VersionedPlugins: map[int]plugin.PluginSet{
			5: {providerPluginName: providerGRPCPlugin{protocolVersion: 5}},
			6: {providerPluginName: providerGRPCPlugin{protocolVersion: 6}},
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
	raw, err := rpcClient.Dispense(providerPluginName)
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to dispense provider: %w", err)
	}

	// The plugin framework will return either a V5 or V6 client based on negotiation
	// We need to wrap it in a universal client that supports both interfaces
	if v5Client, ok := raw.(*providerGRPCClientV5); ok {
		return &universalProviderClient{
			v5:        v5Client,
			closeFunc: client.Kill,
		}, nil
	}
	if v6Client, ok := raw.(*providerGRPCClientV6); ok {
		return &universalProviderClient{
			v6:        v6Client,
			closeFunc: client.Kill,
		}, nil
	}

	client.Kill()
	return nil, fmt.Errorf("plugin returned unexpected type: %T", raw)
}

// universalProviderClient implements UniversalProvider and wraps either V5 or V6 clients
type universalProviderClient struct {
	v5        *providerGRPCClientV5
	v6        *providerGRPCClientV6
	closeFunc func()
}

func (c *universalProviderClient) v5Schema() (*tfplugin5.GetProviderSchema_Response, error) {
	if c.v5 != nil {
		return c.v5.v5Schema()
	}
	return nil, fmt.Errorf("V5 protocol not supported by this provider")
}

func (c *universalProviderClient) v6Schema() (*tfplugin6.GetProviderSchema_Response, error) {
	if c.v6 != nil {
		return c.v6.v6Schema()
	}
	return nil, fmt.Errorf("V6 protocol not supported by this provider")
}

func (c *universalProviderClient) close() {
	if c.closeFunc != nil {
		c.closeFunc()
	}
	c.v5 = nil
	c.v6 = nil
}

// schema returns a unified terraform-json ProviderSchema regardless of whether the underlying
// provider uses protocol v5 or v6. It prefers v6 when available and falls back to v5.
func (c *universalProviderClient) schema() (*tfjson.ProviderSchema, error) {
	// Prefer v6
	if c.v6 != nil {
		resp, err := c.v6.v6Schema()
		if err == nil {
			ps, convErr := convertV6ResponseToTFJSON(resp)
			if convErr != nil {
				return nil, fmt.Errorf("failed to convert v6 response: %w", convErr)
			}
			return ps, nil
		}
	}

	// Fallback to v5
	if c.v5 != nil {
		resp, err := c.v5.v5Schema()
		if err == nil {
			ps, convErr := convertV5ResponseToTFJSON(resp)
			if convErr != nil {
				return nil, fmt.Errorf("failed to convert v5 response: %w", convErr)
			}
			return ps, nil
		}
	}

	return nil, fmt.Errorf("failed to get provider schema for either V5 or V6 protocols")
}

// Conversion helpers ------------------------------------------------------

// convertV6ResponseToTFJSON converts a tfplugin6 GetProviderSchema_Response into a terraform-json ProviderSchema
func convertV6ResponseToTFJSON(resp *tfplugin6.GetProviderSchema_Response) (*tfjson.ProviderSchema, error) {
	if resp == nil {
		return nil, fmt.Errorf("nil v6 response")
	}

	ps := &tfjson.ProviderSchema{}

	// Provider / Config schema
	if resp.Provider != nil {
		ps.ConfigSchema = convertV6SchemaToTFJSON(resp.Provider)
	}

	// Resource schemas
	if len(resp.ResourceSchemas) > 0 {
		ps.ResourceSchemas = make(map[string]*tfjson.Schema, len(resp.ResourceSchemas))
		for k, v := range resp.ResourceSchemas {
			ps.ResourceSchemas[k] = convertV6SchemaToTFJSON(v)
		}
	}

	// Data source schemas
	if len(resp.DataSourceSchemas) > 0 {
		ps.DataSourceSchemas = make(map[string]*tfjson.Schema, len(resp.DataSourceSchemas))
		for k, v := range resp.DataSourceSchemas {
			ps.DataSourceSchemas[k] = convertV6SchemaToTFJSON(v)
		}
	}

	// Ephemeral resource schemas
	if len(resp.EphemeralResourceSchemas) > 0 {
		ps.EphemeralResourceSchemas = make(map[string]*tfjson.Schema, len(resp.EphemeralResourceSchemas))
		for k, v := range resp.EphemeralResourceSchemas {
			ps.EphemeralResourceSchemas[k] = convertV6SchemaToTFJSON(v)
		}
	}

	// Functions
	if len(resp.Functions) > 0 {
		ps.Functions = make(map[string]*tfjson.FunctionSignature, len(resp.Functions))
		for k, v := range resp.Functions {
			ps.Functions[k] = convertV6FunctionToTFJSON(v)
		}
	}

	// Note: GetProviderSchema does not include resource identity schemas in the v6 response.
	// Those are available via a separate RPC. Leave ResourceIdentitySchemas nil for now.

	return ps, nil
}

// convertV5ResponseToTFJSON converts a tfplugin5 GetProviderSchema_Response into a terraform-json ProviderSchema
func convertV5ResponseToTFJSON(resp *tfplugin5.GetProviderSchema_Response) (*tfjson.ProviderSchema, error) {
	if resp == nil {
		return nil, fmt.Errorf("nil v5 response")
	}

	ps := &tfjson.ProviderSchema{}

	// Provider / Config schema
	if resp.Provider != nil {
		ps.ConfigSchema = convertV5SchemaToTFJSON(resp.Provider)
	}

	// Resource schemas
	if len(resp.ResourceSchemas) > 0 {
		ps.ResourceSchemas = make(map[string]*tfjson.Schema, len(resp.ResourceSchemas))
		for k, v := range resp.ResourceSchemas {
			ps.ResourceSchemas[k] = convertV5SchemaToTFJSON(v)
		}
	}

	// Data source schemas
	if len(resp.DataSourceSchemas) > 0 {
		ps.DataSourceSchemas = make(map[string]*tfjson.Schema, len(resp.DataSourceSchemas))
		for k, v := range resp.DataSourceSchemas {
			ps.DataSourceSchemas[k] = convertV5SchemaToTFJSON(v)
		}
	}

	// Ephemeral resource schemas
	if len(resp.EphemeralResourceSchemas) > 0 {
		ps.EphemeralResourceSchemas = make(map[string]*tfjson.Schema, len(resp.EphemeralResourceSchemas))
		for k, v := range resp.EphemeralResourceSchemas {
			ps.EphemeralResourceSchemas[k] = convertV5SchemaToTFJSON(v)
		}
	}

	// Functions
	if len(resp.Functions) > 0 {
		ps.Functions = make(map[string]*tfjson.FunctionSignature, len(resp.Functions))
		for k, v := range resp.Functions {
			ps.Functions[k] = convertV5FunctionToTFJSON(v)
		}
	}

	return ps, nil
}

// convertV6SchemaToTFJSON converts a proto v6 Schema into a terraform-json Schema
func convertV6SchemaToTFJSON(s *tfplugin6.Schema) *tfjson.Schema {
	if s == nil {
		return nil
	}
	return &tfjson.Schema{
		Version: uint64(s.GetVersion()),
		Block:   convertV6BlockToTFJSON(s.GetBlock()),
	}
}

func convertV6BlockToTFJSON(b *tfplugin6.Schema_Block) *tfjson.SchemaBlock {
	if b == nil {
		return nil
	}
	sb := &tfjson.SchemaBlock{
		Description:     b.GetDescription(),
		DescriptionKind: tfjson.SchemaDescriptionKindPlain,
		Deprecated:      b.GetDeprecated(),
	}

	// Description kind
	switch b.GetDescriptionKind() {
	case tfplugin6.StringKind_MARKDOWN:
		sb.DescriptionKind = tfjson.SchemaDescriptionKindMarkdown
	default:
		sb.DescriptionKind = tfjson.SchemaDescriptionKindPlain
	}

	// Attributes
	if len(b.GetAttributes()) > 0 {
		sb.Attributes = make(map[string]*tfjson.SchemaAttribute, len(b.GetAttributes()))
		for _, a := range b.GetAttributes() {
			sa := &tfjson.SchemaAttribute{
				Description:     a.GetDescription(),
				Deprecated:      a.GetDeprecated(),
				Required:        a.GetRequired(),
				Optional:        a.GetOptional(),
				Computed:        a.GetComputed(),
				Sensitive:       a.GetSensitive(),
				WriteOnly:       a.GetWriteOnly(),
				DescriptionKind: tfjson.SchemaDescriptionKindPlain,
			}
			switch a.GetDescriptionKind() {
			case tfplugin6.StringKind_MARKDOWN:
				sa.DescriptionKind = tfjson.SchemaDescriptionKindMarkdown
			default:
				sa.DescriptionKind = tfjson.SchemaDescriptionKindPlain
			}

			// Attribute type (bytes contain JSON type signature). Prefer explicit type
			if tbytes := a.GetType(); len(tbytes) > 0 {
				if ctyType, err := decodeCtyTypeFromJSONBytes(tbytes); err == nil {
					sa.AttributeType = ctyType
				}
			}

			// Nested type
			if a.NestedType != nil {
				sa.AttributeNestedType = convertV6ObjectToNested(a.NestedType)
			}

			sb.Attributes[a.GetName()] = sa
		}
	}

	// Block types
	if len(b.GetBlockTypes()) > 0 {
		sb.NestedBlocks = make(map[string]*tfjson.SchemaBlockType, len(b.GetBlockTypes()))
		for _, nb := range b.GetBlockTypes() {
			bt := &tfjson.SchemaBlockType{
				Block:    convertV6BlockToTFJSON(nb.GetBlock()),
				MinItems: uint64(nb.GetMinItems()),
				MaxItems: uint64(nb.GetMaxItems()),
			}
			switch nb.GetNesting() {
			case tfplugin6.Schema_NestedBlock_SINGLE:
				bt.NestingMode = tfjson.SchemaNestingModeSingle
			case tfplugin6.Schema_NestedBlock_GROUP:
				bt.NestingMode = tfjson.SchemaNestingModeGroup
			case tfplugin6.Schema_NestedBlock_LIST:
				bt.NestingMode = tfjson.SchemaNestingModeList
			case tfplugin6.Schema_NestedBlock_SET:
				bt.NestingMode = tfjson.SchemaNestingModeSet
			case tfplugin6.Schema_NestedBlock_MAP:
				bt.NestingMode = tfjson.SchemaNestingModeMap
			default:
				bt.NestingMode = tfjson.SchemaNestingModeSingle
			}
			sb.NestedBlocks[nb.GetTypeName()] = bt
		}
	}

	return sb
}

func convertV6ObjectToNested(o *tfplugin6.Schema_Object) *tfjson.SchemaNestedAttributeType {
	if o == nil {
		return nil
	}
	n := &tfjson.SchemaNestedAttributeType{}
	if len(o.GetAttributes()) > 0 {
		n.Attributes = make(map[string]*tfjson.SchemaAttribute, len(o.GetAttributes()))
		for _, a := range o.GetAttributes() {
			sa := &tfjson.SchemaAttribute{
				Description:     a.GetDescription(),
				Deprecated:      a.GetDeprecated(),
				Required:        a.GetRequired(),
				Optional:        a.GetOptional(),
				Computed:        a.GetComputed(),
				Sensitive:       a.GetSensitive(),
				WriteOnly:       a.GetWriteOnly(),
				DescriptionKind: tfjson.SchemaDescriptionKindPlain,
			}
			switch a.GetDescriptionKind() {
			case tfplugin6.StringKind_MARKDOWN:
				sa.DescriptionKind = tfjson.SchemaDescriptionKindMarkdown
			default:
				sa.DescriptionKind = tfjson.SchemaDescriptionKindPlain
			}
			// Attribute type
			if tbytes := a.GetType(); len(tbytes) > 0 {
				if ctyType, err := decodeCtyTypeFromJSONBytes(tbytes); err == nil {
					sa.AttributeType = ctyType
				}
			}
			if a.NestedType != nil {
				sa.AttributeNestedType = convertV6ObjectToNested(a.NestedType)
			}
			n.Attributes[a.GetName()] = sa
		}
	}

	switch o.GetNesting() {
	case tfplugin6.Schema_Object_SINGLE:
		n.NestingMode = tfjson.SchemaNestingModeSingle
	case tfplugin6.Schema_Object_LIST:
		n.NestingMode = tfjson.SchemaNestingModeList
	case tfplugin6.Schema_Object_SET:
		n.NestingMode = tfjson.SchemaNestingModeSet
	case tfplugin6.Schema_Object_MAP:
		n.NestingMode = tfjson.SchemaNestingModeMap
	default:
		n.NestingMode = tfjson.SchemaNestingModeSingle
	}

	// Note: MinItems/MaxItems on Schema_Object are deprecated in protocol; omit copying.

	return n
}

func convertV6FunctionToTFJSON(f *tfplugin6.Function) *tfjson.FunctionSignature {
	if f == nil {
		return nil
	}
	fs := &tfjson.FunctionSignature{
		Summary:            f.GetSummary(),
		Description:        f.GetDescription(),
		DeprecationMessage: f.GetDeprecationMessage(),
	}

	if len(f.GetParameters()) > 0 {
		fs.Parameters = make([]*tfjson.FunctionParameter, len(f.GetParameters()))
		for i, p := range f.GetParameters() {
			fs.Parameters[i] = &tfjson.FunctionParameter{
				Name:        p.GetName(),
				Description: p.GetDescription(),
				IsNullable:  p.GetAllowNullValue(),
			}
			if tbytes := p.GetType(); len(tbytes) > 0 {
				if ctyType, err := decodeCtyTypeFromJSONBytes(tbytes); err == nil {
					fs.Parameters[i].Type = ctyType
				}
			}
		}
	}

	if f.GetVariadicParameter() != nil {
		vp := f.GetVariadicParameter()
		fs.VariadicParameter = &tfjson.FunctionParameter{
			Name:        vp.GetName(),
			Description: vp.GetDescription(),
			IsNullable:  vp.GetAllowNullValue(),
		}
		if tbytes := vp.GetType(); len(tbytes) > 0 {
			if ctyType, err := decodeCtyTypeFromJSONBytes(tbytes); err == nil {
				fs.VariadicParameter.Type = ctyType
			}
		}
	}

	if r := f.GetReturn(); r != nil {
		if tbytes := r.GetType(); len(tbytes) > 0 {
			if ctyType, err := decodeCtyTypeFromJSONBytes(tbytes); err == nil {
				fs.ReturnType = ctyType
			}
		}
	}

	return fs
}

// convertV5 helpers just map to the v6 converters because the proto shapes are equivalent
func convertV5SchemaToTFJSON(s *tfplugin5.Schema) *tfjson.Schema {
	if s == nil {
		return nil
	}
	return &tfjson.Schema{
		Version: uint64(s.GetVersion()),
		Block:   convertV5BlockToTFJSON(s.GetBlock()),
	}
}

func convertV5BlockToTFJSON(b *tfplugin5.Schema_Block) *tfjson.SchemaBlock {
	if b == nil {
		return nil
	}
	// Reuse v6 implementation by mapping types
	sb := &tfjson.SchemaBlock{
		Description:     b.GetDescription(),
		DescriptionKind: tfjson.SchemaDescriptionKindPlain,
		Deprecated:      b.GetDeprecated(),
	}

	switch b.GetDescriptionKind() {
	case tfplugin5.StringKind_MARKDOWN:
		sb.DescriptionKind = tfjson.SchemaDescriptionKindMarkdown
	default:
		sb.DescriptionKind = tfjson.SchemaDescriptionKindPlain
	}

	if len(b.GetAttributes()) > 0 {
		sb.Attributes = make(map[string]*tfjson.SchemaAttribute, len(b.GetAttributes()))
		for _, a := range b.GetAttributes() {
			sa := &tfjson.SchemaAttribute{
				Description:     a.GetDescription(),
				Deprecated:      a.GetDeprecated(),
				Required:        a.GetRequired(),
				Optional:        a.GetOptional(),
				Computed:        a.GetComputed(),
				Sensitive:       a.GetSensitive(),
				WriteOnly:       a.GetWriteOnly(),
				DescriptionKind: tfjson.SchemaDescriptionKindPlain,
			}
			switch a.GetDescriptionKind() {
			case tfplugin5.StringKind_MARKDOWN:
				sa.DescriptionKind = tfjson.SchemaDescriptionKindMarkdown
			default:
				sa.DescriptionKind = tfjson.SchemaDescriptionKindPlain
			}

			// Nested type (not available in v5 Attribute schema)
			// v5 uses raw type bytes on the Attribute, so we avoid attempting to access NestedType here
			// if a.NestedType != nil {
			//     sa.AttributeNestedType = convertV5ObjectToNested(a.NestedType)
			// }

			// Attribute type
			if tbytes := a.GetType(); len(tbytes) > 0 {
				if ctyType, err := decodeCtyTypeFromJSONBytes(tbytes); err == nil {
					sa.AttributeType = ctyType
				}
			}

			sb.Attributes[a.GetName()] = sa
		}
	}

	if len(b.GetBlockTypes()) > 0 {
		sb.NestedBlocks = make(map[string]*tfjson.SchemaBlockType, len(b.GetBlockTypes()))
		for _, nb := range b.GetBlockTypes() {
			bt := &tfjson.SchemaBlockType{
				Block:    convertV5BlockToTFJSON(nb.GetBlock()),
				MinItems: uint64(nb.GetMinItems()),
				MaxItems: uint64(nb.GetMaxItems()),
			}
			switch nb.GetNesting() {
			case tfplugin5.Schema_NestedBlock_SINGLE:
				bt.NestingMode = tfjson.SchemaNestingModeSingle
			case tfplugin5.Schema_NestedBlock_GROUP:
				bt.NestingMode = tfjson.SchemaNestingModeGroup
			case tfplugin5.Schema_NestedBlock_LIST:
				bt.NestingMode = tfjson.SchemaNestingModeList
			case tfplugin5.Schema_NestedBlock_SET:
				bt.NestingMode = tfjson.SchemaNestingModeSet
			case tfplugin5.Schema_NestedBlock_MAP:
				bt.NestingMode = tfjson.SchemaNestingModeMap
			default:
				bt.NestingMode = tfjson.SchemaNestingModeSingle
			}
			sb.NestedBlocks[nb.GetTypeName()] = bt
		}
	}

	return sb
}

func convertV5FunctionToTFJSON(f *tfplugin5.Function) *tfjson.FunctionSignature {
	if f == nil {
		return nil
	}
	fs := &tfjson.FunctionSignature{
		Summary:            f.GetSummary(),
		Description:        f.GetDescription(),
		DeprecationMessage: f.GetDeprecationMessage(),
	}

	if len(f.GetParameters()) > 0 {
		fs.Parameters = make([]*tfjson.FunctionParameter, len(f.GetParameters()))
		for i, p := range f.GetParameters() {
			fs.Parameters[i] = &tfjson.FunctionParameter{
				Name:        p.GetName(),
				Description: p.GetDescription(),
				IsNullable:  p.GetAllowNullValue(),
			}
			if tbytes := p.GetType(); len(tbytes) > 0 {
				if ctyType, err := decodeCtyTypeFromJSONBytes(tbytes); err == nil {
					fs.Parameters[i].Type = ctyType
				}
			}
		}
	}

	if f.GetVariadicParameter() != nil {
		vp := f.GetVariadicParameter()
		fs.VariadicParameter = &tfjson.FunctionParameter{
			Name:        vp.GetName(),
			Description: vp.GetDescription(),
			IsNullable:  vp.GetAllowNullValue(),
		}
		if tbytes := vp.GetType(); len(tbytes) > 0 {
			if ctyType, err := decodeCtyTypeFromJSONBytes(tbytes); err == nil {
				fs.VariadicParameter.Type = ctyType
			}
		}
	}

	if r := f.GetReturn(); r != nil {
		if tbytes := r.GetType(); len(tbytes) > 0 {
			if ctyType, err := decodeCtyTypeFromJSONBytes(tbytes); err == nil {
				fs.ReturnType = ctyType
			}
		}
	}

	return fs
}

// decodeCtyTypeFromJSONBytes attempts to parse provider-sent JSON type bytes into cty.Type.
// It first uses tftypes.ParseJSONType for robust decoding, then converts to cty.Type
// via JSON, falling back to direct cty/json parsing if needed.
func decodeCtyTypeFromJSONBytes(buf []byte) (cty.Type, error) {
	if len(buf) == 0 {
		return cty.NilType, fmt.Errorf("empty type bytes")
	}
	// Providers send JSON-encoded Terraform type signatures. Try cty/json first.
	if ty, err := ctyjson.UnmarshalType(buf); err == nil {
		return ty, nil
	}

	// Fallback: accept a minimal subset of common encodings like
	// {"list":"string"} and {"object":{"a":"number"}}
	// without pulling extra dependencies.
	var raw any
	if err := json.Unmarshal(buf, &raw); err != nil {
		return cty.NilType, err
	}
	switch v := raw.(type) {
	case string:
		return primitiveFromString(v)
	case map[string]any:
		if len(v) == 1 {
			for k, inner := range v {
				switch k {
				case "list":
					if s, ok := inner.(string); ok {
						et, err := primitiveFromString(s)
						if err != nil {
							return cty.NilType, err
						}
						return cty.List(et), nil
					}
				case "set":
					if s, ok := inner.(string); ok {
						et, err := primitiveFromString(s)
						if err != nil {
							return cty.NilType, err
						}
						return cty.Set(et), nil
					}
				case "map":
					if s, ok := inner.(string); ok {
						et, err := primitiveFromString(s)
						if err != nil {
							return cty.NilType, err
						}
						return cty.Map(et), nil
					}
				case "object":
					if obj, ok := inner.(map[string]any); ok {
						attrs := make(map[string]cty.Type, len(obj))
						for name, typ := range obj {
							s, ok := typ.(string)
							if !ok {
								return cty.NilType, fmt.Errorf("invalid object attribute type for %s", name)
							}
							pt, err := primitiveFromString(s)
							if err != nil {
								return cty.NilType, err
							}
							attrs[name] = pt
						}
						return cty.Object(attrs), nil
					}
				}
			}
		}
	}
	return cty.NilType, fmt.Errorf("invalid complex type description")
}

// primitiveFromString maps simple string names to cty primitive types.
func primitiveFromString(s string) (cty.Type, error) {
	switch s {
	case "string":
		return cty.String, nil
	case "number":
		return cty.Number, nil
	case "bool":
		return cty.Bool, nil
	default:
		return cty.NilType, fmt.Errorf("unsupported primitive type: %s", s)
	}
}
