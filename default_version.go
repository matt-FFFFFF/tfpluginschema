package tfpluginschema

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func versionOrLatest(namespace, providerType, version string) (string, error) {
	if version == "" {
		v, err := getLatestVersion(namespace, providerType)
		if err != nil {
			return "", err
		}
		version = v
	}
	return strings.TrimPrefix(version, "v"), nil
}

var getLatestVersion = func(namespace string, providerType string) (string, error) {
	url := fmt.Sprintf("https://registry.terraform.io/v1/providers/%s/%s", namespace, providerType)

	resp, err := http.Get(url) // #nosec G107
	if err != nil {
		return "", fmt.Errorf("failed to fetch provider info from registry: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry API returned status %d for provider %s/%s", resp.StatusCode, namespace, providerType)
	}

	var providerInfo struct {
		Tag string `json:"tag"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&providerInfo); err != nil {
		return "", fmt.Errorf("failed to decode provider info response: %w", err)
	}

	if providerInfo.Tag == "" {
		return "", fmt.Errorf("no tag found in provider info for %s/%s", namespace, providerType)
	}

	return providerInfo.Tag, nil
}
