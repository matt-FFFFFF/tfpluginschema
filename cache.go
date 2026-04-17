package tfpluginschema

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// EnvCacheDir is the environment variable used to override the provider cache
// directory. When set (and non-empty), its value is used as the root of the
// provider cache instead of the default.
const EnvCacheDir = "TFPLUGINSCHEMA_CACHE_DIR"

// CacheStatus indicates whether a provider was served from the local cache or
// had to be downloaded from the registry.
type CacheStatus int

const (
	// CacheStatusMiss indicates the provider was not in the cache and was
	// downloaded from the registry.
	CacheStatusMiss CacheStatus = iota
	// CacheStatusHit indicates the provider was served from the local cache.
	CacheStatusHit
)

// String returns a human-readable form of the CacheStatus.
func (c CacheStatus) String() string {
	switch c {
	case CacheStatusHit:
		return "hit"
	case CacheStatusMiss:
		return "miss"
	default:
		return "unknown"
	}
}

// CacheStatusFunc is invoked by the Server after resolving a provider request
// to report whether the provider binary was served from the local cache
// (CacheStatusHit) or downloaded (CacheStatusMiss). The request passed in has
// a concrete (fixed) version.
type CacheStatusFunc func(request Request, status CacheStatus)

// ServerOption configures a Server at construction time.
type ServerOption func(*Server)

// WithCacheDir overrides the provider cache directory used by the Server.
// An empty dir is ignored and the default is used instead.
func WithCacheDir(dir string) ServerOption {
	return func(s *Server) {
		if dir != "" {
			s.cacheDir = dir
		}
	}
}

// WithForceFetch configures the Server to always re-download providers,
// bypassing any existing cache entries. Downloads still populate the cache.
func WithForceFetch(force bool) ServerOption {
	return func(s *Server) {
		s.forceFetch = force
	}
}

// WithCacheStatusFunc installs a callback invoked after the Server resolves a
// provider to indicate whether the cache was hit or the provider was
// downloaded. Useful for CLIs wishing to report download/cache activity.
func WithCacheStatusFunc(fn CacheStatusFunc) ServerOption {
	return func(s *Server) {
		s.cacheStatusFn = fn
	}
}

// defaultCacheDir returns the default provider cache directory, honoring the
// TFPLUGINSCHEMA_CACHE_DIR environment variable if set. Falls back to
// os.UserCacheDir()/tfpluginschema, then os.TempDir()/tfpluginschema-cache.
func defaultCacheDir() string {
	if v := os.Getenv(EnvCacheDir); v != "" {
		return v
	}
	if d, err := os.UserCacheDir(); err == nil && d != "" {
		return filepath.Join(d, "tfpluginschema")
	}
	return filepath.Join(os.TempDir(), "tfpluginschema-cache")
}

// cachePathSegment returns a filesystem-safe cache path segment.
// Empty values are mapped to "default" to keep the cache layout stable.
func cachePathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "default"
	}

	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_")
	return replacer.Replace(value)
}

// cacheProviderDir returns the predictable cache directory for a given
// provider request. The layout is:
//
//	<cacheDir>/<namespace>/terraform-provider-<name>/<version>/<os>_<arch>
//
// The request version must be a concrete version (not a constraint).
func cacheProviderDir(cacheDir string, request Request) string {
	return filepath.Join(
		cacheDir,
		cachePathSegment(request.Namespace),
		providerFileNamePrefix+request.Name,
		request.Version,
		runtime.GOOS+"_"+runtime.GOARCH,
	)
}

// findProviderBinary walks dir looking for a file whose name starts with
// "terraform-provider-<name>". It returns the absolute path to the first
// match, or ("", false) if none is found or the directory does not exist.
func findProviderBinary(dir, providerName string) (string, bool) {
	wantPrefix := providerFileNamePrefix + providerName
	if _, err := os.Stat(dir); err != nil {
		return "", false
	}
	var found string
	_ = fs.WalkDir(os.DirFS(dir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), wantPrefix) {
			found = filepath.Join(dir, path)
			return fs.SkipAll
		}
		return nil
	})
	if found == "" {
		return "", false
	}
	return found, true
}


