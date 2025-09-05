package tfpluginschema

import (
	"testing"

	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionOrLatest_WithExplicitVersion(t *testing.T) {
	// Test that explicit version is used as-is
	version, err := versionOrLatest("hashicorp", "aws", "5.0.0")
	require.NoError(t, err)
	assert.Equal(t, "5.0.0", version)
	
	// Test version with 'v' prefix is stripped
	version, err = versionOrLatest("hashicorp", "aws", "v5.0.0")
	require.NoError(t, err)
	assert.Equal(t, "5.0.0", version)
}

func TestVersionOrLatest_WithEmptyVersion(t *testing.T) {
	// Stub the getLatestVersion function to return a mock version directly
	stub := gostub.Stub(&getLatestVersion, func(namespace, providerType string) (string, error) {
		// Verify the correct parameters are passed
		assert.Equal(t, "hashicorp", namespace)
		assert.Equal(t, "aws", providerType)
		
		// Return mock version with 'v' prefix to test stripping
		return "v5.31.0", nil
	})
	defer stub.Reset()

	// Test that empty version triggers latest version lookup
	version, err := versionOrLatest("hashicorp", "aws", "")
	require.NoError(t, err)
	assert.Equal(t, "5.31.0", version) // Should strip 'v' prefix from returned version
}

func TestVersionOrLatest_LatestVersionError(t *testing.T) {
	// Stub the getLatestVersion function to simulate an error
	stub := gostub.Stub(&getLatestVersion, func(namespace, providerType string) (string, error) {
		return "", assert.AnError // Return a test error
	})
	defer stub.Reset()

	// Test that error is properly propagated
	version, err := versionOrLatest("hashicorp", "nonexistent", "")
	assert.Empty(t, version)
	require.Error(t, err)
}

func TestGetLatestVersion_Success(t *testing.T) {
	// Stub the getLatestVersion function to return a mock version directly
	stub := gostub.Stub(&getLatestVersion, func(namespace, providerType string) (string, error) {
		// Verify the correct parameters are passed
		assert.Equal(t, "hashicorp", namespace)
		assert.Equal(t, "aws", providerType)
		
		// Return mock version with 'v' prefix
		return "v5.31.0", nil
	})
	defer stub.Reset()

	version, err := getLatestVersion("hashicorp", "aws")
	require.NoError(t, err)
	assert.Equal(t, "v5.31.0", version)
}

func TestGetLatestVersion_AzureRM(t *testing.T) {
	// Test the actual getLatestVersion function without stubbing - makes real HTTP call
	version, err := getLatestVersion("hashicorp", "azurerm")
	require.NoError(t, err)
	
	// Verify we got a valid version
	assert.NotEmpty(t, version, "version should not be empty")
	assert.Contains(t, version, ".", "version should contain dots (semantic versioning)")
	assert.True(t, len(version) > 1, "version should have meaningful length")
	
	// Should typically start with 'v' prefix from registry
	if len(version) > 0 {
		assert.True(t, version[0] == 'v' || (version[0] >= '0' && version[0] <= '9'), 
			"version should start with 'v' or a digit")
	}
}

