// Package archive provides constrained reads from Minecraft jar and zip files.
package archive

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractFile extracts one exact zip entry to destination atomically.
func ExtractFile(zipPath, name, destination string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer func() { _ = reader.Close() }()

	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		return writeZipFile(file, destination)
	}
	return fmt.Errorf("%w: %s in %s", os.ErrNotExist, name, zipPath)
}

// ReadFile reads one exact zip entry with a fixed size limit.
func ReadFile(zipPath, name string) ([]byte, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer func() { _ = reader.Close() }()
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", name, err)
		}
		defer func() { _ = stream.Close() }()
		data, err := io.ReadAll(io.LimitReader(stream, 64<<20))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		return data, nil
	}
	return nil, fmt.Errorf("%w: %s in %s", os.ErrNotExist, name, zipPath)
}

// ListClassPaths returns class entry paths without the .class suffix.
func ListClassPaths(jarPath string) ([]string, error) {
	reader, err := zip.OpenReader(jarPath)
	if err != nil {
		return nil, fmt.Errorf("open jar %s: %w", jarPath, err)
	}
	defer func() { _ = reader.Close() }()
	classes := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !strings.HasSuffix(file.Name, ".class") || strings.HasPrefix(file.Name, "META-INF/versions/") {
			continue
		}
		classes = append(classes, strings.TrimSuffix(file.Name, ".class"))
	}
	return classes, nil
}

// ListClasses returns JVM binary class names stored in a jar.
func ListClasses(jarPath string) ([]string, error) {
	paths, err := ListClassPaths(jarPath)
	if err != nil {
		return nil, err
	}
	classes := make([]string, 0, len(paths))
	for _, path := range paths {
		classes = append(classes, strings.ReplaceAll(path, "/", "."))
	}
	return classes, nil
}

// HasFile reports whether a zip contains an exact entry name.
func HasFile(zipPath, name string) (bool, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return false, fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer func() { _ = reader.Close() }()
	for _, file := range reader.File {
		if file.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func writeZipFile(file *zip.File, destination string) error {
	if file.FileInfo().IsDir() {
		return errors.New("zip entry is a directory")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return fmt.Errorf("create extraction directory: %w", err)
	}
	stream, err := file.Open()
	if err != nil {
		return fmt.Errorf("open zip entry %s: %w", file.Name, err)
	}
	defer func() { _ = stream.Close() }()
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".extract-*")
	if err != nil {
		return fmt.Errorf("create extraction file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if _, err := io.Copy(temporary, stream); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("extract %s: %w", file.Name, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close extraction file: %w", err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("publish extraction: %w", err)
	}
	return nil
}
