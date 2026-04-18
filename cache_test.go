package tfpluginschema

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFakeProviderBinary creates a predictable cache directory layout
// populated with a fake provider binary. It returns the absolute path to the
// fake binary.
func writeFakeProviderBinary(t *testing.T, cacheRoot string, request Request) string {
	t.Helper()
	dir := cacheProviderDir(cacheRoot, request)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	bin := filepath.Join(dir, providerFileNamePrefix+request.Name+"_v"+request.Version)
	require.NoError(t, os.WriteFile(bin, []byte("fake"), 0o644))
	return bin
}

func TestDefaultCacheDir_EnvOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "tfps-cache")
	t.Setenv(EnvCacheDir, want)

	got := defaultCacheDir()
	assert.Equal(t, want, got)
}

func TestDefaultCacheDir_FallbackToUserCacheDir(t *testing.T) {
	t.Setenv(EnvCacheDir, "")

	got := defaultCacheDir()
	require.NotEmpty(t, got)
	// Accept the user cache dir path and the temp-dir fallback path.
	assert.Contains(
		t,
		[]string{"tfpluginschema", "tfpluginschema-cache"},
		filepath.Base(got),
		"default cache dir should end with either 'tfpluginschema' or 'tfpluginschema-cache', got %q",
		got,
	)
}

func TestWithCacheDir_OverridesDefault(t *testing.T) {
	t.Setenv(EnvCacheDir, "")
	custom := t.TempDir()
	s := NewServer(nil, WithCacheDir(custom))
	assert.Equal(t, custom, s.CacheDir())
}

func TestWithCacheDir_EmptyIsIgnored(t *testing.T) {
	dflt := t.TempDir()
	t.Setenv(EnvCacheDir, dflt)
	s := NewServer(nil, WithCacheDir(""))
	assert.Equal(t, dflt, s.CacheDir())
}

func TestCacheProviderDir_Layout(t *testing.T) {
	root := "/tmp/root"
	req := Request{
		Namespace:    "hashicorp",
		Name:         "aws",
		Version:      "1.2.3",
		RegistryType: RegistryTypeOpenTofu,
	}
	want := filepath.Join(
		root,
		string(RegistryTypeOpenTofu),
		"hashicorp",
		"terraform-provider-aws",
		"1.2.3",
		runtime.GOOS+"_"+runtime.GOARCH,
	)
	assert.Equal(t, want, cacheProviderDir(root, req))
}

func TestCacheProviderDir_LayoutDefaultsForEmptyFields(t *testing.T) {
	root := "/tmp/root"
	req := Request{Name: "aws", Version: "1.2.3"}
	want := filepath.Join(
		root,
		"opentofu", // empty RegistryType is normalized to OpenTofu
		"default",  // empty Namespace
		"terraform-provider-aws",
		"1.2.3",
		runtime.GOOS+"_"+runtime.GOARCH,
	)
	assert.Equal(t, want, cacheProviderDir(root, req))
}

func TestCacheProviderDir_IsolatesNamespacesAndRegistries(t *testing.T) {
	root := "/tmp/root"
	reqA := Request{Namespace: "a", Name: "p", Version: "1", RegistryType: RegistryTypeOpenTofu}
	reqB := Request{Namespace: "b", Name: "p", Version: "1", RegistryType: RegistryTypeOpenTofu}
	reqC := Request{Namespace: "a", Name: "p", Version: "1", RegistryType: RegistryTypeTerraform}

	assert.NotEqual(t, cacheProviderDir(root, reqA), cacheProviderDir(root, reqB),
		"different namespaces must map to different cache dirs")
	assert.NotEqual(t, cacheProviderDir(root, reqA), cacheProviderDir(root, reqC),
		"different registry types must map to different cache dirs")
}

func TestFindProviderBinary_FindsFile(t *testing.T) {
	dir := t.TempDir()
	req := Request{Name: "aws", Version: "1.0.0"}
	bin := filepath.Join(dir, providerFileNamePrefix+req.Name+"_v"+req.Version)
	require.NoError(t, os.WriteFile(bin, []byte("x"), 0o644))

	got, ok := findProviderBinary(dir, req.Name)
	assert.True(t, ok)
	assert.Equal(t, bin, got)
}

func TestFindProviderBinary_MissingDir(t *testing.T) {
	got, ok := findProviderBinary(filepath.Join(t.TempDir(), "does-not-exist"), "aws")
	assert.False(t, ok)
	assert.Equal(t, "", got)
}

func TestFindProviderBinary_NoMatch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("x"), 0o644))

	got, ok := findProviderBinary(dir, "aws")
	assert.False(t, ok)
	assert.Equal(t, "", got)
}

func TestServer_Get_CacheHit(t *testing.T) {
	cacheRoot := t.TempDir()
	req := Request{Namespace: "hashicorp", Name: "aws", Version: "1.2.3", RegistryType: RegistryTypeOpenTofu}
	bin := writeFakeProviderBinary(t, cacheRoot, req)

	var gotStatus CacheStatus
	var gotReq Request
	called := 0
	s := NewServer(nil,
		WithCacheDir(cacheRoot),
		WithCacheStatusFunc(func(r Request, st CacheStatus) {
			called++
			gotStatus = st
			gotReq = r
		}),
	)
	t.Cleanup(s.Cleanup)

	require.NoError(t, s.Get(req))
	assert.Equal(t, bin, s.dlc[req], "cache hit should populate download cache with cached path")
	assert.Equal(t, 1, called)
	assert.Equal(t, CacheStatusHit, gotStatus)
	assert.Equal(t, req, gotReq)
}

func TestServer_Get_CacheHitSkippedByForceFetch(t *testing.T) {
	cacheRoot := t.TempDir()
	req := Request{
		Namespace: "does-not-exist-tfpluginschema",
		Name:      "does-not-exist-tfpluginschema",
		Version:   "1.2.3",
	}
	writeFakeProviderBinary(t, cacheRoot, req)

	// Route all HTTP traffic to a local test server that always 404s. This
	// keeps the test deterministic and offline — force-fetch bypasses the
	// cache, we observe a Miss callback, and the stubbed registry fails the
	// download attempt without any real network I/O.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	t.Cleanup(ts.Close)

	tsURL, err := url.Parse(ts.URL)
	require.NoError(t, err)
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &rewriteHostTransport{
			host:    tsURL.Host,
			scheme:  tsURL.Scheme,
			wrapped: http.DefaultTransport,
		},
	}

	called := 0
	var gotStatus CacheStatus
	s := NewServer(nil,
		WithCacheDir(cacheRoot),
		WithForceFetch(true),
		WithHTTPClient(client),
		WithCacheStatusFunc(func(_ Request, st CacheStatus) {
			called++
			gotStatus = st
		}),
	)
	t.Cleanup(s.Cleanup)

	err = s.Get(req)
	assert.Error(t, err, "force-fetch should attempt a download; stubbed 404 must surface as an error")
	assert.Equal(t, 1, called, "cache status callback should fire once")
	assert.Equal(t, CacheStatusMiss, gotStatus, "force-fetch must report a miss, not a hit")
}

func TestServer_CacheStatusFunc_PanicIsContained(t *testing.T) {
	cacheRoot := t.TempDir()
	req := Request{Namespace: "hashicorp", Name: "aws", Version: "1.2.3"}
	writeFakeProviderBinary(t, cacheRoot, req)

	s := NewServer(nil,
		WithCacheDir(cacheRoot),
		WithCacheStatusFunc(func(_ Request, _ CacheStatus) {
			panic("boom")
		}),
	)
	t.Cleanup(s.Cleanup)

	// Panic in the callback must not propagate from Get.
	assert.NotPanics(t, func() {
		require.NoError(t, s.Get(req))
	})
}

func TestCacheStatus_String(t *testing.T) {
	assert.Equal(t, "hit", CacheStatusHit.String())
	assert.Equal(t, "miss", CacheStatusMiss.String())
	assert.Equal(t, "unknown", CacheStatus(99).String())
}

func TestNewServer_EnvCacheDirUsedByDefault(t *testing.T) {
	custom := t.TempDir()
	t.Setenv(EnvCacheDir, custom)

	s := NewServer(nil)
	assert.Equal(t, custom, s.CacheDir())
}

// rewriteHostTransport redirects all outgoing requests to a fixed host/scheme,
// allowing tests to stub the registry without real network calls.
type rewriteHostTransport struct {
	host    string
	scheme  string
	wrapped http.RoundTripper
}

func (t *rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Host = t.host
	clone.URL.Scheme = t.scheme
	clone.Host = ""
	return t.wrapped.RoundTrip(clone)
}
