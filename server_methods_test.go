package tfpluginschema

import (
	"testing"

	goversion "github.com/hashicorp/go-version"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDataSourceSchema_Success(t *testing.T) {
	s := NewServer(nil)
	t.Cleanup(s.Cleanup)
	req := Request{Namespace: "n", Name: "p", Version: "1.2.3"}
	s.sc[req] = &tfjson.ProviderSchema{
		DataSourceSchemas: map[string]*tfjson.Schema{
			"ds": {Block: &tfjson.SchemaBlock{}},
		},
	}
	got, err := s.GetDataSourceSchema(req, "ds")
	require.NoError(t, err)
	assert.NotNil(t, got)
}

func TestGetDataSourceSchema_NotFound(t *testing.T) {
	s := NewServer(nil)
	req := Request{Namespace: "n", Name: "p", Version: "1.2.3"}
	s.sc[req] = &tfjson.ProviderSchema{DataSourceSchemas: map[string]*tfjson.Schema{}}
	got, err := s.GetDataSourceSchema(req, "missing")
	assert.Nil(t, got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "data source schema not found")
}

func TestGetFunctionSchema_Success(t *testing.T) {
	s := NewServer(nil)
	t.Cleanup(s.Cleanup)
	req := Request{Namespace: "n", Name: "p", Version: "1.2.3"}
	s.sc[req] = &tfjson.ProviderSchema{
		Functions: map[string]*tfjson.FunctionSignature{
			"fn": {Summary: "ok"},
		},
	}
	got, err := s.GetFunctionSchema(req, "fn")
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "ok", got.Summary)
}

func TestGetFunctionSchema_NotFound(t *testing.T) {
	s := NewServer(nil)
	req := Request{Namespace: "n", Name: "p", Version: "1.2.3"}
	s.sc[req] = &tfjson.ProviderSchema{Functions: map[string]*tfjson.FunctionSignature{}}
	got, err := s.GetFunctionSchema(req, "missing")
	assert.Nil(t, got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "function schema not found")
}

func TestGetEphemeralResourceSchema_Success(t *testing.T) {
	s := NewServer(nil)
	t.Cleanup(s.Cleanup)
	req := Request{Namespace: "n", Name: "p", Version: "1.2.3"}
	s.sc[req] = &tfjson.ProviderSchema{
		EphemeralResourceSchemas: map[string]*tfjson.Schema{
			"er": {Block: &tfjson.SchemaBlock{}},
		},
	}
	got, err := s.GetEphemeralResourceSchema(req, "er")
	require.NoError(t, err)
	assert.NotNil(t, got)
}

func TestGetEphemeralResourceSchema_NotFound(t *testing.T) {
	s := NewServer(nil)
	req := Request{Namespace: "n", Name: "p", Version: "1.2.3"}
	s.sc[req] = &tfjson.ProviderSchema{EphemeralResourceSchemas: map[string]*tfjson.Schema{}}
	got, err := s.GetEphemeralResourceSchema(req, "missing")
	assert.Nil(t, got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ephemeral resource schema not found")
}

func TestGetResourceSchema_NotFound(t *testing.T) {
	s := NewServer(nil)
	req := Request{Namespace: "n", Name: "p", Version: "1.2.3"}
	s.sc[req] = &tfjson.ProviderSchema{ResourceSchemas: map[string]*tfjson.Schema{}}
	got, err := s.GetResourceSchema(req, "missing")
	assert.Nil(t, got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource schema not found")
}

func TestRequest_fixedVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected bool
	}{
		// Valid semantic versions
		{
			name:     "valid semantic version 1.0.0",
			version:  "1.0.0",
			expected: true,
		},
		{
			name:     "valid semantic version 2.5.1",
			version:  "2.5.1",
			expected: true,
		},
		{
			name:     "valid semantic version with prerelease",
			version:  "1.0.0-alpha",
			expected: true,
		},
		{
			name:     "valid semantic version with prerelease and build metadata",
			version:  "1.0.0-alpha.1+build.123",
			expected: true,
		},
		{
			name:     "valid semantic version with build metadata",
			version:  "1.0.0+20230101",
			expected: true,
		},
		{
			name:     "valid version with v prefix",
			version:  "v1.0.0",
			expected: true,
		},
		{
			name:     "valid simple version",
			version:  "1.0",
			expected: true,
		},
		{
			name:     "valid single digit version",
			version:  "1",
			expected: true,
		},
		{
			name:     "valid version with leading zeros",
			version:  "01.02.03",
			expected: true,
		},
		// Invalid versions
		{
			name:     "empty version",
			version:  "",
			expected: false,
		},
		{
			name:     "invalid version with text",
			version:  "invalid",
			expected: false,
		},
		{
			name:     "version with only dots",
			version:  "...",
			expected: false,
		},
		{
			name:     "version with special characters",
			version:  "1.0.0@#$",
			expected: false,
		},
		{
			name:     "version with spaces",
			version:  "1.0.0 beta",
			expected: false,
		},
		{
			name:     "version starting with dot",
			version:  ".1.0.0",
			expected: false,
		},
		{
			name:     "version ending with dot",
			version:  "1.0.0.",
			expected: false,
		},
		{
			name:     "negative version numbers",
			version:  "-1.0.0",
			expected: false,
		},
		{
			name:     "version with multiple consecutive dots",
			version:  "1..0.0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := Request{
				Namespace: "test",
				Name:      "provider",
				Version:   tt.version,
			}
			result := req.fixedVersion()
			assert.Equal(t, tt.expected, result, "fixedVersion() for version %q should return %v", tt.version, tt.expected)
		})
	}
}

func TestRequest_fixVersion(t *testing.T) {
	// Helper function to create version collection from string slice
	mustVersions := func(versions []string) goversion.Collection {
		var collection goversion.Collection
		for _, v := range versions {
			version, err := goversion.NewVersion(v)
			if err != nil {
				t.Fatalf("failed to create version %s: %v", v, err)
			}
			collection = append(collection, version)
		}
		return collection
	}

	tests := []struct {
		name           string
		request        Request
		setupServer    func(*Server)
		expectedResult Request
		expectedError  string
	}{
		{
			name: "fixed version stays unchanged",
			request: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "1.2.3",
			},
			setupServer: func(s *Server) {
				// No setup needed for fixed version
			},
			expectedResult: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "1.2.3",
			},
			expectedError: "",
		},
		{
			name: "empty version gets resolved to latest",
			request: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "",
			},
			setupServer: func(s *Server) {
				// Mock the versions response
				s.versionsc[VersionsRequest{Namespace: "hashicorp", Name: "aws"}] = mustVersions([]string{"1.0.0", "1.1.0", "2.0.0"})
			},
			expectedResult: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "2.0.0",
			},
			expectedError: "",
		},
		{
			name: "constraint version gets resolved to latest matching",
			request: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   ">=1.0.0, <2.0.0",
			},
			setupServer: func(s *Server) {
				// Mock the versions response
				s.versionsc[VersionsRequest{Namespace: "hashicorp", Name: "aws"}] = mustVersions([]string{"0.9.0", "1.0.0", "1.5.0", "2.0.0", "2.1.0"})
			},
			expectedResult: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "1.5.0",
			},
			expectedError: "",
		},
		{
			name: "tilde constraint gets resolved correctly",
			request: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "~>1.1",
			},
			setupServer: func(s *Server) {
				// Mock the versions response
				s.versionsc[VersionsRequest{Namespace: "hashicorp", Name: "aws"}] = mustVersions([]string{"1.0.0", "1.1.0", "1.1.5", "1.2.0", "2.0.0"})
			},
			expectedResult: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "1.2.0", // ~>1.1 allows 1.2.0 as the latest match
			},
			expectedError: "",
		},
		{
			name: "invalid constraint falls back to latest version",
			request: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "invalid-constraint",
			},
			setupServer: func(s *Server) {
				// Mock the versions response
				s.versionsc[VersionsRequest{Namespace: "hashicorp", Name: "aws"}] = mustVersions([]string{"1.0.0", "1.1.0"})
			},
			expectedResult: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "1.1.0", // Falls back to latest when constraint is invalid
			},
			expectedError: "",
		},
		{
			name: "restrictive tilde constraint",
			request: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "~>1.1.0",
			},
			setupServer: func(s *Server) {
				// Mock the versions response
				s.versionsc[VersionsRequest{Namespace: "hashicorp", Name: "aws"}] = mustVersions([]string{"1.0.0", "1.1.0", "1.1.5", "1.2.0", "2.0.0"})
			},
			expectedResult: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "1.1.5", // ~>1.1.0 allows patch updates but not minor
			},
			expectedError: "",
		},
		{
			name: "constraint matching no versions returns error",
			request: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   ">=5.0.0",
			},
			setupServer: func(s *Server) {
				// Mock the versions response with no matching versions
				s.versionsc[VersionsRequest{Namespace: "hashicorp", Name: "aws"}] = mustVersions([]string{"1.0.0", "2.0.0", "3.0.0"})
			},
			expectedResult: Request{},
			expectedError:  "failed to get latest version",
		},
		{
			name: "no available versions returns error",
			request: Request{
				Namespace: "nonexistent",
				Name:      "provider",
				Version:   "",
			},
			setupServer: func(s *Server) {
				// Empty versions map - no versions available for this provider
			},
			expectedResult: Request{},
			expectedError:  "failed to get latest version",
		},
		{
			name: "valid version with v prefix stays unchanged",
			request: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "v1.2.3",
			},
			setupServer: func(s *Server) {
				// No setup needed for fixed version
			},
			expectedResult: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "v1.2.3",
			},
			expectedError: "",
		},
		{
			name: "prerelease version stays unchanged",
			request: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "1.2.3-beta.1",
			},
			setupServer: func(s *Server) {
				// No setup needed for fixed version
			},
			expectedResult: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "1.2.3-beta.1",
			},
			expectedError: "",
		},
		{
			name: "exact constraint matches specific version",
			request: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "= 1.1.0",
			},
			setupServer: func(s *Server) {
				// Mock the versions response
				s.versionsc[VersionsRequest{Namespace: "hashicorp", Name: "aws"}] = mustVersions([]string{"1.0.0", "1.1.0", "1.2.0"})
			},
			expectedResult: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "1.1.0",
			},
			expectedError: "",
		},
		{
			name: "complex constraint with multiple conditions",
			request: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   ">= 1.0.0, < 2.0.0, != 1.3.0",
			},
			setupServer: func(s *Server) {
				// Mock the versions response
				s.versionsc[VersionsRequest{Namespace: "hashicorp", Name: "aws"}] = mustVersions([]string{"0.9.0", "1.0.0", "1.2.0", "1.3.0", "1.4.0", "2.0.0"})
			},
			expectedResult: Request{
				Namespace: "hashicorp",
				Name:      "aws",
				Version:   "1.4.0", // Should get 1.4.0 since 1.3.0 is excluded
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer(nil)
			defer s.Cleanup()

			// Setup server state for test
			tt.setupServer(s)

			result, err := tt.request.fixVersion(s)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				// For error cases, we expect zero value Request
				assert.Equal(t, Request{}, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}
