package tfpluginschema

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
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
