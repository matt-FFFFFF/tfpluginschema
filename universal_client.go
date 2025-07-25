package tfpluginschema

import (
	"context"
	"encoding/json"
	"fmt"
)

// ProviderClient provides a unified interface for both protocol v5 and v6
type ProviderClient interface {
	GetProviderSchema(ctx context.Context) ([]byte, error) // Returns JSON
	Close() error
}

// NewProviderClient creates a provider client that automatically selects the best protocol version
func NewProviderClient(providerPath string) (ProviderClient, error) {
	// Try v6 first
	client, err := newV6UniversalClient(providerPath)
	if err == nil {
		return client, nil
	}

	// If v6 fails, try v5
	client5, err5 := newV5UniversalClient(providerPath)
	if err5 == nil {
		return client5, nil
	}

	// If both fail, return the v6 error
	return nil, fmt.Errorf("failed to connect with v6: %w, failed to connect with v5: %w", err, err5)
}

// v6UniversalClient wraps the V6 client to implement UniversalProviderClient
type v6UniversalClient struct {
	client  V6Provider
	cleanup func()
}

func newV6UniversalClient(providerPath string) (*v6UniversalClient, error) {
	client, cleanupFunc, err := NewClientV6(providerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create v6 client: %w", err)
	}

	return &v6UniversalClient{
		client:  client,
		cleanup: cleanupFunc,
	}, nil
}

func (c *v6UniversalClient) GetProviderSchema(ctx context.Context) ([]byte, error) {
	resp, err := c.client.V6Schema(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get v6 schema: %w", err)
	}

	return json.Marshal(resp)
}

func (c *v6UniversalClient) Close() error {
	c.cleanup()
	return nil
}

// v5UniversalClient wraps the V5 client to implement UniversalProviderClient
type v5UniversalClient struct {
	client  V5Provider
	cleanup func()
}

func newV5UniversalClient(providerPath string) (*v5UniversalClient, error) {
	client, cleanupFunc, err := NewClientV5(providerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create v5 client: %w", err)
	}

	return &v5UniversalClient{
		client:  client,
		cleanup: cleanupFunc,
	}, nil
}

func (c *v5UniversalClient) GetProviderSchema(ctx context.Context) ([]byte, error) {
	resp, err := c.client.V5Schema(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get v5 schema: %w", err)
	}

	return json.Marshal(resp)
}

func (c *v5UniversalClient) Close() error {
	c.cleanup()
	return nil
}
