package tfpluginschema_test

import (
	"encoding/json"
	"fmt"

	"github.com/matt-FFFFFF/tfpluginschema"
)

// ExampleNewServer demonstrates how to create a new server instance,
// download a provider, and retrieve its schema.
// It uses the Azure azapi provider as an example.
func ExampleNewServer() {
	s := tfpluginschema.NewServer(nil)
	defer s.Cleanup()
	request := tfpluginschema.Request{
		Namespace: "Azure",
		Name:      "azapi",
		Version:   "2.5.0",
	}

	bytes, err := s.GetProviderSchema(request)
	if err != nil {
		panic(err)
	}

	// Using this type to unmarshal the provider schema for display purposes.
	// If using this with an MCP server you can just output the JSON bytes directly.
	type exampleProviderSchema struct {
		Block struct {
			Attributes []struct {
				Name string `json:"name"`
			} `json:"attributes"`
		} `json:"block"`
	}

	var schema exampleProviderSchema

	if err := json.Unmarshal(bytes, &schema); err != nil {
		panic(err)
	}

	for _, attr := range schema.Block.Attributes {
		fmt.Printf("Attribute: %s\n", attr.Name)
	}

	// Output:
	// Attribute: auxiliary_tenant_ids
	// Attribute: client_certificate
	// Attribute: client_certificate_password
	// Attribute: client_certificate_path
	// Attribute: client_id
	// Attribute: client_id_file_path
	// Attribute: client_secret
	// Attribute: client_secret_file_path
	// Attribute: custom_correlation_request_id
	// Attribute: default_location
	// Attribute: default_name
	// Attribute: default_tags
	// Attribute: disable_correlation_request_id
	// Attribute: disable_default_output
	// Attribute: disable_terraform_partner_id
	// Attribute: enable_preflight
	// Attribute: endpoint
	// Attribute: environment
	// Attribute: ignore_no_op_changes
	// Attribute: maximum_busy_retry_attempts
	// Attribute: oidc_azure_service_connection_id
	// Attribute: oidc_request_token
	// Attribute: oidc_request_url
	// Attribute: oidc_token
	// Attribute: oidc_token_file_path
	// Attribute: partner_id
	// Attribute: skip_provider_registration
	// Attribute: subscription_id
	// Attribute: tenant_id
	// Attribute: use_aks_workload_identity
	// Attribute: use_cli
	// Attribute: use_msi
	// Attribute: use_oidc
}
