package tfpluginschema

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRequest_String_URLFormat tests that the URL format is correct for both registries
func TestRequest_String(t *testing.T) {
	tests := []struct {
		name            string
		request         Request
		expectedStrings []string
	}{
		{
			name: "OpenTofu default",
			request: Request{
				Namespace: "Azure",
				Name:      "azapi",
				Version:   "2.7.0",
			},
			expectedStrings: []string{"registry.opentofu.org", "/Azure/azapi/2.7.0/"},
		},
		{
			name: "Terraform explicit",
			request: Request{
				Namespace:    "Azure",
				Name:         "azapi",
				Version:      "2.7.0",
				RegistryType: RegistryTypeTerraform,
			},
			expectedStrings: []string{"registry.terraform.io", "/Azure/azapi/2.7.0/"},
		},
		{
			name: "OpenTofu explicit",
			request: Request{
				Namespace:    "Azure",
				Name:         "azapi",
				Version:      "2.7.0",
				RegistryType: RegistryTypeOpenTofu,
			},
			expectedStrings: []string{"registry.opentofu.org", "/Azure/azapi/2.7.0/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.request.String()

			// Check that URL starts with https://
			assert.True(t, strings.HasPrefix(url, "https://"), "URL should start with https://")

			for _, expected := range tt.expectedStrings {
				// Check that URL contains expected host
				assert.Contains(t, url, expected, "URL should contain expected registry host")
			}

			// Check that URL contains /v1/providers/
			assert.Contains(t, url, "/v1/providers/", "URL should contain /v1/providers/")

			// Check that URL contains the path components
			expectedPath := "/" + tt.request.Namespace + "/" + tt.request.Name + "/" + tt.request.Version + "/download/"
			assert.Contains(t, url, expectedPath, "URL should contain correct path")
		})
	}
}

// TestRegistryType_BaseURL tests the BaseURL method for different registry types
func TestRegistryType_BaseURL(t *testing.T) {
	tests := []struct {
		name         string
		registryType RegistryType
		expectedURL  string
	}{
		{
			name:         "OpenTofu registry",
			registryType: RegistryTypeOpenTofu,
			expectedURL:  "https://registry.opentofu.org/v1/providers",
		},
		{
			name:         "Terraform registry",
			registryType: RegistryTypeTerraform,
			expectedURL:  "https://registry.terraform.io/v1/providers",
		},
		{
			name:         "Empty/default registry type",
			registryType: "",
			expectedURL:  "https://registry.opentofu.org/v1/providers",
		},
		{
			name:         "Unknown registry type defaults to OpenTofu",
			registryType: "unknown",
			expectedURL:  "https://registry.opentofu.org/v1/providers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.registryType.BaseURL()
			assert.Equal(t, tt.expectedURL, url, "BaseURL should return expected URL")
		})
	}
}

// TestBackwardCompatibility_RequestWithoutRegistryType ensures existing code continues to work
func TestBackwardCompatibility_RequestWithoutRegistryType(t *testing.T) {
	// This simulates existing code that doesn't set RegistryType
	req := Request{
		Namespace: "hashicorp",
		Name:      "random",
		Version:   "3.6.0",
		// RegistryType is not set (zero value)
	}

	url := req.String()

	// Should default to OpenTofu registry
	assert.Contains(t, url, "registry.opentofu.org", "Backward compatibility: should default to OpenTofu")
	assert.Contains(t, url, "https://registry.opentofu.org/v1/providers/hashicorp/random/3.6.0/download/")
}

// TestBackwardCompatibility_VersionsRequestWithoutRegistryType ensures existing code continues to work
func TestBackwardCompatibility_VersionsRequestWithoutRegistryType(t *testing.T) {
	// This simulates existing code that doesn't set RegistryType
	req := VersionsRequest{
		Namespace: "hashicorp",
		Name:      "aws",
		// RegistryType is not set (zero value)
	}

	url := req.String()

	// Should default to OpenTofu registry
	assert.Contains(t, url, "registry.opentofu.org", "Backward compatibility: should default to OpenTofu")
	assert.Contains(t, url, "https://registry.opentofu.org/v1/providers/hashicorp/aws/versions")
}

// TestRegistryTypeConstants verifies that the constants are defined correctly
func TestRegistryTypeConstants(t *testing.T) {
	assert.Equal(t, RegistryType("opentofu"), RegistryTypeOpenTofu, "OpenTofu constant should be 'opentofu'")
	assert.Equal(t, RegistryType("terraform"), RegistryTypeTerraform, "Terraform constant should be 'terraform'")
	assert.NotEqual(t, RegistryTypeOpenTofu, RegistryTypeTerraform, "Constants should be different")
}
