package tfpluginschema

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	goversion "github.com/hashicorp/go-version"
	tfjson "github.com/hashicorp/terraform-json"
)

const (
	pluginApi              = "https://registry.opentofu.org/v1/providers"
	providerFileNamePrefix = "terraform-provider-"
	urlPathSeparator       = '/'
)

var (
	ErrPluginNotFound = fmt.Errorf("plugin not found")
	ErrPluginApi      = fmt.Errorf("plugin API error")
)

// ContextKey is a type used to store the server instance in the context.
type ContextKey struct{}

// Request is a request structure used to specify the details of a plugin
// so that it can be downloaded.
// Note that the request fields are case-sensitive.
type Request struct {
	Namespace string // Namespace of the provider (e.g., "Azure")
	Name      string // Name of the provider (e.g., "azapi")
	Version   string // Version of the provider (e.g., "2.5.0") or constraint (e.g., ">=1.0.0", "~>2.1")
}

// String returns a string representation of the Request in the format:
// "https://registry.opentofu.org/v1/providers/{namespace}/{name}/{version}/download/{os}/{arch}"
// This format is used to construct the URL for downloading the plugin.
func (r Request) String() string {
	sb := strings.Builder{}
	sb.WriteString(pluginApi)
	sb.WriteRune(urlPathSeparator)
	sb.WriteString(r.Namespace)
	sb.WriteRune(urlPathSeparator)
	sb.WriteString(r.Name)
	sb.WriteRune(urlPathSeparator)
	sb.WriteString(r.Version)
	sb.WriteString("/download/")
	sb.WriteString(runtime.GOOS)
	sb.WriteRune(urlPathSeparator)
	sb.WriteString(runtime.GOARCH)
	result := sb.String()
	if _, err := url.Parse(result); err != nil {
		panic(fmt.Sprintf("failed to parse URL: %s, error: %v", result, err))
	}
	return result
}

func (r Request) fixedVersion() bool {
	_, err := goversion.NewVersion(r.Version)
	return err == nil
}

func (r Request) fixVersion(s *Server) (Request, error) {
	if !r.fixedVersion() {
		ver, err := s.latestVersionOf(r)
		if err != nil {
			return Request{}, fmt.Errorf("failed to get latest version: %w", err)
		}
		r.Version = ver
		s.l.Info("No version specified, using latest version", "version", r.Version)
	}
	return r, nil
}

type pluginApiResponse struct {
	Protocols   []string `json:"protocols"`
	OS          string   `json:"os"`
	Arch        string   `json:"arch"`
	FileName    string   `json:"filename"`
	DownloadURL string   `json:"download_url"`
}

type downloadCache map[Request]string
type schemaCache map[Request]*tfjson.ProviderSchema
type versionsCache map[VersionsRequest]goversion.Collection

// Server is a struct that manages the plugin download and caching process.
type Server struct {
	tmpDir    string
	dlc       downloadCache
	sc        schemaCache
	l         *slog.Logger
	versionsc versionsCache
	mu        *sync.RWMutex
}

// NewServer creates a new Server instance with an optional logger.
// If no logger is provided, it defaults to a logger that discards all logs.
func NewServer(l *slog.Logger) *Server {
	if l == nil {
		l = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
			Level:     slog.LevelError,
			AddSource: false,
		}))
	}
	l.Info("Creating new server instance")
	return &Server{
		dlc:       make(downloadCache),
		sc:        make(schemaCache),
		l:         l,
		versionsc: make(versionsCache),
		mu:        &sync.RWMutex{},
	}
}

// Cleanup removes the temporary directory used for plugin downloads.
func (s *Server) Cleanup() {
	s.l.Info("Cleaning up temporary directory", "dir", s.tmpDir)
	os.RemoveAll(s.tmpDir)
}

// Get retrieves the plugin for the specified request, downloading it if necessary.
// The GetXxx methods (GetResourceSchema, GetDataSourceSchema, etc.) will call this method anyway,
// so it is not necessary to call Get directly unless you want to ensure the plugin is downloaded first.
// It is stored in a temporary directory and cached for future use.
// Make sure to call Cleanup() to remove the temporary files.
func (s *Server) Get(request Request) error {
	l := s.l.With("request_namespace", request.Namespace, "request_name", request.Name, "request_version", request.Version)
	s.mu.RLock()
	if _, exists := s.dlc[request]; exists {
		l.Info("Request already exists in download cache")
		s.mu.RUnlock()
		return nil // Request already exists, no need to add again
	}
	s.mu.RUnlock()

	var err error
	if request, err = request.fixVersion(s); err != nil {
		return err
	}

	// Lock for the download and extraction process to avoid multiple downloads of the same plugin
	s.mu.Lock()
	defer s.mu.Unlock()

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
	wantProviderFileName := fmt.Sprintf("%s%s", providerFileNamePrefix, request.Name)
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

	// At this point we still hold the write lock (deferred Unlock above), so we must NOT
	// attempt to acquire a read lock again (doing so deadlocks). Just check directly.
	if _, exists := s.dlc[request]; !exists {
		return fmt.Errorf("provider file not found in extracted directory (%s) for request: %s", extractDir, request.String())
	}

	return nil
}

// GetResourceSchema retrieves the schema for a specific resource from the provider.
func (s *Server) GetResourceSchema(request Request, resource string) (*tfjson.Schema, error) {
	s.l.Info("Getting resource schema", "request", request, "resource", resource)

	s.mu.RLock()
	schemaResp, ok := s.sc[request]
	s.mu.RUnlock()

	if !ok {
		if _, err := s.getSchema(request); err != nil {
			return nil, fmt.Errorf("failed to read provider schema: %w", err)
		}

		s.mu.RLock()
		schemaResp = s.sc[request]
		s.mu.RUnlock()
	}

	schemaResource, ok := schemaResp.ResourceSchemas[resource]
	if !ok {
		return nil, fmt.Errorf("resource schema not found: %s", resource)
	}

	return schemaResource, nil
}

// GetDataSourceSchema retrieves the schema for a specific data source from the provider.
func (s *Server) GetDataSourceSchema(request Request, dataSource string) (*tfjson.Schema, error) {
	s.l.Info("Getting data source schema", "request", request, "data_source", dataSource)

	s.mu.RLock()
	schemaResp, ok := s.sc[request]
	s.mu.RUnlock()

	if !ok {
		if _, err := s.getSchema(request); err != nil {
			return nil, fmt.Errorf("failed to read provider schema: %w", err)
		}
		s.mu.RLock()
		schemaResp = s.sc[request]
		s.mu.RUnlock()
	}

	schemaResource, ok := schemaResp.DataSourceSchemas[dataSource]
	if !ok {
		return nil, fmt.Errorf("data source schema not found: %s", dataSource)
	}

	return schemaResource, nil
}

// GetFunctionSchema retrieves the schema for a specific function from the provider.
func (s *Server) GetFunctionSchema(request Request, function string) (*tfjson.FunctionSignature, error) {
	s.l.Info("Getting function schema", "request", request, "function", function)

	s.mu.RLock()
	schemaResp, ok := s.sc[request]
	s.mu.RUnlock()

	if !ok {
		if _, err := s.getSchema(request); err != nil {
			return nil, fmt.Errorf("failed to read provider schema: %w", err)
		}
		s.mu.RLock()
		schemaResp = s.sc[request]
		s.mu.RUnlock()
	}

	schemaFunction, ok := schemaResp.Functions[function]
	if !ok {
		return nil, fmt.Errorf("function schema not found: %s", function)
	}
	return schemaFunction, nil
}

// GetEphemeralResourceSchema retrieves the schema for a specific ephemeral resource from the provider.
func (s *Server) GetEphemeralResourceSchema(request Request, ephemeralResource string) (*tfjson.Schema, error) {
	s.l.Info("Getting ephemeral resource schema", "request", request, "ephemeral_resource", ephemeralResource)

	s.mu.RLock()
	schemaResp, ok := s.sc[request]
	s.mu.RUnlock()

	if !ok {
		if _, err := s.getSchema(request); err != nil {
			return nil, fmt.Errorf("failed to read provider schema: %w", err)
		}
		s.mu.RLock()
		schemaResp = s.sc[request]
		s.mu.RUnlock()
	}

	schemaResource, ok := schemaResp.EphemeralResourceSchemas[ephemeralResource]
	if !ok {
		return nil, fmt.Errorf("ephemeral resource schema not found: %s", ephemeralResource)
	}

	return schemaResource, nil
}

// GetProviderSchema retrieves the schema for the provider configuration.
func (s *Server) GetProviderSchema(request Request) (*tfjson.Schema, error) {
	s.l.Info("Getting provider schema", "request", request)

	s.mu.RLock()
	schemaResp, ok := s.sc[request]
	s.mu.RUnlock()

	if !ok {
		if _, err := s.getSchema(request); err != nil {
			return nil, fmt.Errorf("failed to read provider schema: %w", err)
		}
		s.mu.RLock()
		schemaResp = s.sc[request]
		s.mu.RUnlock()
	}
	return schemaResp.ConfigSchema, nil
}

// ListResources retrieves the list of resource names from the provider.
func (s *Server) ListResources(request Request) ([]string, error) {
	s.l.Info("Listing resources", "request", request)

	s.mu.RLock()
	schemaResp, ok := s.sc[request]
	s.mu.RUnlock()

	if !ok {
		if _, err := s.getSchema(request); err != nil {
			return nil, fmt.Errorf("failed to read provider schema: %w", err)
		}

		s.mu.RLock()
		schemaResp = s.sc[request]
		s.mu.RUnlock()
	}

	if schemaResp.ResourceSchemas == nil {
		return nil, nil
	}

	resources := make([]string, 0, len(schemaResp.ResourceSchemas))
	for name := range schemaResp.ResourceSchemas {
		resources = append(resources, name)
	}

	slices.Sort(resources)
	return resources, nil
}

// ListDataSources retrieves the list of data source names from the provider.
func (s *Server) ListDataSources(request Request) ([]string, error) {
	s.l.Info("Listing data sources", "request", request)

	s.mu.RLock()
	schemaResp, ok := s.sc[request]
	s.mu.RUnlock()

	if !ok {
		if _, err := s.getSchema(request); err != nil {
			return nil, fmt.Errorf("failed to read provider schema: %w", err)
		}

		s.mu.RLock()
		schemaResp = s.sc[request]
		s.mu.RUnlock()
	}

	if schemaResp.DataSourceSchemas == nil {
		return nil, nil
	}

	dataSources := make([]string, 0, len(schemaResp.DataSourceSchemas))
	for name := range schemaResp.DataSourceSchemas {
		dataSources = append(dataSources, name)
	}

	slices.Sort(dataSources)
	return dataSources, nil
}

// ListFunctions retrieves the list of function names from the provider.
func (s *Server) ListFunctions(request Request) ([]string, error) {
	s.l.Info("Listing functions", "request", request)

	s.mu.RLock()
	schemaResp, ok := s.sc[request]
	s.mu.RUnlock()

	if !ok {
		if _, err := s.getSchema(request); err != nil {
			return nil, fmt.Errorf("failed to read provider schema: %w", err)
		}
		s.mu.RLock()
		schemaResp = s.sc[request]
		s.mu.RUnlock()
	}

	if schemaResp.Functions == nil {
		return nil, nil
	}

	functions := make([]string, 0, len(schemaResp.Functions))
	for name := range schemaResp.Functions {
		functions = append(functions, name)
	}

	slices.Sort(functions)
	return functions, nil
}

// ListEphemeralResources retrieves the list of ephemeral resource names from the provider.
func (s *Server) ListEphemeralResources(request Request) ([]string, error) {
	s.l.Info("Listing ephemeral resources", "request", request)

	s.mu.RLock()
	schemaResp, ok := s.sc[request]
	s.mu.RUnlock()

	if !ok {
		if _, err := s.getSchema(request); err != nil {
			return nil, fmt.Errorf("failed to read provider schema: %w", err)
		}
		s.mu.RLock()
		schemaResp = s.sc[request]
		s.mu.RUnlock()
	}

	if schemaResp.EphemeralResourceSchemas == nil {
		return nil, nil
	}

	ephemeralResources := make([]string, 0, len(schemaResp.EphemeralResourceSchemas))
	for name := range schemaResp.EphemeralResourceSchemas {
		ephemeralResources = append(ephemeralResources, name)
	}

	slices.Sort(ephemeralResources)
	return ephemeralResources, nil
}

// getSchema creates a universal provider client for the given request
func (s *Server) getSchema(request Request) (*tfjson.ProviderSchema, error) {
	s.l.Info("Getting provider schema", "request", request)

	s.mu.RLock()
	if resp, exists := s.sc[request]; exists {
		s.mu.RUnlock()
		return resp, nil
	}
	s.mu.RUnlock()

	var err error
	if request, err = request.fixVersion(s); err != nil {
		return nil, err
	}

	// Ensure the provider is downloaded
	if err := s.Get(request); err != nil {
		return nil, fmt.Errorf("failed to download provider: %w", err)
	}

	// Get the provider path
	s.mu.RLock()
	providerPath, exists := s.dlc[request]
	if !exists {
		s.mu.RUnlock()
		return nil, fmt.Errorf("provider not found in cache: %s", request.String())
	}
	s.mu.RUnlock()

	client, err := newGrpcClient(providerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}
	defer client.close()

	// Use the unified Schema() method to retrieve a terraform-json ProviderSchema
	providerSchema, err := client.schema()
	if err != nil {
		return nil, fmt.Errorf("failed to get provider schema: %w", err)
	}

	if providerSchema == nil {
		return nil, errors.New("provider schema is nil")
	}

	// cache and return
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sc[request] = providerSchema
	return s.sc[request], nil
}

// latestVersionOf returns the latest version from the provided collection that matches the given constraints.
func (s *Server) latestVersionOf(request Request) (string, error) {
	vers, err := s.GetAvailableVersions(VersionsRequest{
		Namespace: request.Namespace,
		Name:      request.Name,
	})

	if err != nil {
		return "", fmt.Errorf("failed to get available versions: %w", err)
	}

	if len(vers) == 0 {
		return "", fmt.Errorf("no available versions found for provider: %s/%s", request.Namespace, request.Name)
	}

	var constraints goversion.Constraints
	if c, err := goversion.NewConstraint(request.Version); err == nil {
		constraints = c
	}

	latest, err := GetLatestVersionMatch(vers, constraints)
	if err != nil {
		return "", fmt.Errorf("failed to get latest version: %w", err)
	}

	return latest.String(), nil
}
