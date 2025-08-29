package tfpluginschema

import (
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/matt-FFFFFF/tfpluginschema/tfplugin5"
	"github.com/matt-FFFFFF/tfpluginschema/tfplugin6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertV6BlockToTFJSON_NestingModesAndAttributes(t *testing.T) {
	b := &tfplugin6.Schema_Block{
		Description:     "d",
		DescriptionKind: tfplugin6.StringKind_MARKDOWN,
		Deprecated:      true,
		Attributes: []*tfplugin6.Schema_Attribute{
			{
				Name:            "a",
				Description:     "ad",
				DescriptionKind: tfplugin6.StringKind_MARKDOWN,
				Required:        true,
				Optional:        false,
				Computed:        false,
				Sensitive:       true,
				WriteOnly:       true,
				Type:            []byte(`"string"`),
			},
		},
		BlockTypes: []*tfplugin6.Schema_NestedBlock{
			{TypeName: "single", Nesting: tfplugin6.Schema_NestedBlock_SINGLE, Block: &tfplugin6.Schema_Block{}},
			{TypeName: "group", Nesting: tfplugin6.Schema_NestedBlock_GROUP, Block: &tfplugin6.Schema_Block{}},
			{TypeName: "list", Nesting: tfplugin6.Schema_NestedBlock_LIST, MinItems: 1, MaxItems: 2, Block: &tfplugin6.Schema_Block{}},
			{TypeName: "set", Nesting: tfplugin6.Schema_NestedBlock_SET, Block: &tfplugin6.Schema_Block{}},
			{TypeName: "map", Nesting: tfplugin6.Schema_NestedBlock_MAP, Block: &tfplugin6.Schema_Block{}},
		},
	}

	sb := convertV6BlockToTFJSON(b)
	require.NotNil(t, sb)
	assert.Equal(t, tfjson.SchemaDescriptionKindMarkdown, sb.DescriptionKind)
	assert.True(t, sb.Deprecated)
	// attribute
	a := sb.Attributes["a"]
	require.NotNil(t, a)
	assert.Equal(t, tfjson.SchemaDescriptionKindMarkdown, a.DescriptionKind)
	assert.True(t, a.Required)
	assert.True(t, a.Sensitive)
	assert.True(t, a.WriteOnly)
	assert.True(t, a.AttributeType.IsPrimitiveType())
	// nested blocks modes
	assert.Equal(t, tfjson.SchemaNestingModeSingle, sb.NestedBlocks["single"].NestingMode)
	assert.Equal(t, tfjson.SchemaNestingModeGroup, sb.NestedBlocks["group"].NestingMode)
	lb := sb.NestedBlocks["list"]
	assert.Equal(t, tfjson.SchemaNestingModeList, lb.NestingMode)
	assert.Equal(t, uint64(1), lb.MinItems)
	assert.Equal(t, uint64(2), lb.MaxItems)
	assert.Equal(t, tfjson.SchemaNestingModeSet, sb.NestedBlocks["set"].NestingMode)
	assert.Equal(t, tfjson.SchemaNestingModeMap, sb.NestedBlocks["map"].NestingMode)
}

func TestConvertV6ObjectToNested_Recursive(t *testing.T) {
	obj := &tfplugin6.Schema_Object{
		Nesting: tfplugin6.Schema_Object_LIST,
		Attributes: []*tfplugin6.Schema_Attribute{
			{Name: "x", Type: []byte(`"number"`)},
			{Name: "child", NestedType: &tfplugin6.Schema_Object{Nesting: tfplugin6.Schema_Object_SINGLE}},
		},
	}
	n := convertV6ObjectToNested(obj)
	require.NotNil(t, n)
	assert.Equal(t, tfjson.SchemaNestingModeList, n.NestingMode)
	assert.True(t, n.Attributes["x"].AttributeType.IsPrimitiveType())
	assert.NotNil(t, n.Attributes["child"].AttributeNestedType)
}

func TestConvertV6FunctionToTFJSON_Full(t *testing.T) {
	f := &tfplugin6.Function{
		Summary:            "s",
		Description:        "d",
		DeprecationMessage: "dep",
		Parameters: []*tfplugin6.Function_Parameter{
			{Name: "p1", Description: "d1", AllowNullValue: true, Type: []byte(`"string"`)},
		},
		VariadicParameter: &tfplugin6.Function_Parameter{Name: "vp", Description: "dv", AllowNullValue: false, Type: []byte(`"number"`)},
		Return:            &tfplugin6.Function_Return{Type: []byte(`"bool"`)},
	}
	fs := convertV6FunctionToTFJSON(f)
	require.NotNil(t, fs)
	assert.Equal(t, "s", fs.Summary)
	require.Len(t, fs.Parameters, 1)
	assert.True(t, fs.Parameters[0].IsNullable)
	require.NotNil(t, fs.VariadicParameter)
	assert.False(t, fs.VariadicParameter.IsNullable)
	assert.True(t, fs.ReturnType.IsPrimitiveType())
}

func TestConvertV5SchemaToTFJSON_Parity(t *testing.T) {
	b := &tfplugin5.Schema_Block{
		Attributes: []*tfplugin5.Schema_Attribute{{Name: "a", Type: []byte(`"string"`), DescriptionKind: tfplugin5.StringKind_MARKDOWN}},
		BlockTypes: []*tfplugin5.Schema_NestedBlock{{TypeName: "single", Nesting: tfplugin5.Schema_NestedBlock_SINGLE, Block: &tfplugin5.Schema_Block{}}},
	}
	s := &tfplugin5.Schema{Version: 3, Block: b}
	res := convertV5SchemaToTFJSON(s)
	require.NotNil(t, res)
	assert.Equal(t, uint64(3), res.Version)
	assert.Equal(t, tfjson.SchemaDescriptionKindMarkdown, res.Block.Attributes["a"].DescriptionKind)
	assert.Equal(t, tfjson.SchemaNestingModeSingle, res.Block.NestedBlocks["single"].NestingMode)
}

func TestDecodeCtyTypeFromJSONBytes_Cases(t *testing.T) {
	// empty
	_, err := decodeCtyTypeFromJSONBytes(nil)
	require.Error(t, err)
	// invalid
	_, err = decodeCtyTypeFromJSONBytes([]byte("{not json}"))
	require.Error(t, err)
	// valid primitive
	ty, err := decodeCtyTypeFromJSONBytes([]byte(`"string"`))
	require.NoError(t, err)
	assert.True(t, ty.IsPrimitiveType())
	// valid container
	_, err = decodeCtyTypeFromJSONBytes([]byte(`{"list":"string"}`))
	require.NoError(t, err)
	// valid object
	_, err = decodeCtyTypeFromJSONBytes([]byte(`{"object":{"a":"number"}}`))
	require.NoError(t, err)
}
