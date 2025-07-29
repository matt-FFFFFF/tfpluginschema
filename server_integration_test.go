package tfpluginschema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	schemaJSON, err := s.getSchema(request)
	require.NoError(t, err)
	require.NotNil(t, schemaJSON)

	// Parse the JSON to verify structure
	var schemaData map[string]interface{}
	err = json.Unmarshal(schemaJSON, &schemaData)
	require.NoError(t, err)

	// Check that we got actual schema data
	resourceSchemas, hasResources := schemaData["resource_schemas"].(map[string]interface{})
	dataSourceSchemas, hasDataSources := schemaData["data_source_schemas"].(map[string]interface{})

	require.True(t, hasResources, "Should have resource_schemas field")
	require.True(t, hasDataSources, "Should have data_source_schemas field")
	require.Greater(t, len(resourceSchemas), 0, "Should have resource schemas")
	require.Greater(t, len(dataSourceSchemas), 0, "Should have data source schemas")

	t.Logf("JSON schema size: %d bytes", len(schemaJSON))
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

	azapiResource, err := s.GetResourceSchema(request, "azapi_resource")
	require.NoError(t, err)
	require.NotNil(t, azapiResource)
	t.Logf("azapi_resource schema: %s", string(azapiResource))
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
	schemaJSON, err := s.getSchema(request)
	require.NoError(t, err)
	require.NotNil(t, schemaJSON)

	// Parse the JSON to verify structure
	var schemaData map[string]interface{}
	err = json.Unmarshal(schemaJSON, &schemaData)
	require.NoError(t, err)

	// Check that we got actual schema data
	resourceSchemas, hasResources := schemaData["resource_schemas"].(map[string]interface{})
	dataSourceSchemas, hasDataSources := schemaData["data_source_schemas"].(map[string]interface{})

	require.True(t, hasResources, "Should have resource_schemas field")
	require.True(t, hasDataSources, "Should have data_source_schemas field")
	require.Greater(t, len(resourceSchemas), 0, "Should have resource schemas")
	require.Greater(t, len(dataSourceSchemas), 0, "Should have data source schemas")

	t.Logf("JSON schema size: %d bytes", len(schemaJSON))
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
