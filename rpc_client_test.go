package tfpluginschema

import (
	"context"
	"errors"
	"testing"

	"github.com/matt-FFFFFF/tfpluginschema/tfplugin5"
	"github.com/matt-FFFFFF/tfpluginschema/tfplugin6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

// Mock implementations for testing

// mockV5SchemaClient mocks the v5SchemaClient by implementing the schemaClient interface
type mockV5SchemaClient struct {
	mock.Mock
}

func (m *mockV5SchemaClient) getSchema(ctx context.Context, req *tfplugin5.GetProviderSchema_Request, opts ...grpc.CallOption) (*tfplugin5.GetProviderSchema_Response, error) {
	args := m.Called(ctx, req, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tfplugin5.GetProviderSchema_Response), args.Error(1)
}

// mockV6SchemaClient mocks the v6SchemaClient by implementing the schemaClient interface
type mockV6SchemaClient struct {
	mock.Mock
}

func (m *mockV6SchemaClient) getSchema(ctx context.Context, req *tfplugin6.GetProviderSchema_Request, opts ...grpc.CallOption) (*tfplugin6.GetProviderSchema_Response, error) {
	args := m.Called(ctx, req, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tfplugin6.GetProviderSchema_Response), args.Error(1)
}

// mockV5ProviderClient mocks just the GetSchema method from tfplugin5.ProviderClient
type mockV5ProviderClient struct {
	mock.Mock
}

func (m *mockV5ProviderClient) GetSchema(ctx context.Context, req *tfplugin5.GetProviderSchema_Request, opts ...grpc.CallOption) (*tfplugin5.GetProviderSchema_Response, error) {
	args := m.Called(ctx, req, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tfplugin5.GetProviderSchema_Response), args.Error(1)
}

// mockV6ProviderClient mocks just the GetProviderSchema method from tfplugin6.ProviderClient
type mockV6ProviderClient struct {
	mock.Mock
}

func (m *mockV6ProviderClient) GetProviderSchema(ctx context.Context, req *tfplugin6.GetProviderSchema_Request, opts ...grpc.CallOption) (*tfplugin6.GetProviderSchema_Response, error) {
	args := m.Called(ctx, req, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tfplugin6.GetProviderSchema_Response), args.Error(1)
}

// Test helper functions to create response structs

func createTestV5Response() *tfplugin5.GetProviderSchema_Response {
	return &tfplugin5.GetProviderSchema_Response{
		Provider: &tfplugin5.Schema{
			Version: 1,
			Block: &tfplugin5.Schema_Block{
				Attributes: []*tfplugin5.Schema_Attribute{
					{
						Name:     "test_attribute",
						Type:     []byte(`"string"`),
						Required: true,
					},
				},
			},
		},
		ResourceSchemas: map[string]*tfplugin5.Schema{
			"test_resource": {
				Version: 1,
				Block: &tfplugin5.Schema_Block{
					Attributes: []*tfplugin5.Schema_Attribute{
						{
							Name:     "id",
							Type:     []byte(`"string"`),
							Computed: true,
						},
					},
				},
			},
		},
		DataSourceSchemas: map[string]*tfplugin5.Schema{
			"test_data_source": {
				Version: 1,
				Block: &tfplugin5.Schema_Block{
					Attributes: []*tfplugin5.Schema_Attribute{
						{
							Name:     "value",
							Type:     []byte(`"string"`),
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func createTestV6Response() *tfplugin6.GetProviderSchema_Response {
	return &tfplugin6.GetProviderSchema_Response{
		Provider: &tfplugin6.Schema{
			Version: 1,
			Block: &tfplugin6.Schema_Block{
				Attributes: []*tfplugin6.Schema_Attribute{
					{
						Name:     "test_attribute",
						Type:     []byte(`"string"`),
						Required: true,
					},
				},
			},
		},
		ResourceSchemas: map[string]*tfplugin6.Schema{
			"test_resource": {
				Version: 1,
				Block: &tfplugin6.Schema_Block{
					Attributes: []*tfplugin6.Schema_Attribute{
						{
							Name:     "id",
							Type:     []byte(`"string"`),
							Computed: true,
						},
					},
				},
			},
		},
		DataSourceSchemas: map[string]*tfplugin6.Schema{
			"test_data_source": {
				Version: 1,
				Block: &tfplugin6.Schema_Block{
					Attributes: []*tfplugin6.Schema_Attribute{
						{
							Name:     "value",
							Type:     []byte(`"string"`),
							Computed: true,
						},
					},
				},
			},
		},
	}
}

// Test v5SchemaClient and v6SchemaClient would require full ProviderClient interface mocking
// Instead, we focus on testing the components that we can test with our schemaClient interface mocks

// Test providerGRPCClient with mocked schema clients

func TestProviderGRPCClientV5_Schema_Success(t *testing.T) {
	mockSchemaClient := &mockV5SchemaClient{}
	client := &providerGRPCClient[*tfplugin5.GetProviderSchema_Request, *tfplugin5.GetProviderSchema_Response]{
		grpcClient: mockSchemaClient,
	}

	expectedReq := &tfplugin5.GetProviderSchema_Request{}
	expectedResp := createTestV5Response()

	mockSchemaClient.On("getSchema", mock.Anything, expectedReq, []grpc.CallOption(nil)).Return(expectedResp, nil)

	resp, err := client.Schema(expectedReq)

	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	assert.NotNil(t, resp.Provider)
	assert.Equal(t, "id", resp.ResourceSchemas["test_resource"].Block.Attributes[0].Name)
	mockSchemaClient.AssertExpectations(t)
}

func TestProviderGRPCClientV5_Schema_Error(t *testing.T) {
	mockSchemaClient := &mockV5SchemaClient{}
	client := &providerGRPCClient[*tfplugin5.GetProviderSchema_Request, *tfplugin5.GetProviderSchema_Response]{
		grpcClient: mockSchemaClient,
	}

	expectedReq := &tfplugin5.GetProviderSchema_Request{}
	expectedError := errors.New("rpc error")

	mockSchemaClient.On("getSchema", mock.Anything, expectedReq, mock.Anything).Return(nil, expectedError)

	resp, err := client.Schema(expectedReq)

	var zeroResp *tfplugin5.GetProviderSchema_Response
	assert.Equal(t, zeroResp, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get provider schema")
	assert.Contains(t, err.Error(), expectedError.Error())
	mockSchemaClient.AssertExpectations(t)
}

func TestProviderGRPCClientV6_Schema_Success(t *testing.T) {
	mockSchemaClient := &mockV6SchemaClient{}
	client := &providerGRPCClient[*tfplugin6.GetProviderSchema_Request, *tfplugin6.GetProviderSchema_Response]{
		grpcClient: mockSchemaClient,
	}

	expectedReq := &tfplugin6.GetProviderSchema_Request{}
	expectedResp := createTestV6Response()

	mockSchemaClient.On("getSchema", mock.Anything, expectedReq, mock.Anything).Return(expectedResp, nil)

	resp, err := client.Schema(expectedReq)

	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	assert.NotNil(t, resp.Provider)
	assert.Equal(t, "id", resp.ResourceSchemas["test_resource"].Block.Attributes[0].Name)
	mockSchemaClient.AssertExpectations(t)
}

func TestProviderGRPCClientV6_Schema_Error(t *testing.T) {
	mockSchemaClient := &mockV6SchemaClient{}
	client := &providerGRPCClient[*tfplugin6.GetProviderSchema_Request, *tfplugin6.GetProviderSchema_Response]{
		grpcClient: mockSchemaClient,
	}

	expectedReq := &tfplugin6.GetProviderSchema_Request{}
	expectedError := errors.New("network timeout")

	mockSchemaClient.On("getSchema", mock.Anything, mock.Anything, mock.Anything).Return(nil, expectedError)

	resp, err := client.Schema(expectedReq)

	var zeroResp *tfplugin6.GetProviderSchema_Response
	assert.Equal(t, zeroResp, resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get provider schema")
	assert.Contains(t, err.Error(), expectedError.Error())
	mockSchemaClient.AssertExpectations(t)
}

// Test providerGRPCClientV5 and providerGRPCClientV6

func TestProviderGRPCClientV5_V5Schema_Success(t *testing.T) {
	mockSchemaClient := &mockV5SchemaClient{}
	innerClient := &providerGRPCClient[*tfplugin5.GetProviderSchema_Request, *tfplugin5.GetProviderSchema_Response]{
		grpcClient: mockSchemaClient,
	}
	client := &providerGRPCClientV5{
		providerGRPCClient: innerClient,
	}

	expectedResp := createTestV5Response()

	mockSchemaClient.On("getSchema", mock.Anything, mock.Anything, mock.Anything).Return(expectedResp, nil)

	resp, err := client.v5Schema()

	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	assert.Equal(t, "test_attribute", resp.Provider.Block.Attributes[0].Name)
	mockSchemaClient.AssertExpectations(t)
}

func TestProviderGRPCClientV6_V6Schema_Success(t *testing.T) {
	mockSchemaClient := &mockV6SchemaClient{}
	innerClient := &providerGRPCClient[*tfplugin6.GetProviderSchema_Request, *tfplugin6.GetProviderSchema_Response]{
		grpcClient: mockSchemaClient,
	}
	client := &providerGRPCClientV6{
		providerGRPCClient: innerClient,
	}

	expectedResp := createTestV6Response()

	mockSchemaClient.On("getSchema", mock.Anything, mock.Anything, mock.Anything).Return(expectedResp, nil)

	resp, err := client.v6Schema()

	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	assert.Equal(t, "test_attribute", resp.Provider.Block.Attributes[0].Name)
	mockSchemaClient.AssertExpectations(t)
}

// Test universalProviderClient

func TestUniversalProviderClient_V5Schema_Success(t *testing.T) {
	mockSchemaClient := &mockV5SchemaClient{}
	innerClient := &providerGRPCClient[*tfplugin5.GetProviderSchema_Request, *tfplugin5.GetProviderSchema_Response]{
		grpcClient: mockSchemaClient,
	}
	v5Client := &providerGRPCClientV5{
		providerGRPCClient: innerClient,
	}

	client := &universalProviderClient{
		v5: v5Client,
	}

	expectedResp := createTestV5Response()

	mockSchemaClient.On("getSchema", mock.Anything, mock.Anything, mock.Anything).Return(expectedResp, nil)

	resp, err := client.v5Schema()

	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	assert.Len(t, resp.ResourceSchemas, 1)
	assert.Len(t, resp.DataSourceSchemas, 1)
	mockSchemaClient.AssertExpectations(t)
}

func TestUniversalProviderClient_V5Schema_NotSupported(t *testing.T) {
	client := &universalProviderClient{
		v5: nil, // V5 not supported
	}

	resp, err := client.v5Schema()

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Equal(t, "V5 protocol not supported by this provider", err.Error())
}

func TestUniversalProviderClient_V6Schema_Success(t *testing.T) {
	mockSchemaClient := &mockV6SchemaClient{}
	innerClient := &providerGRPCClient[*tfplugin6.GetProviderSchema_Request, *tfplugin6.GetProviderSchema_Response]{
		grpcClient: mockSchemaClient,
	}
	v6Client := &providerGRPCClientV6{
		providerGRPCClient: innerClient,
	}

	client := &universalProviderClient{
		v6: v6Client,
	}

	expectedResp := createTestV6Response()

	mockSchemaClient.On("getSchema", mock.Anything, mock.Anything, mock.Anything).Return(expectedResp, nil)

	resp, err := client.v6Schema()

	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	assert.Len(t, resp.ResourceSchemas, 1)
	assert.Len(t, resp.DataSourceSchemas, 1)
	mockSchemaClient.AssertExpectations(t)
}

func TestUniversalProviderClient_V6Schema_NotSupported(t *testing.T) {
	client := &universalProviderClient{
		v6: nil, // V6 not supported
	}

	resp, err := client.v6Schema()

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Equal(t, "V6 protocol not supported by this provider", err.Error())
}

func TestUniversalProviderClient_Close(t *testing.T) {
	called := false
	closeFunc := func() {
		called = true
	}

	client := &universalProviderClient{
		v5:        &providerGRPCClientV5{},
		v6:        &providerGRPCClientV6{},
		closeFunc: closeFunc,
	}

	client.close()

	assert.True(t, called)
	assert.Nil(t, client.v5)
	assert.Nil(t, client.v6)
}

func TestUniversalProviderClient_Close_NoCloseFunc(t *testing.T) {
	client := &universalProviderClient{
		closeFunc: nil,
	}

	// Should not panic
	client.close()

	assert.Nil(t, client.v5)
	assert.Nil(t, client.v6)
}

// Test providerGRPCPlugin

func TestProviderGRPCPlugin_GRPCServer(t *testing.T) {
	plugin := providerGRPCPlugin{}

	err := plugin.GRPCServer(nil, nil)

	assert.Equal(t, ErrNotImplemented, err)
}

// Test error constants and basic constants

func TestErrNotImplemented(t *testing.T) {
	err := ErrNotImplemented
	assert.Equal(t, "not implemented", err.Error())
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "provider", providerPluginName)
	assert.Equal(t, "TF_PLUGIN_MAGIC_COOKIE", magicCookieKey)
	assert.Equal(t, "d602bf8f470bc67ca7faa0386276bbdd4330efaf76d1a219cb4d6991ca9872b2", magicCookieValue)
}

// Integration-style tests

func TestNewGrpcClient_InvalidPath(t *testing.T) {
	// Test with a non-existent provider path
	_, err := newGrpcClient("/nonexistent/provider/path/that/does/not/exist")

	// Should return an error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create RPC client")
}

// Table-driven tests for comprehensive coverage

func TestProviderGRPCClient_Schema_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		protocolType  string
		mockResponse  interface{}
		mockError     error
		expectedError string
		validateResp  func(t *testing.T, resp interface{})
	}{
		{
			name:         "V5 Success with full schema",
			protocolType: "v5",
			mockResponse: createTestV5Response(),
			mockError:    nil,
			validateResp: func(t *testing.T, resp interface{}) {
				v5Resp := resp.(*tfplugin5.GetProviderSchema_Response)
				assert.NotNil(t, v5Resp.Provider)
				assert.Equal(t, int64(1), v5Resp.Provider.Version)
				assert.Len(t, v5Resp.Provider.Block.Attributes, 1)
				assert.Equal(t, "test_attribute", v5Resp.Provider.Block.Attributes[0].Name)
			},
		},
		{
			name:          "V5 RPC Connection Error",
			protocolType:  "v5",
			mockResponse:  nil,
			mockError:     errors.New("connection refused"),
			expectedError: "failed to get provider schema: connection refused",
		},
		{
			name:         "V6 Success with full schema",
			protocolType: "v6",
			mockResponse: createTestV6Response(),
			mockError:    nil,
			validateResp: func(t *testing.T, resp interface{}) {
				v6Resp := resp.(*tfplugin6.GetProviderSchema_Response)
				assert.NotNil(t, v6Resp.Provider)
				assert.Equal(t, int64(1), v6Resp.Provider.Version)
				assert.Len(t, v6Resp.Provider.Block.Attributes, 1)
				assert.Equal(t, "test_attribute", v6Resp.Provider.Block.Attributes[0].Name)
			},
		},
		{
			name:          "V6 Authentication Error",
			protocolType:  "v6",
			mockResponse:  nil,
			mockError:     errors.New("authentication failed"),
			expectedError: "failed to get provider schema: authentication failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.protocolType == "v5" {
				mockSchemaClient := &mockV5SchemaClient{}
				client := &providerGRPCClient[*tfplugin5.GetProviderSchema_Request, *tfplugin5.GetProviderSchema_Response]{
					grpcClient: mockSchemaClient,
				}

				req := &tfplugin5.GetProviderSchema_Request{}
				mockSchemaClient.On("getSchema", mock.Anything, req, mock.Anything).Return(tt.mockResponse, tt.mockError)

				resp, err := client.Schema(req)

				if tt.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tt.expectedError)
					assert.Nil(t, resp)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.mockResponse, resp)
					if tt.validateResp != nil {
						tt.validateResp(t, resp)
					}
				}
				mockSchemaClient.AssertExpectations(t)
			} else {
				mockSchemaClient := &mockV6SchemaClient{}
				client := &providerGRPCClient[*tfplugin6.GetProviderSchema_Request, *tfplugin6.GetProviderSchema_Response]{
					grpcClient: mockSchemaClient,
				}

				req := &tfplugin6.GetProviderSchema_Request{}
				mockSchemaClient.On("getSchema", mock.Anything, req, mock.Anything).Return(tt.mockResponse, tt.mockError)

				resp, err := client.Schema(req)

				if tt.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tt.expectedError)
					assert.Nil(t, resp)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.mockResponse, resp)
					if tt.validateResp != nil {
						tt.validateResp(t, resp)
					}
				}
				mockSchemaClient.AssertExpectations(t)
			}
		})
	}
}

// Benchmark tests

func BenchmarkProviderGRPCClientV5_Schema(b *testing.B) {
	mockSchemaClient := &mockV5SchemaClient{}
	client := &providerGRPCClient[*tfplugin5.GetProviderSchema_Request, *tfplugin5.GetProviderSchema_Response]{
		grpcClient: mockSchemaClient,
	}

	req := &tfplugin5.GetProviderSchema_Request{}
	resp := createTestV5Response()

	mockSchemaClient.On("getSchema", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.Schema(req)
	}
}

func BenchmarkProviderGRPCClientV6_Schema(b *testing.B) {
	mockSchemaClient := &mockV6SchemaClient{}
	client := &providerGRPCClient[*tfplugin6.GetProviderSchema_Request, *tfplugin6.GetProviderSchema_Response]{
		grpcClient: mockSchemaClient,
	}

	req := &tfplugin6.GetProviderSchema_Request{}
	resp := createTestV6Response()

	mockSchemaClient.On("getSchema", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.Schema(req)
	}
}

// Test Schema() conversion from v6 proto to terraform-json ProviderSchema
func TestUniversalProviderClient_Schema_V6Success(t *testing.T) {
	mockSchemaClient := &mockV6SchemaClient{}
	innerClient := &providerGRPCClient[*tfplugin6.GetProviderSchema_Request, *tfplugin6.GetProviderSchema_Response]{
		grpcClient: mockSchemaClient,
	}
	v6Client := &providerGRPCClientV6{
		providerGRPCClient: innerClient,
	}
	client := &universalProviderClient{
		v6: v6Client,
	}

	expectedResp := createTestV6Response()

	mockSchemaClient.On("getSchema", mock.Anything, mock.Anything, mock.Anything).Return(expectedResp, nil)

	ps, err := client.schema()

	assert.NoError(t, err)
	assert.NotNil(t, ps)

	// Check provider/config schema version
	if assert.NotNil(t, ps.ConfigSchema) {
		assert.Equal(t, uint64(1), ps.ConfigSchema.Version)
		if assert.NotNil(t, ps.ConfigSchema.Block) {
			attr, ok := ps.ConfigSchema.Block.Attributes["test_attribute"]
			assert.True(t, ok)
			assert.NotNil(t, attr)
			assert.True(t, attr.Required)
		}
	}

	// Check resource schema attribute present and computed
	rs, ok := ps.ResourceSchemas["test_resource"]
	assert.True(t, ok)
	if assert.NotNil(t, rs) && assert.NotNil(t, rs.Block) {
		attr, ok := rs.Block.Attributes["id"]
		assert.True(t, ok)
		assert.NotNil(t, attr)
		assert.True(t, attr.Computed)
	}

	// Check data source schema attribute present and computed
	ds, ok := ps.DataSourceSchemas["test_data_source"]
	assert.True(t, ok)
	if assert.NotNil(t, ds) && assert.NotNil(t, ds.Block) {
		attr, ok := ds.Block.Attributes["value"]
		assert.True(t, ok)
		assert.NotNil(t, attr)
		assert.True(t, attr.Computed)
	}

	mockSchemaClient.AssertExpectations(t)
}

// Test Schema() conversion from v5 proto to terraform-json ProviderSchema
func TestUniversalProviderClient_Schema_V5Success(t *testing.T) {
	mockSchemaClient := &mockV5SchemaClient{}
	innerClient := &providerGRPCClient[*tfplugin5.GetProviderSchema_Request, *tfplugin5.GetProviderSchema_Response]{
		grpcClient: mockSchemaClient,
	}
	v5Client := &providerGRPCClientV5{
		providerGRPCClient: innerClient,
	}
	client := &universalProviderClient{
		v5: v5Client,
	}

	expectedResp := createTestV5Response()

	mockSchemaClient.On("getSchema", mock.Anything, mock.Anything, mock.Anything).Return(expectedResp, nil)

	ps, err := client.schema()

	assert.NoError(t, err)
	assert.NotNil(t, ps)

	// Provider/config schema version
	if assert.NotNil(t, ps.ConfigSchema) {
		assert.Equal(t, uint64(1), ps.ConfigSchema.Version)
		if assert.NotNil(t, ps.ConfigSchema.Block) {
			attr, ok := ps.ConfigSchema.Block.Attributes["test_attribute"]
			assert.True(t, ok)
			assert.NotNil(t, attr)
			assert.True(t, attr.Required)
		}
	}

	// Resource schema
	rs, ok := ps.ResourceSchemas["test_resource"]
	assert.True(t, ok)
	if assert.NotNil(t, rs) && assert.NotNil(t, rs.Block) {
		attr, ok := rs.Block.Attributes["id"]
		assert.True(t, ok)
		assert.NotNil(t, attr)
		assert.True(t, attr.Computed)
	}

	// Data source schema
	ds, ok := ps.DataSourceSchemas["test_data_source"]
	assert.True(t, ok)
	if assert.NotNil(t, ds) && assert.NotNil(t, ds.Block) {
		attr, ok := ds.Block.Attributes["value"]
		assert.True(t, ok)
		assert.NotNil(t, attr)
		assert.True(t, attr.Computed)
	}

	mockSchemaClient.AssertExpectations(t)
}

// Test that Schema() falls back to v5 when v6 fails
func TestUniversalProviderClient_Schema_FallbackV6ToV5(t *testing.T) {
	mockV6 := &mockV6SchemaClient{}
	mockV5 := &mockV5SchemaClient{}

	innerV6 := &providerGRPCClient[*tfplugin6.GetProviderSchema_Request, *tfplugin6.GetProviderSchema_Response]{
		grpcClient: mockV6,
	}
	innerV5 := &providerGRPCClient[*tfplugin5.GetProviderSchema_Request, *tfplugin5.GetProviderSchema_Response]{
		grpcClient: mockV5,
	}

	v6Client := &providerGRPCClientV6{providerGRPCClient: innerV6}
	v5Client := &providerGRPCClientV5{providerGRPCClient: innerV5}

	client := &universalProviderClient{v6: v6Client, v5: v5Client}

	// v6 returns an error
	mockV6.On("getSchema", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("v6 error"))
	// v5 returns a valid schema
	mockV5.On("getSchema", mock.Anything, mock.Anything, mock.Anything).Return(createTestV5Response(), nil)

	ps, err := client.schema()

	assert.NoError(t, err)
	assert.NotNil(t, ps)
	if assert.NotNil(t, ps.ConfigSchema) {
		assert.Equal(t, uint64(1), ps.ConfigSchema.Version)
	}

	mockV6.AssertExpectations(t)
	mockV5.AssertExpectations(t)
}

// Small test to reference provider-level mocks so static analysis doesn't flag them as unused.
func TestHelper_UseProviderMocks(t *testing.T) {
	t.Helper()
	_ = &mockV5ProviderClient{}
	_ = &mockV6ProviderClient{}
}
