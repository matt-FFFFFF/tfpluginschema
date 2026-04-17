package tfpluginschema

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
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

// ensureWithinBaseDir returns an error if targetDir is not contained within
// baseDir once both have been lexically cleaned and symlinks in any existing
// ancestor of targetDir have been resolved. This protects filesystem
// operations (os.RemoveAll, os.MkdirAll, extraction) against both lexical
// escapes ("../") and symlink-based escapes where a path segment inside the
// cache directory points outside of it.
func ensureWithinBaseDir(baseDir, targetDir string) error {
	baseClean := filepath.Clean(baseDir)
	targetClean := filepath.Clean(targetDir)

	// Lexical check first, cheap and deterministic.
	rel, err := filepath.Rel(baseClean, targetClean)
	if err != nil {
		return fmt.Errorf("failed to evaluate cache path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("computed cache path %q escapes cache root %q", targetClean, baseClean)
	}

	// Resolve symlinks on the base directory (if it exists). If baseDir does
	// not yet exist, EvalSymlinks returns an error; that's fine — there are
	// no symlinks to escape through in that case.
	baseReal, err := filepath.EvalSymlinks(baseClean)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to resolve cache root %q: %w", baseClean, err)
	}

	// Walk up targetClean to the deepest existing ancestor and resolve
	// symlinks on it; then confirm containment.
	existing := targetClean
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("failed to stat %q: %w", existing, err)
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			// Reached filesystem root without finding an existing ancestor;
			// this shouldn't happen because baseReal exists, but be safe.
			return nil
		}
		existing = parent
	}

	existingReal, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return fmt.Errorf("failed to resolve %q: %w", existing, err)
	}

	rel2, err := filepath.Rel(baseReal, existingReal)
	if err != nil {
		return fmt.Errorf("failed to evaluate resolved cache path: %w", err)
	}
	if rel2 == ".." || strings.HasPrefix(rel2, ".."+string(filepath.Separator)) || filepath.IsAbs(rel2) {
		return fmt.Errorf("resolved cache path %q escapes cache root %q (symlink)", existingReal, baseReal)
	}

	return nil
}

// RegistryType represents the type of provider registry to use.
type RegistryType string

const (
	// RegistryTypeOpenTofu represents the OpenTofu registry (default).
	RegistryTypeOpenTofu RegistryType = "opentofu"
	// RegistryTypeTerraform represents the Terraform registry.
	RegistryTypeTerraform RegistryType = "terraform"
)

// BaseURL returns the base URL for the registry API.
// It defaults to OpenTofu registry for empty or unknown registry types.
func (r RegistryType) BaseURL() string {
	switch r {
	case RegistryTypeTerraform:
		return "https://registry.terraform.io/v1/providers"
	default:
		return "https://registry.opentofu.org/v1/providers"
	}
}

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
	Namespace    string       // Namespace of the provider (e.g., "Azure")
	Name         string       // Name of the provider (e.g., "azapi")
	Version      string       // Version of the provider (e.g., "2.5.0") or constraint (e.g., ">=1.0.0", "~>2.1")
	RegistryType RegistryType // Registry to use (defaults to OpenTofu if not specified)
}

// String returns a string representation of the Request in the format:
// "https://{registry}/v1/providers/{namespace}/{name}/{version}/download/{os}/{arch}"
// where {registry} is either registry.opentofu.org (default) or registry.terraform.io.
// This format is used to construct the URL for downloading the plugin.
// Note: String is a best-effort representation. Server.Get validates the
// request's components before constructing the URL, so callers using the
// public Server API do not need to pre-validate Request fields.
func (r Request) String() string {
	sb := strings.Builder{}
	sb.WriteString(r.RegistryType.BaseURL())
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
	return sb.String()
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
	tmpDir        string
	dlc           downloadCache
	sc            schemaCache
	l             *slog.Logger
	versionsc     versionsCache
	mu            *sync.RWMutex
	cacheDir      string
	forceFetch    bool
	cacheStatusFn CacheStatusFunc
	httpClient    *http.Client
}

// NewServer creates a new Server instance with an optional logger and zero or
// more ServerOption values for customization (cache directory, force fetch,
// cache-status callback, ...).
// If no logger is provided, it defaults to a logger that discards all logs.
func NewServer(l *slog.Logger, opts ...ServerOption) *Server {
	if l == nil {
		l = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
			Level:     slog.LevelError,
			AddSource: false,
		}))
	}
	l.Info("Creating new server instance")
	s := &Server{
		dlc:        make(downloadCache),
		sc:         make(schemaCache),
		l:          l,
		versionsc:  make(versionsCache),
		mu:         &sync.RWMutex{},
		cacheDir:   defaultCacheDir(),
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(s)
	}
	l.Debug("Server configured", "cache_dir", s.cacheDir, "force_fetch", s.forceFetch)
	return s
}

// CacheDir returns the root directory used by the Server to cache downloaded
// providers.
func (s *Server) CacheDir() string {
	return s.cacheDir
}

func (s *Server) readSchema(request Request) (*tfjson.ProviderSchema, error) {
	if !request.fixedVersion() {
		var err error
		if request, err = request.fixVersion(s); err != nil {
			return nil, err
		}
	}

	resp, err := s.getSchema(request)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider schema: %w", err)
	}

	if resp == nil {
		return nil, errors.New("provider schema is nil but no error was returned")
	}

	return resp, nil
}

// Cleanup removes the Server's in-memory state and any legacy temporary
// directory used for plugin downloads.
func (s *Server) Cleanup() {
	s.mu.Lock()
	tmpDir := s.tmpDir
	clear(s.dlc)
	clear(s.sc)
	clear(s.versionsc)
	s.tmpDir = ""
	s.mu.Unlock()

	s.l.Info("Cleaning up temporary directory", "dir", tmpDir)
	os.RemoveAll(tmpDir)
}

// validateProviderFileName ensures the filename reported by the registry is a
// safe, simple basename before it is joined with s.tmpDir. Rejects empty
// names, anything containing a path separator (forward or back slash), any
// "." / ".." traversal component, NUL bytes, or absolute paths. Keeps the
// checks conservative — registry filenames in practice are of the form
// "terraform-provider-<name>_<version>_<os>_<arch>.zip".
func validateProviderFileName(name string) error {
	if name == "" {
		return fmt.Errorf("filename must not be empty")
	}
	if strings.ContainsAny(name, "/\\\x00") {
		return fmt.Errorf("filename %q must not contain path separators or NUL bytes", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("filename %q is not a valid basename", name)
	}
	if filepath.IsAbs(name) || filepath.Base(name) != name {
		return fmt.Errorf("filename %q must be a simple basename", name)
	}
	return nil
}

// validateCachePathComponent validates a single Request field used both as a
// URL path segment and as an on-disk cache path segment. When required is
// true an empty value is rejected; otherwise an empty value is allowed (used
// for Version, which may be empty to mean "latest").
//
// Values must only contain characters from a conservative URL-safe set:
// ASCII letters, digits, and the unreserved punctuation "-", "_", ".", "+",
// "~". This avoids having to URL-escape segments when constructing registry
// URLs via Request.String(), and rejects characters (like "?", "#", "%",
// "/", or whitespace) that would change URL semantics or escape the cache
// root on disk.
func validateCachePathComponent(name, value string, required bool) error {
	if value == "" {
		if required {
			return fmt.Errorf("%s must not be empty", name)
		}
		return nil
	}

	if filepath.IsAbs(value) {
		return fmt.Errorf("%s must not be an absolute path", name)
	}

	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.' || r == '+' || r == '~':
		default:
			return fmt.Errorf("%s contains invalid character %q (allowed: letters, digits, '-', '_', '.', '+', '~')", name, r)
		}
	}

	cleaned := filepath.Clean(value)
	if cleaned != value || cleaned == "." || cleaned == ".." {
		return fmt.Errorf("%s contains invalid path content", name)
	}

	return nil
}

// validateCacheRequestIdentity validates the request fields that identify the
// provider (namespace, name). Version is not validated here — it may still be
// a constraint like "~>2.1" at this point and is validated once resolved by
// fixVersion.
func (s *Server) validateCacheRequestIdentity(request Request) error {
	if err := validateCachePathComponent("namespace", request.Namespace, true); err != nil {
		return err
	}
	if err := validateCachePathComponent("name", request.Name, true); err != nil {
		return err
	}
	return nil
}

// validateCacheRequestVersion validates that a concrete, resolved provider
// version is safe to use as a URL path and on-disk cache segment. It must be
// called after fixVersion, never on a user-supplied constraint.
func (s *Server) validateCacheRequestVersion(request Request) error {
	return validateCachePathComponent("version", request.Version, true)
}

// Get retrieves the plugin for the specified request, downloading it if necessary.
// The GetXxx methods (GetResourceSchema, GetDataSourceSchema, etc.) will call this method anyway,
// so it is not necessary to call Get directly unless you want to ensure the plugin is downloaded first.
//
// Providers are extracted into a predictable on-disk cache (see CacheDir and
// the TFPLUGINSCHEMA_CACHE_DIR environment variable). Subsequent calls for the
// same provider/version/os/arch are served from the cache. Pass
// WithForceFetch(true) to NewServer to bypass the cache and always download.
// Cleanup() removes only the Server's in-memory state and any legacy temp
// directory; the persistent cache is preserved across runs.
func (s *Server) Get(request Request) error {
	if err := s.validateCacheRequestIdentity(request); err != nil {
		return fmt.Errorf("invalid provider request: %w", err)
	}

	var notifyRequest Request
	var notifyStatus CacheStatus
	var shouldNotify bool

	if !request.fixedVersion() {
		var err error
		request, err = request.fixVersion(s)
		if err != nil {
			return err
		}
	}

	// The (possibly resolved) version is now used for URL/cache-path
	// construction, so it must be URL/path safe.
	if err := s.validateCacheRequestVersion(request); err != nil {
		return fmt.Errorf("invalid provider request: %w", err)
	}

	// Build the request-scoped logger *after* fixVersion, so that logs
	// carry the concrete resolved version rather than the caller-supplied
	// constraint (e.g. "~>2.1").
	l := s.l.With("request_namespace", request.Namespace, "request_name", request.Name, "request_version", request.Version)

	s.mu.RLock()
	if _, exists := s.dlc[request]; exists {
		l.Info("Request already exists in download cache")
		s.mu.RUnlock()
		return nil // Request already exists, no need to add again
	}
	s.mu.RUnlock()

	// Lock for the download and extraction process to avoid multiple downloads of the same plugin.
	// The cache-status callback reference is captured under the lock and
	// invoked *after* the lock is released, so user callbacks may safely
	// call back into the Server without deadlocking.
	s.mu.Lock()
	var notifyFn CacheStatusFunc
	defer func() {
		s.mu.Unlock()
		if shouldNotify && notifyFn != nil {
			s.notifyCacheStatusWith(notifyFn, notifyRequest, notifyStatus)
		}
	}()

	// Re-check after acquiring the write lock: another goroutine may have
	// populated the cache between the RUnlock above and Lock here.
	if _, exists := s.dlc[request]; exists {
		l.Info("Request already exists in download cache")
		return nil
	}

	// Check the persistent on-disk cache first (unless force-fetch is set).
	extractDir := cacheProviderDir(s.cacheDir, request)
	if err := ensureWithinBaseDir(s.cacheDir, extractDir); err != nil {
		return err
	}

	if !s.forceFetch {
		if path, ok := findProviderBinary(extractDir, request.Name); ok {
			l.Info("Provider cache hit", "path", path, "cache_dir", s.cacheDir)
			s.dlc[request] = path
			notifyRequest, notifyStatus, shouldNotify = request, CacheStatusHit, true
			notifyFn = s.cacheStatusFn
			return nil
		}
	}

	l.Info("Provider cache miss, downloading", "cache_dir", s.cacheDir, "force_fetch", s.forceFetch)
	notifyRequest, notifyStatus, shouldNotify = request, CacheStatusMiss, true
	notifyFn = s.cacheStatusFn

	registryApiRequest, err := http.NewRequest(http.MethodGet, request.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for registry API: %w", err)
	}
	l.Debug("Sending request to registry API", "url", registryApiRequest.URL.String())

	resp, err := s.httpClient.Do(registryApiRequest)
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

	// Sanitize the filename reported by the registry before using it as a
	// local filesystem path component. It must be a simple base name with no
	// path separators or traversal; anything else is rejected to avoid
	// writing outside s.tmpDir if the registry response is malicious or
	// corrupted.
	if err := validateProviderFileName(pluginResponse.FileName); err != nil {
		return fmt.Errorf("invalid plugin filename from registry: %w", err)
	}

	downloadURL := pluginResponse.DownloadURL
	if downloadURL == "" {
		return fmt.Errorf("download URL is empty for request: %s", request.String())
	}

	// Create a temp directory for the download so that partial downloads do not
	// corrupt the persistent cache.
	if s.tmpDir == "" {
		tmpFile, err := os.MkdirTemp("", "tfpluginschema-")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory: %w", err)
		}
		s.tmpDir = tmpFile
	}

	downloadRequest, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for plugin download: %w", err)
	}

	resp, err = s.httpClient.Do(downloadRequest)
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

	// Extract into the persistent cache directory. If it already exists (e.g.
	// from a prior partial run or force-fetch), clear it first so extraction
	// starts from a known-clean state.
	if err := os.RemoveAll(extractDir); err != nil {
		return fmt.Errorf("failed to clear cache directory %s: %w", extractDir, err)
	}
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory %s: %w", extractDir, err)
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

// notifyCacheStatusWith invokes the provided cache-status callback. The
// callback reference must be captured under the Server lock and this helper
// must be called *after* releasing the lock, so user callbacks may safely
// call back into the Server without deadlocking. Panics from user callbacks
// are recovered so they do not break the Server.
func (s *Server) notifyCacheStatusWith(fn CacheStatusFunc, request Request, status CacheStatus) {
	if fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			s.l.Error("cache status callback panicked", "panic", r)
		}
	}()
	fn(request, status)
}

// GetResourceSchema retrieves the schema for a specific resource from the provider.
func (s *Server) GetResourceSchema(request Request, resource string) (*tfjson.Schema, error) {
	s.l.Info("Getting resource schema", "request", request, "resource", resource)

	schemaResp, err := s.readSchema(request)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider schema: %w", err)
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

	schemaResp, err := s.readSchema(request)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider schema: %w", err)
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

	schemaResp, err := s.readSchema(request)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider schema: %w", err)
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

	schemaResp, err := s.readSchema(request)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider schema: %w", err)
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

	schemaResp, err := s.readSchema(request)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider schema: %w", err)
	}

	return schemaResp.ConfigSchema, nil
}

// ListResources retrieves the list of resource names from the provider.
func (s *Server) ListResources(request Request) ([]string, error) {
	s.l.Info("Listing resources", "request", request)

	schemaResp, err := s.readSchema(request)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider schema: %w", err)
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

	schemaResp, err := s.readSchema(request)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider schema: %w", err)
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

	schemaResp, err := s.readSchema(request)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider schema: %w", err)
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

	schemaResp, err := s.readSchema(request)
	if err != nil {
		return nil, fmt.Errorf("failed to read provider schema: %w", err)
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

	if !request.fixedVersion() {
		return nil, fmt.Errorf("version must be fixed before getting schema")
	}

	s.mu.RLock()
	if resp, exists := s.sc[request]; exists {
		s.mu.RUnlock()
		return resp, nil
	}
	s.mu.RUnlock()

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

	// Sanitize nil values to avoid nil dereference errors later
	// (these should ideally never be nil, but just in case).
	if providerSchema.DataSourceSchemas == nil {
		providerSchema.DataSourceSchemas = make(map[string]*tfjson.Schema)
	}

	if providerSchema.ResourceSchemas == nil {
		providerSchema.ResourceSchemas = make(map[string]*tfjson.Schema)
	}

	if providerSchema.EphemeralResourceSchemas == nil {
		providerSchema.EphemeralResourceSchemas = make(map[string]*tfjson.Schema)
	}

	if providerSchema.Functions == nil {
		providerSchema.Functions = make(map[string]*tfjson.FunctionSignature)
	}

	if providerSchema.ConfigSchema == nil {
		providerSchema.ConfigSchema = &tfjson.Schema{}
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
