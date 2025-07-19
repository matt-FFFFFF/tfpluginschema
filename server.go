package tfpluginschema

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/provider"
)

const (
	PluginApi              = "https://registry.terraform.io/v1/providers"
	ProviderFileNamePrefix = "terraform-provider-"
	UrlPathSeparator       = '/'
)

var (
	ErrPluginNotFound = fmt.Errorf("plugin not found")
	ErrPluginApi      = fmt.Errorf("plugin API error")
)

// Request is a request structure used to specify the details of a plugin
// so that it can be downloaded.
type Request struct {
	Namespace string
	Name      string
	Version   string
}

// String returns a string representation of the Request in the format:
// "https://registry.terraform.io/v1/providers/{namespace}/{name}/{version}/{os}/{arch}"
// This format is used to construct the URL for downloading the plugin.
func (r Request) String() string {
	sb := strings.Builder{}
	sb.WriteString(PluginApi)
	sb.WriteRune(UrlPathSeparator)
	sb.WriteString(r.Namespace)
	sb.WriteRune(UrlPathSeparator)
	sb.WriteString(r.Name)
	sb.WriteRune(UrlPathSeparator)
	sb.WriteString(r.Version)
	sb.WriteString("/download/")
	sb.WriteString(runtime.GOOS)
	sb.WriteRune(UrlPathSeparator)
	sb.WriteString(runtime.GOARCH)
	result := sb.String()
	if _, err := url.Parse(result); err != nil {
		panic(fmt.Sprintf("failed to parse URL: %s, error: %v", result, err))
	}
	return result
}

type PluginApiResponse struct {
	Protocols   []string `json:"protocols"`
	OS          string   `json:"os"`
	Arch        string   `json:"arch"`
	FileName    string   `json:"filename"`
	DownloadURL string   `json:"download_url"`
}

type downloadCache map[Request]string
type schemaCache map[Request]*provider.SchemaResponse

// Server is a struct that manages the plugin download and caching process.
type Server struct {
	tmpDir        string
	downloadCache downloadCache
	schemaCache   schemaCache
}

func NewServer() *Server {
	return &Server{
		downloadCache: make(downloadCache),
		schemaCache:   make(schemaCache),
	}
}

func (s *Server) Cleanup() {
	os.RemoveAll(s.tmpDir)
}

func (s *Server) Get(request Request) error {
	if _, exists := s.downloadCache[request]; exists {
		return nil // Request already exists, no need to add again
	}

	registryApiRequest, err := http.NewRequest(http.MethodGet, request.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for registry API: %w", err)
	}

	resp, err := http.DefaultClient.Do(registryApiRequest)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request to registry API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %s", ErrPluginNotFound, request.String())
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s => %d", ErrPluginApi, request.String(), resp.StatusCode)
	}

	var pluginResponse PluginApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&pluginResponse); err != nil {
		return fmt.Errorf("failed to decode plugin API response: %w", err)
	}

	if s.tmpDir == "" {
		tmpFile, err := os.MkdirTemp("", "tfpluginschema-")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory: %w", err)
		}
		s.tmpDir = tmpFile
	}

	downloadURL := pluginResponse.DownloadURL
	if downloadURL == "" {
		return fmt.Errorf("download URL is empty for request: %s", request.String())
	}

	downloadRequest, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for plugin download: %w", err)
	}

	resp, err = http.DefaultClient.Do(downloadRequest)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download plugin: %s => %d", downloadURL, resp.StatusCode)
	}

	pluginFilePath := filepath.Join(s.tmpDir, pluginResponse.FileName)

	file, err := os.Create(pluginFilePath)
	if err != nil {
		return fmt.Errorf("failed to create plugin file: %w", err)
	}

	defer file.Close()

	if _, err := file.ReadFrom(resp.Body); err != nil {
		return fmt.Errorf("failed to read plugin data into file: %w", err)
	}

	// unzip the file
	extractDir := strings.TrimSuffix(pluginResponse.FileName, filepath.Ext(pluginResponse.FileName)) // Remove extension for directory name
	extractDir = filepath.Join(s.tmpDir, extractDir)

	if err := os.Mkdir(extractDir, 0755); err != nil {
		return fmt.Errorf("failed to create extraction directory: %w", err)
	}

	if err := unzip(pluginFilePath, extractDir); err != nil {
		return fmt.Errorf("failed to unzip plugin file: %w", err)
	}

	// check the extracted directory
	providerFileName := fmt.Sprintf("%s%s_v%s", ProviderFileNamePrefix, request.Name, request.Version)
	if err = fs.WalkDir(os.DirFS(extractDir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking extracted directory (%s): %w", extractDir, err)
		}

		if d.IsDir() && d.Name() != "." {
			return fs.SkipDir
		}

		if !strings.HasPrefix(d.Name(), providerFileName) {
			return nil
		}

		s.downloadCache[request] = filepath.Join(extractDir, path)
		return fs.SkipAll
	}); err != nil {
		return fmt.Errorf("error checking extracted files: %w", err)
	}

	if _, exists := s.downloadCache[request]; !exists {
		return fmt.Errorf("provider file not found in extracted directory (%s) for request: %s", extractDir, request.String())
	}

	return nil
}

// GetUniversalClient creates a universal provider client for the given request
func (s *Server) GetUniversalClient(request Request) (ProviderClient, error) {
	// Ensure the provider is downloaded
	if err := s.Get(request); err != nil {
		return nil, fmt.Errorf("failed to download provider: %w", err)
	}

	// Get the provider path
	providerPath, exists := s.downloadCache[request]
	if !exists {
		return nil, fmt.Errorf("provider not found in cache: %s", request.String())
	}

	// Create the universal client
	config := ClientConfig{
		ProviderPath: providerPath,
	}

	return NewProviderClient(config)
}
