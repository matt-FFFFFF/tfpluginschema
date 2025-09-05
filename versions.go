package tfpluginschema

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	Namespace string
	Name      string
}

func (v VersionsRequest) String() string {
	sb := strings.Builder{}
	sb.WriteString(pluginApi)
	sb.WriteRune(urlPathSeparator)
	sb.WriteString(v.Namespace)
	sb.WriteRune(urlPathSeparator)
	sb.WriteString(v.Name)
	sb.WriteRune(urlPathSeparator)
	sb.WriteString(pluginApiVersions)
	result := sb.String()
	if _, err := url.Parse(result); err != nil {
		panic(fmt.Sprintf("failed to parse URL: %s, error: %v", result, err))
	}
	return result
}

// GetAvailableVersions fetches the available versions for the given provider from the plugin registry.
// It caches the results to avoid redundant network calls.
// It returns a sorted collection of versions.
func (s *Server) GetAvailableVersions(req VersionsRequest) (goversion.Collection, error) {
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

	resp, err := http.DefaultClient.Do(versionRequest)
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
		if constraints.Check(v) {
			lastGood = v
		}
	}

	if lastGood == nil {
		return nil, fmt.Errorf("no matching version found")
	}

	return lastGood, nil
}
