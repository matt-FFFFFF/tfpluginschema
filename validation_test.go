package tfpluginschema

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCachePathComponent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		field    string
		value    string
		required bool
		wantErr  bool
	}{
		{"empty required", "namespace", "", true, true},
		{"empty optional", "version", "", false, false},
		{"simple ok", "name", "aws", true, false},
		{"digits ok", "version", "1.2.3", false, false},
		{"allowed punct", "namespace", "foo_bar-baz.+~", true, false},
		{"slash rejected", "namespace", "foo/bar", true, true},
		{"backslash rejected", "namespace", "foo\\bar", true, true},
		{"dotdot rejected", "namespace", "..", true, true},
		{"percent rejected", "namespace", "foo%20", true, true},
		{"query rejected", "namespace", "a?b", true, true},
		{"hash rejected", "namespace", "a#b", true, true},
		{"space rejected", "namespace", "a b", true, true},
		{"tab rejected", "namespace", "a\tb", true, true},
		{"nul rejected", "namespace", "a\x00b", true, true},
		{"absolute rejected", "namespace", "/etc", true, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateCachePathComponent(tc.field, tc.value, tc.required)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateProviderFileName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"ok zip", "terraform-provider-aws_5.0.0_linux_amd64.zip", false},
		{"empty", "", true},
		{"dot", ".", true},
		{"dotdot", "..", true},
		{"slash", "dir/file.zip", true},
		{"backslash", "dir\\file.zip", true},
		{"parent traversal", "../evil.zip", true},
		{"nul", "evil\x00.zip", true},
		{"absolute unix", "/tmp/evil.zip", true},
	}
	if runtime.GOOS == "windows" {
		cases = append(cases,
			struct {
				name    string
				value   string
				wantErr bool
			}{"absolute windows drive", `C:\evil.zip`, true},
			struct {
				name    string
				value   string
				wantErr bool
			}{"unc path", `\\server\share\evil.zip`, true},
		)
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateProviderFileName(tc.value)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEnsureWithinBaseDir(t *testing.T) {
	t.Parallel()

	t.Run("contained target", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		target := filepath.Join(base, "opentofu", "hashicorp", "aws", "1.0.0")
		require.NoError(t, ensureWithinBaseDir(base, target))
	})

	t.Run("lexical parent escape", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		target := filepath.Join(base, "..", "evil")
		assert.Error(t, ensureWithinBaseDir(base, target))
	})

	t.Run("absolute target outside base", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		other := t.TempDir()
		assert.Error(t, ensureWithinBaseDir(base, filepath.Join(other, "x")))
	})

	t.Run("base created when missing", func(t *testing.T) {
		t.Parallel()
		parent := t.TempDir()
		base := filepath.Join(parent, "does-not-exist-yet")
		target := filepath.Join(base, "sub", "leaf")
		require.NoError(t, ensureWithinBaseDir(base, target))
		// Base should have been materialized to close the TOCTOU window.
		info, err := os.Stat(base)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("symlink ancestor escape rejected", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink creation may require elevated permissions on Windows")
		}
		t.Parallel()
		base := t.TempDir()
		outside := t.TempDir()
		// Replace <base>/opentofu with a symlink pointing outside of base.
		linkName := filepath.Join(base, "opentofu")
		require.NoError(t, os.Symlink(outside, linkName))

		target := filepath.Join(linkName, "hashicorp", "aws", "1.0.0")
		err := ensureWithinBaseDir(base, target)
		assert.Error(t, err, "symlink escape must be detected")
	})
}
