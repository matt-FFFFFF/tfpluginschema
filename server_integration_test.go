package tfpluginschema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_GetAvailableVersions(t *testing.T) {
	s := NewServer(nil)
	defer s.Cleanup()

	req := VersionsRequest{
		Namespace: "hashicorp",
		Name:      "aws",
	}

	versions, err := s.GetAvailableVersions(req)
	require.NoError(t, err)
	require.NotNil(t, versions)
	require.Greater(t, len(versions), 0, "Should have at least one version")
	t.Log("Available versions:", versions)
}

func TestServer_AzAPI(t *testing.T) {
	s := NewServer(nil)
	defer s.Cleanup()

	request := Request{
		Namespace: "Azure",
		Name:      "azapi",
		Version:   "2.5.0",
	}

	// Download and extract the provider
	require.NoError(t, s.Get(request))
	assert.Len(t, s.dlc, 1)

	// Get schema
	schema, err := s.getSchema(request)
	require.NoError(t, err)
	require.NotNil(t, schema)

	// Check that we got actual schema data
	var resourceSchemas = schema.ResourceSchemas
	var dataSourceSchemas = schema.DataSourceSchemas
	var ephemeralResourceSchemas = schema.EphemeralResourceSchemas
	providerSchema := schema.ConfigSchema
	require.NotNil(t, providerSchema, "Should have provider schema")

	require.NotNil(t, resourceSchemas, "Should have resource_schemas field")
	require.NotNil(t, dataSourceSchemas, "Should have data_source_schemas field")
	require.NotNil(t, ephemeralResourceSchemas, "Should have ephemeral_resource_schemas field")
	require.Greater(t, len(resourceSchemas), 0, "Should have resource schemas")
	require.Greater(t, len(dataSourceSchemas), 0, "Should have data source schemas")
	require.Greater(t, len(ephemeralResourceSchemas), 0, "Should have ephemeral resource schemas")

	t.Logf("Number of resource schemas: %d", len(resourceSchemas))
	t.Logf("Number of data source schemas: %d", len(dataSourceSchemas))
	t.Logf("Number of ephemeral resource schemas: %d", len(ephemeralResourceSchemas))

	// Log some resource names for verification
	for name := range resourceSchemas {
		t.Logf("Resource: %s", name)
	}

	// Log some data source names for verification
	for name := range dataSourceSchemas {
		t.Logf("Data source: %s", name)
	}

	// Log some ephemeral resource names for verification
	for name := range ephemeralResourceSchemas {
		t.Logf("Ephemeral resource: %s", name)
	}

	azapiResource, err := s.GetResourceSchema(request, "azapi_resource")
	require.NoError(t, err)
	require.NotNil(t, azapiResource)
	azapiResourceJson, err := json.Marshal(azapiResource)
	require.NoError(t, err)
	t.Logf("azapi_resource schema: %s", string(azapiResourceJson))

	providerSchema, err = s.GetProviderSchema(request)
	require.NoError(t, err)
	require.NotNil(t, providerSchema)

	providerSchemaJson, err := json.Marshal(providerSchema)
	require.NoError(t, err)

	t.Logf("Provider schema: %s", string(providerSchemaJson))
}

func TestServer_AzureRM(t *testing.T) {
	s := NewServer(nil)
	defer s.Cleanup()

	request := Request{
		Namespace: "hashicorp",
		Name:      "azurerm",
		Version:   "4.37.0",
	}

	// Download and extract the provider
	require.NoError(t, s.Get(request))
	assert.Len(t, s.dlc, 1)

	// Get schema
	schema, err := s.getSchema(request)
	require.NoError(t, err)
	require.NotNil(t, schema)

	// Check that we got actual schema data
	resourceSchemas := schema.ResourceSchemas
	dataSourceSchemas := schema.DataSourceSchemas

	require.NotNil(t, resourceSchemas, "Should have resource_schemas field")
	require.NotNil(t, dataSourceSchemas, "Should have data_source_schemas field")
	require.Greater(t, len(resourceSchemas), 0, "Should have resource schemas")
	require.Greater(t, len(dataSourceSchemas), 0, "Should have data source schemas")

	t.Logf("Number of resource schemas: %d", len(resourceSchemas))
	t.Logf("Number of data source schemas: %d", len(dataSourceSchemas))

	// Log some resource names for verification
	for name := range resourceSchemas {
		t.Logf("Resource: %s", name)
	}

	// Log some data source names for verification
	for name := range dataSourceSchemas {
		t.Logf("Data source: %s", name)
	}
}
