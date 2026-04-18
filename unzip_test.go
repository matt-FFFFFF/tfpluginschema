package tfpluginschema

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	w := zip.NewWriter(f)
	for name, content := range entries {
		if content == "<DIR>" {
			_, err := w.Create(name + "/")
			require.NoError(t, err)
			continue
		}
		fw, err := w.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(fw, content)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
}

func TestUnzip_DirectoryAndFile(t *testing.T) {
	temp := t.TempDir()
	z := filepath.Join(temp, "test.zip")
	createZip(t, z, map[string]string{
		"dir":       "<DIR>",
		"dir/a.txt": "hello",
	})

	dst := filepath.Join(temp, "out")
	require.NoError(t, os.MkdirAll(dst, 0o755))

	err := unzip(z, dst)
	require.NoError(t, err)

	// directory exists
	fi, err := os.Stat(filepath.Join(dst, "dir"))
	require.NoError(t, err)
	assert.True(t, fi.IsDir())
	// file extracted with content
	data, err := os.ReadFile(filepath.Join(dst, "dir", "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestUnzipFile_CreateFileError(t *testing.T) {
	// create a zip with a file at the root
	temp := t.TempDir()
	z := filepath.Join(temp, "test.zip")
	createZip(t, z, map[string]string{"a.txt": "x"})

	// destination is a file (not a directory) to cause create errors on write
	dst := filepath.Join(temp, "out")
	require.NoError(t, os.WriteFile(dst, []byte("not a dir"), 0o644))

	err := unzip(z, dst)
	require.Error(t, err)
}

// createZipOrdered preserves entry order so traversal/absolute names are
// written exactly as supplied (a regression test for Zip Slip hardening).
func createZipOrdered(t *testing.T, path string, names []string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	w := zip.NewWriter(f)
	for _, name := range names {
		fw, err := w.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(fw, "x")
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
}

func TestUnzip_RejectsZipSlip_TraversalEntry(t *testing.T) {
	temp := t.TempDir()
	z := filepath.Join(temp, "evil.zip")
	createZipOrdered(t, z, []string{"../escaped.txt"})

	dst := filepath.Join(temp, "out")
	require.NoError(t, os.MkdirAll(dst, 0o755))

	err := unzip(z, dst)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes destination directory")

	// The traversal target must not have been created outside dst.
	_, statErr := os.Stat(filepath.Join(temp, "escaped.txt"))
	assert.True(t, os.IsNotExist(statErr), "traversal entry must not be written outside destination")
}

func TestUnzip_RejectsZipSlip_AbsoluteEntry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("absolute-path semantics differ on Windows; covered by traversal test")
	}
	temp := t.TempDir()
	z := filepath.Join(temp, "evil.zip")
	createZipOrdered(t, z, []string{"/abs.txt"})

	dst := filepath.Join(temp, "out")
	require.NoError(t, os.MkdirAll(dst, 0o755))

	err := unzip(z, dst)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absolute path not allowed")
}
