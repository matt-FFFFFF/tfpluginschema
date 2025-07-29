package tfpluginschema

import (
	"encoding/json"
	"testing"
)

func TestSchemaResponseMarshalJSON(t *testing.T) {
	// Create a test schema response with base64 encoded type fields
	schema := schemaResponse{
		ResourceSchemas: map[string]any{
			"azapi_resource": map[string]any{
				"block": map[string]any{
					"attributes": []any{
						map[string]any{
							"name": "body",
							"type": "ImR5bmFtaWMi", // base64 for "dynamic"
						},
						map[string]any{
							"name": "id",
							"type": "InN0cmluZyI=", // base64 for "string"
						},
						map[string]any{
							"name": "ignore_casing",
							"type": "ImJvb2wi", // base64 for "bool"
						},
					},
				},
			},
		},
	}

	// Marshal the schema
	result, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal schema: %v", err)
	}

	// Verify that type fields are decoded
	var decoded map[string]any
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Navigate to the attributes and check if type fields are decoded
	resourceSchemas := decoded["resource_schemas"].(map[string]any)
	azapiResource := resourceSchemas["azapi_resource"].(map[string]any)
	block := azapiResource["block"].(map[string]any)
	attributes := block["attributes"].([]any)

	expectedTypes := []string{"dynamic", "string", "bool"}
	for i, attr := range attributes {
		attrMap := attr.(map[string]any)
		typeValue := attrMap["type"].(string)
		if typeValue != expectedTypes[i] {
			t.Errorf("Expected type %q, got %q for attribute %d", expectedTypes[i], typeValue, i)
		}
	}

	t.Logf("Successfully decoded schema:\n%s", result)
}

func TestSchemaResponseMarshalJSONComplex(t *testing.T) {
	// Test with more complex base64 encoded type values from the actual example
	schema := schemaResponse{
		ResourceSchemas: map[string]any{
			"azapi_resource": map[string]any{
				"block": map[string]any{
					"attributes": []any{
						map[string]any{
							"name": "create_headers",
							"type": "WyJtYXAiLCJzdHJpbmciXQ==", // base64 for ["map","string"]
						},
						map[string]any{
							"name": "create_query_parameters",
							"type": "WyJtYXAiLFsibGlzdCIsInN0cmluZyJdXQ==", // base64 for ["map",["list","string"]]
						},
						map[string]any{
							"name": "locks",
							"type": "WyJsaXN0Iiwic3RyaW5nIl0=", // base64 for ["list","string"]
						},
					},
				},
			},
		},
	}

	// Marshal the schema
	result, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal schema: %v", err)
	}

	// Verify that complex type fields are decoded correctly
	var decoded map[string]any
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Navigate to the attributes and check if type fields are decoded
	resourceSchemas := decoded["resource_schemas"].(map[string]any)
	azapiResource := resourceSchemas["azapi_resource"].(map[string]any)
	block := azapiResource["block"].(map[string]any)
	attributes := block["attributes"].([]any)

	// Test first attribute - should be ["map","string"]
	attr1 := attributes[0].(map[string]any)
	typeValue1 := attr1["type"].([]any)
	if len(typeValue1) != 2 || typeValue1[0] != "map" || typeValue1[1] != "string" {
		t.Errorf("Expected [\"map\",\"string\"], got %v", typeValue1)
	}

	// Test second attribute - should be ["map",["list","string"]]
	attr2 := attributes[1].(map[string]any)
	typeValue2 := attr2["type"].([]any)
	if len(typeValue2) != 2 || typeValue2[0] != "map" {
		t.Errorf("Expected first element to be \"map\", got %v", typeValue2)
	}
	if nestedType, ok := typeValue2[1].([]any); !ok || len(nestedType) != 2 || nestedType[0] != "list" || nestedType[1] != "string" {
		t.Errorf("Expected second element to be [\"list\",\"string\"], got %v", typeValue2[1])
	}

	// Test third attribute - should be ["list","string"]
	attr3 := attributes[2].(map[string]any)
	typeValue3 := attr3["type"].([]any)
	if len(typeValue3) != 2 || typeValue3[0] != "list" || typeValue3[1] != "string" {
		t.Errorf("Expected [\"list\",\"string\"], got %v", typeValue3)
	}

	t.Logf("Successfully decoded complex schema:\n%s", result)
}
