package tfpluginschema

import (
"archive/zip"
"fmt"
"io"
"os"
"path/filepath"
"strings"
)

// unzip extracts a Terraform provider zip archive into destination.
// Terraform provider archives are flat: every entry is a regular file
// in the archive root (e.g. "terraform-provider-foo_v1.2.3", "LICENSE").
// Directory entries, nested paths, symlinks, or any other non-regular
// entries are rejected. Entries are written via a temp file in destination
// and then atomically renamed into place, so a pre-existing (or raced-in)
// symlink at the target path is replaced rather than followed.
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
name := f.Name
if name == "" {
return fmt.Errorf("invalid zip entry: empty name")
}

// Directory entries are not expected in provider zips; reject them.
if f.FileInfo().IsDir() || strings.HasSuffix(name, "/") {
return fmt.Errorf("invalid zip entry %q: directory entries not allowed (provider zip must be flat)", name)
}
// Only regular files are allowed — reject symlinks, devices, etc.
if !f.Mode().IsRegular() {
return fmt.Errorf("invalid zip entry %q: non-regular file not allowed", name)
}
// The entry name must be a simple base name in the archive root: no
// path separators, no volume/drive prefix, no traversal, no absolute
// paths.
if strings.ContainsAny(name, `/\`) {
return fmt.Errorf("invalid zip entry %q: path separators not allowed (provider zip must be flat)", name)
}
if strings.ContainsRune(name, 0) {
return fmt.Errorf("invalid zip entry %q: NUL byte not allowed", name)
}
if name == "." || name == ".." {
return fmt.Errorf("invalid zip entry %q: reserved name not allowed", name)
}
if filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
return fmt.Errorf("invalid zip entry %q: absolute path or volume prefix not allowed", name)
}
// Defence in depth: filepath.Clean / filepath.Base must be a no-op for
// a simple base name.
if filepath.Base(name) != name || filepath.Clean(name) != name {
return fmt.Errorf("invalid zip entry %q: must be a simple base name", name)
}

rc, err := f.Open()
if err != nil {
return fmt.Errorf("failed to open file in zip: %w", err)
}
defer rc.Close()

fperm := f.Mode().Perm()
if fperm == 0 {
fperm = 0o644
}

// Write to a temp file in destination and atomically rename into place.
// os.Rename replaces any existing entry at the final path (including a
// symlink) rather than following it, closing the leaf-path TOCTOU that
// a direct os.OpenFile(finalPath) would otherwise hit.
tmp, err := os.CreateTemp(destination, "unzip-*.partial")
if err != nil {
return fmt.Errorf("failed to create temp file for entry %q: %w", name, err)
}
tmpPath := tmp.Name()
cleanupTmp := true
defer func() {
if cleanupTmp {
_ = os.Remove(tmpPath)
}
}()

if _, err := io.Copy(tmp, rc); err != nil {
_ = tmp.Close()
return fmt.Errorf("failed to write entry %q: %w", name, err)
}
if err := tmp.Chmod(fperm); err != nil {
_ = tmp.Close()
return fmt.Errorf("failed to chmod temp file for entry %q: %w", name, err)
}
if err := tmp.Close(); err != nil {
return fmt.Errorf("failed to close temp file for entry %q: %w", name, err)
}

finalPath := filepath.Join(destination, name)
if err := os.Rename(tmpPath, finalPath); err != nil {
return fmt.Errorf("failed to publish entry %q: %w", name, err)
}
cleanupTmp = false
return nil
}
