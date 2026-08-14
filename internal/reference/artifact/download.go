// Package artifact resolves, verifies, and caches Minecraft artifacts.
package artifact

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxArtifactSize = int64(2 << 30)

// DownloadSpec describes one remote artifact and its expected integrity data.
type DownloadSpec struct {
	URL    string
	Size   int64
	SHA1   string
	SHA256 string
}

// DownloadResult records the verified local artifact state.
type DownloadResult struct {
	Path   string `json:"path"`
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	SHA1   string `json:"sha1,omitempty"`
	SHA256 string `json:"sha256"`
	Cached bool   `json:"cached"`
}

// Downloader downloads and atomically caches verified artifacts.
type Downloader struct {
	Client *http.Client
}

// Download returns a verified cache entry, downloading it when necessary.
func (d Downloader) Download(ctx context.Context, spec DownloadSpec, destination string) (DownloadResult, error) {
	if err := validateSpec(spec); err != nil {
		return DownloadResult{}, err
	}
	if result, err := verifyFile(destination, spec); err == nil {
		result.URL = spec.URL
		result.Cached = true
		return result, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		if removeErr := os.Remove(destination); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return DownloadResult{}, fmt.Errorf("remove invalid cache entry %s: %w", destination, removeErr)
		}
	}

	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return DownloadResult{}, fmt.Errorf("create download directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".download-*")
	if err != nil {
		return DownloadResult{}, fmt.Errorf("create temporary download: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, spec.URL, nil)
	if err != nil {
		_ = temporary.Close()
		return DownloadResult{}, fmt.Errorf("create download request: %w", err)
	}
	client := d.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		_ = temporary.Close()
		return DownloadResult{}, fmt.Errorf("download %s: %w", spec.URL, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		_ = temporary.Close()
		return DownloadResult{}, fmt.Errorf("download %s: unexpected HTTP status %s", spec.URL, response.Status)
	}

	written, err := io.Copy(temporary, io.LimitReader(response.Body, maxArtifactSize+1))
	if err != nil {
		_ = temporary.Close()
		return DownloadResult{}, fmt.Errorf("write download %s: %w", spec.URL, err)
	}
	if written > maxArtifactSize {
		_ = temporary.Close()
		return DownloadResult{}, fmt.Errorf("download %s exceeds %d bytes", spec.URL, maxArtifactSize)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return DownloadResult{}, fmt.Errorf("sync download: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return DownloadResult{}, fmt.Errorf("close download: %w", err)
	}

	result, err := verifyFile(temporaryPath, spec)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("verify %s: %w", spec.URL, err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return DownloadResult{}, fmt.Errorf("publish download: %w", err)
	}
	result.Path = destination
	result.URL = spec.URL
	return result, nil
}

func validateSpec(spec DownloadSpec) error {
	if spec.URL == "" {
		return errors.New("download URL is required")
	}
	if spec.Size < 0 || spec.Size > maxArtifactSize {
		return fmt.Errorf("invalid expected size %d", spec.Size)
	}
	if spec.SHA1 == "" && spec.SHA256 == "" {
		return errors.New("at least one artifact digest is required")
	}
	return nil
}

func verifyFile(path string, spec DownloadSpec) (DownloadResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return DownloadResult{}, err
	}
	defer func() { _ = file.Close() }()

	sha1Hash := sha1.New() //nolint:gosec // Mojang metadata uses SHA-1 for integrity.
	sha256Hash := sha256.New()
	writers := []io.Writer{sha256Hash}
	if spec.SHA1 != "" {
		writers = append(writers, sha1Hash)
	}
	size, err := io.Copy(io.MultiWriter(writers...), file)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("hash %s: %w", path, err)
	}
	if spec.Size > 0 && size != spec.Size {
		return DownloadResult{}, fmt.Errorf("size mismatch: got %d, want %d", size, spec.Size)
	}
	sha1Value := digestString(sha1Hash)
	sha256Value := digestString(sha256Hash)
	if spec.SHA1 != "" && !strings.EqualFold(sha1Value, spec.SHA1) {
		return DownloadResult{}, fmt.Errorf("SHA-1 mismatch: got %s, want %s", sha1Value, spec.SHA1)
	}
	if spec.SHA256 != "" && !strings.EqualFold(sha256Value, spec.SHA256) {
		return DownloadResult{}, fmt.Errorf("SHA-256 mismatch: got %s, want %s", sha256Value, spec.SHA256)
	}
	return DownloadResult{Path: path, Size: size, SHA1: sha1Value, SHA256: sha256Value}, nil
}

func digestString(value hash.Hash) string {
	return hex.EncodeToString(value.Sum(nil))
}
