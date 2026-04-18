package tfpluginschema

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	goversion "github.com/hashicorp/go-version"
)

const (
	pluginApiVersions = "versions"
)

type pluginApiVersionsResponse struct {
	Versions []struct {
		Version string `json:"version"`
	} `json:"versions"`
}

type VersionsRequest struct {
	Namespace    string       // Namespace of the provider (e.g., "hashicorp")
	Name         string       // Name of the provider (e.g., "aws")
	RegistryType RegistryType // Registry to use (defaults to OpenTofu if not specified)
}

// String returns a best-effort URL for the versions endpoint. It does not
// panic on invalid input; callers using the public Server API go through
// validateVersionsRequest, which rejects unsafe namespace/name values before
// a URL is ever constructed.
func (v VersionsRequest) String() string {
	sb := strings.Builder{}
	sb.WriteString(v.RegistryType.BaseURL())
	sb.WriteRune(urlPathSeparator)
	sb.WriteString(v.Namespace)
	sb.WriteRune(urlPathSeparator)
	sb.WriteString(v.Name)
	sb.WriteRune(urlPathSeparator)
	sb.WriteString(pluginApiVersions)
	return sb.String()
}

// validateVersionsRequest ensures namespace/name are non-empty and URL/path
// safe. It mirrors the identity-validation rules applied by Server.Get so
// that VersionsRequest.String() segments never need URL-escaping and can't
// alter URL semantics.
func validateVersionsRequest(req VersionsRequest) error {
	if err := validateCachePathComponent("namespace", req.Namespace, true); err != nil {
		return err
	}
	return validateCachePathComponent("name", req.Name, true)
}

// GetAvailableVersions fetches the available versions for the given provider from the plugin registry.
// It caches the results to avoid redundant network calls.
// It returns a sorted collection of versions.
func (s *Server) GetAvailableVersions(req VersionsRequest) (goversion.Collection, error) {
	if err := validateVersionsRequest(req); err != nil {
		return nil, fmt.Errorf("invalid versions request: %w", err)
	}

	// Normalize RegistryType so empty/unknown values share the same
	// cache key as the registry they actually resolve to via BaseURL
	// (OpenTofu). Without this, a caller passing an unknown RegistryType
	// would still hit OpenTofu but cache under a distinct key, producing
	// avoidable cache misses and duplicate network calls.
	req.RegistryType = normalizedRegistryType(req.RegistryType)

	l := s.l.With("request_namespace", req.Namespace, "request_name", req.Name)

	s.mu.RLock()
	if v, ok := s.versionsc[req]; ok {
		s.mu.RUnlock()
		l.Info("Request already exists in download cache")
		return v, nil
	}
	s.mu.RUnlock()

	var result pluginApiVersionsResponse

	versionRequest, err := http.NewRequest(http.MethodGet, req.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for versions: %w", err)
	}

	resp, err := s.httpClient.Do(versionRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to get versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get versions: %s => %d", req.String(), resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode versions response: %w", err)
	}

	var versions goversion.Collection
	for _, v := range result.Versions {
		ver, err := goversion.NewVersion(v.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to parse version %q: %w", v.Version, err)
		}
		versions = append(versions, ver)
	}

	slices.SortFunc(versions, func(a, b *goversion.Version) int {
		return a.Compare(b)
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	s.versionsc[req] = versions
	return versions, nil
}

// GetLatestVersionMatch returns the latest version from the provided collection that matches the given constraints.
// The versions collection must be sorted in ascending order.
// If no versions match the constraints, an error is returned.
// If the constraints are nil or empty, the latest version is returned.
func GetLatestVersionMatch(versions goversion.Collection, constraints goversion.Constraints) (*goversion.Version, error) {
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions provided")
	}

	if !slices.IsSortedFunc(versions, func(a, b *goversion.Version) int {
		return a.Compare(b)
	}) {
		return nil, fmt.Errorf("versions are not sorted")
	}

	// return latest if no constraints
	if constraints == nil || constraints.Len() == 0 {
		return versions[len(versions)-1], nil
	}

	var lastGood *goversion.Version
	for _, v := range versions {
		if !constraints.Check(v) {
			continue
		}
		lastGood = v
	}

	if lastGood == nil {
		return nil, fmt.Errorf("no matching version found")
	}

	return lastGood, nil
}
