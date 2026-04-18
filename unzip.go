package tfpluginschema

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func unzip(source, destination string) error {
	r, err := zip.OpenReader(source)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if err := unzipFile(f, destination); err != nil {
			return fmt.Errorf("failed to extract file from zip: %w", err)
		}
	}

	return nil
}

func unzipFile(f *zip.File, destination string) error {
	// Zip Slip hardening: validate the entry name before joining it with the
	// destination directory. Reject absolute paths, drive/volume prefixes
	// (Windows), and any entry whose cleaned join escapes destination via
	// "../" segments. This must happen before any filesystem operation so a
	// malicious archive cannot create directories or files outside dest.
	name := f.Name
	if name == "" {
		return fmt.Errorf("invalid zip entry: empty name")
	}
	if filepath.IsAbs(name) || filepath.IsAbs(filepath.FromSlash(name)) {
		return fmt.Errorf("invalid zip entry %q: absolute path not allowed", name)
	}
	if vol := filepath.VolumeName(filepath.FromSlash(name)); vol != "" {
		return fmt.Errorf("invalid zip entry %q: volume/drive prefix not allowed", name)
	}
	cleanDest := filepath.Clean(destination)
	path := filepath.Join(cleanDest, filepath.FromSlash(name))
	rel, err := filepath.Rel(cleanDest, path)
	if err != nil {
		return fmt.Errorf("invalid zip entry %q: %w", name, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid zip entry %q: escapes destination directory", name)
	}

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("failed to open file in zip: %w", err)
	}
	defer rc.Close()

	if f.FileInfo().IsDir() {
		// Use a sane default permission for directories
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		return nil
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}
	// Ensure parent dir is usable even if earlier directory entry had 000 perms
	if fi, err := os.Stat(parentDir); err == nil {
		if fi.Mode().Perm()&0o700 == 0 { // no owner perms
			_ = os.Chmod(parentDir, 0o755)
		}
	}

	fperm := f.Mode().Perm()
	if fperm == 0 {
		fperm = 0o644
	}
	outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fperm)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, rc); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
