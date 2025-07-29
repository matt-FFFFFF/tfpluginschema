package tfpluginschema

import (
	"encoding/base64"
	"encoding/json"
)

type schemaResponse struct {
	Provider                 map[string]any `json:"provider"`
	ResourceSchemas          map[string]any `json:"resource_schemas,omitempty"`
	EphemeralResourceSchemas map[string]any `json:"ephemeral_resource_schemas,omitempty"`
	DataSourceSchemas        map[string]any `json:"data_source_schemas,omitempty"`
	Functions                map[string]any `json:"functions,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for schemaResponse.
// It recursively decodes base64 encoded values for any field named "type".
func (s schemaResponse) MarshalJSON() ([]byte, error) {
	// Create a copy of the struct with decoded type fields
	decoded := map[string]any{
		"provider":                   decodeTypeFields(s.Provider),
		"resource_schemas":           decodeTypeFields(s.ResourceSchemas),
		"ephemeral_resource_schemas": decodeTypeFields(s.EphemeralResourceSchemas),
		"data_source_schemas":        decodeTypeFields(s.DataSourceSchemas),
		"functions":                  decodeTypeFields(s.Functions),
	}

	return json.Marshal(decoded)
}

// decodeTypeFields recursively traverses a data structure and decodes base64 values
// for any field named "type".
func decodeTypeFields(data any) any {
	switch v := data.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, value := range v {
			if key == "type" {
				result[key] = decodeTypeField(value)
				continue
			}
			// Recursively process other fields
			result[key] = decodeTypeFields(value)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = decodeTypeFields(item)
		}
		return result
	default:
		// For primitive types, return as-is
		return v
	}
}

// decodeTypeField handles the decoding of a single type field value.
// It attempts to base64 decode the value if it's a string, then JSON decode the result.
func decodeTypeField(value any) any {
	strValue, ok := value.(string)
	if !ok {
		// Not a string, return original value
		return value
	}

	decoded, err := base64.StdEncoding.DecodeString(strValue)
	if err != nil {
		// Failed to decode base64, return original value
		return value
	}

	var jsonValue any
	if err := json.Unmarshal(decoded, &jsonValue); err != nil {
		// Failed to JSON decode, return the raw decoded string
		return string(decoded)
	}

	// Successfully decoded both base64 and JSON
	return jsonValue
}
