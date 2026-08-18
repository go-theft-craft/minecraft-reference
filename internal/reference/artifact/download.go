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
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	maxArtifactSize = int64(2 << 30)
	maxRedirects    = 10

	// Artifact hosts throttle the compatibility matrix, which fetches the same
	// pinned tools from every job at once. Every download is digest verified,
	// so a repeated attempt cannot widen what a caller ends up trusting.
	maxDownloadAttempts = 5
	initialRetryDelay   = time.Second
	maxRetryDelay       = 30 * time.Second
)

var mojangHosts = map[string]struct{}{
	"piston-data.mojang.com": {},
	"piston-meta.mojang.com": {},
	"launcher.mojang.com":    {},
}

var configuredToolHosts = map[string]struct{}{
	"mcp.zeith.org":             {},
	"raw.githubusercontent.com": {},
	"repo.maven.apache.org":     {},
}

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

// Downloader downloads and atomically caches verified artifacts. It retries
// throttled and transient responses with exponential backoff.
type Downloader struct {
	Client *http.Client

	// Progress reports each retry, so a throttled host leaves a trace in the
	// log rather than showing up only as a slower run. A nil value stays quiet.
	Progress func(string)

	// sleep waits out a retry delay. Tests replace it to avoid real waiting.
	sleep func(context.Context, time.Duration) error
}

// retryableError marks a download failure that a later attempt may survive.
// after carries the delay the server asked for, and is zero when it named none.
type retryableError struct {
	after time.Duration
	err   error
}

func (e retryableError) Error() string { return e.err.Error() }

func (e retryableError) Unwrap() error { return e.err }

// policyError marks a request the downloader itself refused, such as a
// redirect leaving the trusted hosts. The host never gets a say, so no later
// attempt can turn the refusal into a download.
type policyError struct {
	err error
}

func (e policyError) Error() string { return e.err.Error() }

func (e policyError) Unwrap() error { return e.err }

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

	if err := d.fetch(ctx, spec, temporary); err != nil {
		_ = temporary.Close()
		return DownloadResult{}, err
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

// fetch writes the artifact into temporary, retrying throttled and transient
// responses until one succeeds or the attempts run out. Each attempt rewinds
// temporary, so a body that fails midway leaves nothing behind for the next.
func (d Downloader) fetch(ctx context.Context, spec DownloadSpec, temporary *os.File) error {
	client := d.client(spec)
	delay := initialRetryDelay
	for attempt := 1; ; attempt++ {
		err := fetchOnce(ctx, client, spec, temporary)
		if err == nil {
			return nil
		}
		var retryable retryableError
		if !errors.As(err, &retryable) {
			return err
		}
		if attempt == maxDownloadAttempts {
			return fmt.Errorf("after %d attempts: %w", attempt, err)
		}
		pause := min(delay, maxRetryDelay)
		if retryable.after > 0 {
			pause = min(retryable.after, maxRetryDelay)
		}
		d.report(fmt.Sprintf("retry %d of %d in %s: %v", attempt+1, maxDownloadAttempts, pause, err))
		if err := d.wait(ctx, pause); err != nil {
			return fmt.Errorf("download %s: %w", spec.URL, err)
		}
		delay = min(delay*2, maxRetryDelay)
	}
}

func fetchOnce(ctx context.Context, client *http.Client, spec DownloadSpec, temporary *os.File) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, spec.URL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return transientError(ctx, fmt.Errorf("download %s: %w", spec.URL, err))
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		failure := fmt.Errorf("download %s: unexpected HTTP status %s", spec.URL, response.Status)
		if retryableStatus(response.StatusCode) {
			return retryableError{after: retryAfter(response), err: failure}
		}
		return failure
	}

	if err := temporary.Truncate(0); err != nil {
		return fmt.Errorf("reset temporary download: %w", err)
	}
	if _, err := temporary.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("reset temporary download: %w", err)
	}
	written, err := io.Copy(temporary, io.LimitReader(response.Body, maxArtifactSize+1))
	if err != nil {
		return transientError(ctx, fmt.Errorf("write download %s: %w", spec.URL, err))
	}
	if written > maxArtifactSize {
		return fmt.Errorf("download %s exceeds %d bytes", spec.URL, maxArtifactSize)
	}
	return nil
}

// transientError marks a connection-level failure as worth another attempt,
// unless the caller's context is what ended it or the downloader is the one
// that refused the request.
func transientError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return err
	}
	var policy policyError
	if errors.As(err, &policy) {
		return err
	}
	return retryableError{err: err}
}

// retryableStatus reports whether a status is a host asking to be tried later
// rather than a verdict on the request itself.
func retryableStatus(status int) bool {
	if status == http.StatusTooManyRequests || status == http.StatusRequestTimeout {
		return true
	}
	return status >= http.StatusInternalServerError && status != http.StatusNotImplemented
}

// retryAfter reads the delay a host asked for, in either of the forms RFC 9110
// allows. An absent, malformed, or past value reads as no request at all.
func retryAfter(response *http.Response) time.Duration {
	value := strings.TrimSpace(response.Header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		if delay := time.Until(date); delay > 0 {
			return delay
		}
	}
	return 0
}

func (d Downloader) report(message string) {
	if d.Progress != nil {
		d.Progress(message)
	}
}

func (d Downloader) wait(ctx context.Context, delay time.Duration) error {
	if d.sleep != nil {
		return d.sleep(ctx, delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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
	parsed, err := url.Parse(spec.URL)
	if err != nil {
		return fmt.Errorf("parse download URL: %w", err)
	}
	return validateDownloadURL(spec, parsed)
}

func validateDownloadURL(spec DownloadSpec, location *url.URL) error {
	hostname, err := trustedHTTPSHostname(location)
	if err != nil {
		return err
	}
	if _, ok := mojangHosts[hostname]; ok || hostname == "libraries.minecraft.net" {
		if spec.SHA1 == "" {
			return fmt.Errorf("mojang artifact %q requires SHA-1", location.String())
		}
		return nil
	}
	if _, ok := configuredToolHosts[hostname]; ok {
		if spec.SHA256 == "" {
			return fmt.Errorf("configured tool %q requires SHA-256", location.String())
		}
		return nil
	}
	return fmt.Errorf("download URL host %q is not trusted", hostname)
}

func validateMetadataURL(location *url.URL) error {
	hostname, err := trustedHTTPSHostname(location)
	if err != nil {
		return err
	}
	if _, ok := mojangHosts[hostname]; !ok {
		return fmt.Errorf("metadata URL host %q is not trusted", hostname)
	}
	return nil
}

func trustedHTTPSHostname(location *url.URL) (string, error) {
	if location.Scheme != "https" {
		return "", fmt.Errorf("download URL must use HTTPS: %q", location.String())
	}
	if location.User != nil {
		return "", fmt.Errorf("download URL must not contain user credentials: %q", location.String())
	}
	if port := location.Port(); port != "" && port != "443" {
		return "", fmt.Errorf("download URL has an untrusted port: %q", location.String())
	}
	hostname := strings.ToLower(location.Hostname())
	if hostname == "" {
		return "", fmt.Errorf("download URL has no host: %q", location.String())
	}
	return hostname, nil
}

func (d Downloader) client(spec DownloadSpec) *http.Client {
	return clientWithURLPolicy(d.Client, func(location *url.URL) error {
		return validateDownloadURL(spec, location)
	})
}

func (r Resolver) metadataClient() *http.Client {
	return clientWithURLPolicy(r.Client, validateMetadataURL)
}

func clientWithURLPolicy(base *http.Client, validate func(*url.URL) error) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	client := *base
	previousCheckRedirect := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if err := validate(request.URL); err != nil {
			return policyError{fmt.Errorf("reject redirect target: %w", err)}
		}
		if len(via) >= maxRedirects {
			return policyError{fmt.Errorf("stopped after %d redirects", maxRedirects)}
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(request, via)
		}
		return nil
	}
	return &client
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
