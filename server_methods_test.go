package tfpluginschema

import (
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDataSourceSchema_Success(t *testing.T) {
	s := NewServer(nil)
	t.Cleanup(s.Cleanup)
	req := Request{Namespace: "n", Name: "p", Version: "v"}
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
	req := Request{Namespace: "n", Name: "p", Version: "v"}
	s.sc[req] = &tfjson.ProviderSchema{DataSourceSchemas: map[string]*tfjson.Schema{}}
	got, err := s.GetDataSourceSchema(req, "missing")
	assert.Nil(t, got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "data source schema not found")
}

func TestGetFunctionSchema_Success(t *testing.T) {
	s := NewServer(nil)
	t.Cleanup(s.Cleanup)
	req := Request{Namespace: "n", Name: "p", Version: "v"}
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
	req := Request{Namespace: "n", Name: "p", Version: "v"}
	s.sc[req] = &tfjson.ProviderSchema{Functions: map[string]*tfjson.FunctionSignature{}}
	got, err := s.GetFunctionSchema(req, "missing")
	assert.Nil(t, got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "function schema not found")
}

func TestGetEphemeralResourceSchema_Success(t *testing.T) {
	s := NewServer(nil)
	t.Cleanup(s.Cleanup)
	req := Request{Namespace: "n", Name: "p", Version: "v"}
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
	req := Request{Namespace: "n", Name: "p", Version: "v"}
	s.sc[req] = &tfjson.ProviderSchema{EphemeralResourceSchemas: map[string]*tfjson.Schema{}}
	got, err := s.GetEphemeralResourceSchema(req, "missing")
	assert.Nil(t, got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ephemeral resource schema not found")
}

func TestGetResourceSchema_NotFound(t *testing.T) {
	s := NewServer(nil)
	req := Request{Namespace: "n", Name: "p", Version: "v"}
	s.sc[req] = &tfjson.ProviderSchema{ResourceSchemas: map[string]*tfjson.Schema{}}
	got, err := s.GetResourceSchema(req, "missing")
	assert.Nil(t, got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resource schema not found")
}
