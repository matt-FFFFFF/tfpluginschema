package tfpluginschema

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	PluginApi              = "https://registry.opentofu.org/v1/providers"
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
	Namespace string // Namespace of the provider (e.g., "Azure")
	Name      string // Name of the provider (e.g., "azapi")
	Version   string // Version of the provider (e.g., "2.5.0")
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

type pluginApiResponse struct {
	Protocols   []string `json:"protocols"`
	OS          string   `json:"os"`
	Arch        string   `json:"arch"`
	FileName    string   `json:"filename"`
	DownloadURL string   `json:"download_url"`
}

type downloadCache map[Request]string
type schemaCache map[Request]schemaResponse

// Server is a struct that manages the plugin download and caching process.
type Server struct {
	tmpDir string
	dlc    downloadCache
	sc     schemaCache
	l      *slog.Logger
}

func NewServer(l *slog.Logger) *Server {
	if l == nil {
		l = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: false,
		}))
	}
	l.Info("Creating new server instance")
	return &Server{
		dlc: make(downloadCache),
		sc:  make(schemaCache),
		l:   l,
	}
}

// Cleanup removes the temporary directory used for plugin downloads.
func (s *Server) Cleanup() {
	s.l.Info("Cleaning up temporary directory", "dir", s.tmpDir)
	os.RemoveAll(s.tmpDir)
}

func (s *Server) Get(request Request) error {
	l := s.l.With("request_namespace", request.Namespace, "request_name", request.Name, "request_version", request.Version)
	if _, exists := s.dlc[request]; exists {
		l.Info("Request already exists in download cache")
		return nil // Request already exists, no need to add again
	}

	registryApiRequest, err := http.NewRequest(http.MethodGet, request.String(), nil)
	l.Debug("Sending request to registry API", "url", registryApiRequest.URL.String())
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

	var pluginResponse pluginApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&pluginResponse); err != nil {
		return fmt.Errorf("failed to decode plugin API response: %w", err)
	}

	l.Info("Plugin API response received", "arch", pluginResponse.Arch, "os", pluginResponse.OS, "filename", pluginResponse.FileName, "download_url", pluginResponse.DownloadURL)

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
	wantProviderFileName := fmt.Sprintf("%s%s", ProviderFileNamePrefix, request.Name)
	if err = fs.WalkDir(os.DirFS(extractDir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking extracted directory (%s): %w", extractDir, err)
		}

		l.Debug("Checking extracted file", "path", path, "is_dir", d.IsDir(), "name", d.Name())

		if d.IsDir() && d.Name() != "." {
			return fs.SkipDir
		}

		if !strings.HasPrefix(d.Name(), wantProviderFileName) {
			return nil
		}

		l.Info("Found provider file", "provider_file_name", d.Name())

		s.dlc[request] = filepath.Join(extractDir, path)
		return fs.SkipAll
	}); err != nil {
		return fmt.Errorf("error checking extracted files: %w", err)
	}

	if _, exists := s.dlc[request]; !exists {
		return fmt.Errorf("provider file not found in extracted directory (%s) for request: %s", extractDir, request.String())
	}

	return nil
}

// GetProviderClient creates a universal provider client for the given request
func (s *Server) getSchema(request Request) ([]byte, error) {
	if resp, exists := s.sc[request]; exists {
		return json.Marshal(resp)
	}

	// Ensure the provider is downloaded
	if err := s.Get(request); err != nil {
		return nil, fmt.Errorf("failed to download provider: %w", err)
	}

	// Get the provider path
	providerPath, exists := s.dlc[request]
	if !exists {
		return nil, fmt.Errorf("provider not found in cache: %s", request.String())
	}

	client, err := newGrpcClient(providerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}
	defer client.close()

	var respAny any

	if resp, err := client.v6Schema(); err == nil {
		respAny = resp
	}

	if resp, err := client.v5Schema(); err == nil {
		respAny = resp
	}

	if respAny == nil {
		return nil, fmt.Errorf("failed to get provider schema for either V5 or V6 protocols")
	}

	response, err := json.Marshal(respAny)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal provider schema response: %w", err)
	}

	var schemaResp schemaResponse
	if err := json.Unmarshal(response, &schemaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal provider schema response: %w", err)
	}

	// Cache the schema response
	s.sc[request] = schemaResp

	return json.MarshalIndent(schemaResp, "", "  ")
}

// GetResourceSchema retrieves the schema for a specific resource from the provider.
func (s *Server) GetResourceSchema(request Request, resource string) ([]byte, error) {
	s.l.Info("Getting resource schema", "request", request, "resource", resource)
	schemaResp, ok := s.sc[request]
	if !ok {
		if _, err := s.getSchema(request); err != nil {
			return nil, fmt.Errorf("failed to read provider schema: %w", err)
		}
		schemaResp = s.sc[request]
	}

	schemaResource, ok := schemaResp.ResourceSchemas[resource]
	if !ok {
		return nil, fmt.Errorf("resource schema not found: %s", resource)
	}

	// Apply type field decoding to the individual resource schema
	decodedResource := decodeTypeFields(schemaResource)
	return json.MarshalIndent(decodedResource, "", "  ")
}

// GetDataSourceSchema retrieves the schema for a specific data source from the provider.
func (s *Server) GetDataSourceSchema(request Request, dataSource string) ([]byte, error) {
	s.l.Info("Getting data source schema", "request", request, "data_source", dataSource)
	schemaResp, ok := s.sc[request]
	if !ok {
		if _, err := s.getSchema(request); err != nil {
			return nil, fmt.Errorf("failed to read provider schema: %w", err)
		}
		schemaResp = s.sc[request]
	}

	schemaResource, ok := schemaResp.DataSourceSchemas[dataSource]
	if !ok {
		return nil, fmt.Errorf("data source schema not found: %s", dataSource)
	}

	// Apply type field decoding to the individual data source schema
	decodedResource := decodeTypeFields(schemaResource)
	return json.MarshalIndent(decodedResource, "", "  ")
}

// GetFunctionSchema retrieves the schema for a specific function from the provider.
func (s *Server) GetFunctionSchema(request Request, function string) ([]byte, error) {
	s.l.Info("Getting function schema", "request", request, "function", function)
	schemaResp, ok := s.sc[request]
	if !ok {
		if _, err := s.getSchema(request); err != nil {
			return nil, fmt.Errorf("failed to read provider schema: %w", err)
		}
		schemaResp = s.sc[request]
	}

	schemaFunction, ok := schemaResp.Functions[function]
	if !ok {
		return nil, fmt.Errorf("function schema not found: %s", function)
	}

	// Apply type field decoding to the individual function schema
	decodedFunction := decodeTypeFields(schemaFunction)
	return json.MarshalIndent(decodedFunction, "", "  ")
}
