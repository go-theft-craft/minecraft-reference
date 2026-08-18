package artifact

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const trustedFixtureURL = "https://piston-data.mojang.com/test/artifact.bin"

func TestDownloadVerifiesAndReusesCache(t *testing.T) {
	payload := []byte("minecraft fixture")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = writer.Write(payload)
	}))
	defer server.Close()
	sha1Value := sha1.Sum(payload)
	destination := filepath.Join(t.TempDir(), "artifact.bin")
	spec := DownloadSpec{URL: trustedFixtureURL, Size: int64(len(payload)), SHA1: fmt.Sprintf("%x", sha1Value)}
	downloader := Downloader{Client: rewriteHostClient(t, server, "piston-data.mojang.com")}

	first, err := downloader.Download(context.Background(), spec, destination)
	if err != nil {
		t.Fatal(err)
	}
	second, err := downloader.Download(context.Background(), spec, destination)
	if err != nil {
		t.Fatal(err)
	}
	if first.Cached || !second.Cached || requests != 1 {
		t.Fatalf("first cached=%v, second cached=%v, requests=%d", first.Cached, second.Cached, requests)
	}
}

func TestDownloadRejectsDigestMismatchAndLeavesNoDestination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("wrong"))
	}))
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "artifact.bin")
	want := sha1.Sum([]byte("right"))
	_, err := (Downloader{Client: rewriteHostClient(t, server, "piston-data.mojang.com")}).Download(context.Background(), DownloadSpec{URL: trustedFixtureURL, SHA1: fmt.Sprintf("%x", want)}, destination)
	if err == nil {
		t.Fatal("expected digest mismatch")
	}
	if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("destination exists after failure: %v", statErr)
	}
}

func TestDownloadReplacesCorruptCache(t *testing.T) {
	payload := []byte("valid")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(payload)
	}))
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "artifact.bin")
	if err := os.WriteFile(destination, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha1.Sum(payload)
	if _, err := (Downloader{Client: rewriteHostClient(t, server, "piston-data.mojang.com")}).Download(context.Background(), DownloadSpec{URL: trustedFixtureURL, Size: int64(len(payload)), SHA1: fmt.Sprintf("%x", digest)}, destination); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadAllowsTrustedHostsWithRequiredDigest(t *testing.T) {
	payload := []byte("minecraft fixture")
	sha1Value := sha1.Sum(payload)
	sha256Value := sha256.Sum256(payload)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	tests := map[string]DownloadSpec{
		"Launcher metadata":   {URL: "https://launcher.mojang.com/mc/game/version_manifest_v2.json", SHA1: fmt.Sprintf("%x", sha1Value)},
		"Mojang metadata":     {URL: "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json", SHA1: fmt.Sprintf("%x", sha1Value)},
		"Mojang data":         {URL: trustedFixtureURL, SHA1: fmt.Sprintf("%x", sha1Value)},
		"Minecraft libraries": {URL: "https://libraries.minecraft.net/com/example/library.jar", SHA1: fmt.Sprintf("%x", sha1Value)},
		"Maven tool":          {URL: "https://repo.maven.apache.org/maven2/example/tool.jar", SHA256: fmt.Sprintf("%x", sha256Value)},
		"MCP tool":            {URL: "https://mcp.zeith.org/mcp/example.zip", SHA256: fmt.Sprintf("%x", sha256Value)},
		"GitHub tool":         {URL: "https://raw.githubusercontent.com/example/repo/main/tool.zip", SHA256: fmt.Sprintf("%x", sha256Value)},
	}
	for name, spec := range tests {
		t.Run(name, func(t *testing.T) {
			parsed, err := url.Parse(spec.URL)
			if err != nil {
				t.Fatal(err)
			}
			_, err = (Downloader{Client: rewriteHostClient(t, server, parsed.Hostname())}).Download(context.Background(), spec, filepath.Join(t.TempDir(), "artifact.bin"))
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDownloadRejectsUntrustedHostBeforeRequest(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, fmt.Errorf("unexpected request")
	})}
	sha256Value := sha256.Sum256([]byte("fixture"))
	_, err := (Downloader{Client: client}).Download(context.Background(), DownloadSpec{
		URL:    "https://example.invalid/file",
		SHA256: fmt.Sprintf("%x", sha256Value),
	}, filepath.Join(t.TempDir(), "artifact.bin"))
	if err == nil {
		t.Fatal("expected untrusted host error")
	}
	if requests != 0 {
		t.Fatalf("got %d requests, want none", requests)
	}
}

func TestDownloadRejectsToolWithoutSHA256(t *testing.T) {
	sha1Value := sha1.Sum([]byte("fixture"))
	_, err := (Downloader{}).Download(context.Background(), DownloadSpec{
		URL:  "https://repo.maven.apache.org/maven2/example/tool.jar",
		SHA1: fmt.Sprintf("%x", sha1Value),
	}, filepath.Join(t.TempDir(), "artifact.bin"))
	if err == nil {
		t.Fatal("expected missing SHA-256 error")
	}
}

func TestDownloadRejectsMojangArtifactWithoutSHA1(t *testing.T) {
	sha256Value := sha256.Sum256([]byte("fixture"))
	_, err := (Downloader{}).Download(context.Background(), DownloadSpec{
		URL:    trustedFixtureURL,
		SHA256: fmt.Sprintf("%x", sha256Value),
	}, filepath.Join(t.TempDir(), "artifact.bin"))
	if err == nil {
		t.Fatal("expected missing SHA-1 error")
	}
}

func TestDownloadRejectsRedirectToUntrustedHost(t *testing.T) {
	payload := []byte("minecraft fixture")
	sha1Value := sha1.Sum(payload)
	allowedRequests := 0
	allowedServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		allowedRequests++
		http.Redirect(writer, request, "https://example.invalid/file", http.StatusFound)
	}))
	defer allowedServer.Close()
	deniedRequests := 0
	deniedServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		deniedRequests++
		_, _ = writer.Write(payload)
	}))
	defer deniedServer.Close()

	_, err := (Downloader{Client: rewriteHostsClient(t, map[string]*httptest.Server{
		"example.invalid":        deniedServer,
		"piston-data.mojang.com": allowedServer,
	})}).Download(context.Background(), DownloadSpec{
		URL:  trustedFixtureURL,
		SHA1: fmt.Sprintf("%x", sha1Value),
	}, filepath.Join(t.TempDir(), "artifact.bin"))
	if err == nil {
		t.Fatal("expected redirect to untrusted host error")
	}
	if allowedRequests != 1 {
		t.Fatalf("got %d allowed-host requests, want 1", allowedRequests)
	}
	if deniedRequests != 0 {
		t.Fatalf("got %d denied-host requests, want none", deniedRequests)
	}
}

func TestDownloadStopsAfterTenRedirects(t *testing.T) {
	payload := []byte("minecraft fixture")
	sha1Value := sha1.Sum(payload)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if requests <= 10 {
			http.Redirect(writer, request, trustedFixtureURL, http.StatusFound)
			return
		}
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	_, err := (Downloader{Client: rewriteHostClient(t, server, "piston-data.mojang.com")}).Download(context.Background(), DownloadSpec{
		URL:  trustedFixtureURL,
		SHA1: fmt.Sprintf("%x", sha1Value),
	}, filepath.Join(t.TempDir(), "artifact.bin"))
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("got %v, want redirect-limit error", err)
	}
	if requests != 10 {
		t.Fatalf("got %d requests, want 10", requests)
	}
}

func rewriteHostClient(t *testing.T, server *httptest.Server, hostname string) *http.Client {
	t.Helper()
	return rewriteHostsClient(t, map[string]*httptest.Server{hostname: server})
}

func rewriteHostsClient(t *testing.T, servers map[string]*httptest.Server) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		server, ok := servers[request.URL.Hostname()]
		if !ok {
			return nil, fmt.Errorf("unexpected host %q", request.URL.Hostname())
		}
		target, err := url.Parse(server.URL)
		if err != nil {
			return nil, err
		}
		cloned := request.Clone(request.Context())
		cloned.URL = new(url.URL)
		*cloned.URL = *request.URL
		cloned.URL.Scheme = target.Scheme
		cloned.URL.Host = target.Host
		return server.Client().Transport.RoundTrip(cloned)
	})}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

// recordingSleeper stands in for the retry wait so tests observe the backoff
// schedule without serving it.
func recordingSleeper(delays *[]time.Duration) func(context.Context, time.Duration) error {
	return func(_ context.Context, delay time.Duration) error {
		*delays = append(*delays, delay)
		return nil
	}
}

func TestDownloadRetriesThrottledResponse(t *testing.T) {
	payload := []byte("minecraft fixture")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		switch requests {
		case 1:
			writer.WriteHeader(http.StatusTooManyRequests)
		case 2:
			writer.WriteHeader(http.StatusServiceUnavailable)
		default:
			_, _ = writer.Write(payload)
		}
	}))
	defer server.Close()

	var delays []time.Duration
	digest := sha1.Sum(payload)
	destination := filepath.Join(t.TempDir(), "artifact.bin")
	downloader := Downloader{Client: rewriteHostClient(t, server, "piston-data.mojang.com"), sleep: recordingSleeper(&delays)}

	result, err := downloader.Download(context.Background(), DownloadSpec{URL: trustedFixtureURL, Size: int64(len(payload)), SHA1: fmt.Sprintf("%x", digest)}, destination)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 3 {
		t.Errorf("made %d requests, want 3", requests)
	}
	if result.Size != int64(len(payload)) {
		t.Errorf("downloaded %d bytes, want %d", result.Size, len(payload))
	}
	if want := []time.Duration{initialRetryDelay, 2 * initialRetryDelay}; !reflect.DeepEqual(delays, want) {
		t.Errorf("backoff delays %v, want %v", delays, want)
	}
}

func TestDownloadHonoursRetryAfter(t *testing.T) {
	payload := []byte("minecraft fixture")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			writer.Header().Set("Retry-After", "7")
			writer.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	var delays []time.Duration
	digest := sha1.Sum(payload)
	downloader := Downloader{Client: rewriteHostClient(t, server, "piston-data.mojang.com"), sleep: recordingSleeper(&delays)}

	if _, err := downloader.Download(context.Background(), DownloadSpec{URL: trustedFixtureURL, SHA1: fmt.Sprintf("%x", digest)}, filepath.Join(t.TempDir(), "artifact.bin")); err != nil {
		t.Fatal(err)
	}
	if want := []time.Duration{7 * time.Second}; !reflect.DeepEqual(delays, want) {
		t.Errorf("backoff delays %v, want %v", delays, want)
	}
}

func TestDownloadCapsRetryAfterAtMaximumDelay(t *testing.T) {
	payload := []byte("minecraft fixture")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			writer.Header().Set("Retry-After", "86400")
			writer.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	var delays []time.Duration
	digest := sha1.Sum(payload)
	downloader := Downloader{Client: rewriteHostClient(t, server, "piston-data.mojang.com"), sleep: recordingSleeper(&delays)}

	if _, err := downloader.Download(context.Background(), DownloadSpec{URL: trustedFixtureURL, SHA1: fmt.Sprintf("%x", digest)}, filepath.Join(t.TempDir(), "artifact.bin")); err != nil {
		t.Fatal(err)
	}
	if want := []time.Duration{maxRetryDelay}; !reflect.DeepEqual(delays, want) {
		t.Errorf("backoff delays %v, want %v", delays, want)
	}
}

func TestDownloadGivesUpAfterMaximumAttempts(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	var delays []time.Duration
	destination := filepath.Join(t.TempDir(), "artifact.bin")
	downloader := Downloader{Client: rewriteHostClient(t, server, "piston-data.mojang.com"), sleep: recordingSleeper(&delays)}

	_, err := downloader.Download(context.Background(), DownloadSpec{URL: trustedFixtureURL, SHA1: fmt.Sprintf("%x", sha1.Sum(nil))}, destination)
	if err == nil {
		t.Fatal("expected the throttled download to fail")
	}
	if !strings.Contains(err.Error(), "429") || !strings.Contains(err.Error(), "after 5 attempts") {
		t.Errorf("got %v", err)
	}
	if requests != maxDownloadAttempts {
		t.Errorf("made %d requests, want %d", requests, maxDownloadAttempts)
	}
	if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
		t.Errorf("destination exists after failure: %v", statErr)
	}
}

func TestDownloadDoesNotRetryClientRejection(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	downloader := Downloader{Client: rewriteHostClient(t, server, "piston-data.mojang.com"), sleep: func(context.Context, time.Duration) error {
		t.Error("waited before retrying a 404")
		return nil
	}}
	if _, err := downloader.Download(context.Background(), DownloadSpec{URL: trustedFixtureURL, SHA1: fmt.Sprintf("%x", sha1.Sum(nil))}, filepath.Join(t.TempDir(), "artifact.bin")); err == nil {
		t.Fatal("expected the missing artifact to fail")
	}
	if requests != 1 {
		t.Errorf("made %d requests, want 1", requests)
	}
}

func TestDownloadRetryRewindsPartialBody(t *testing.T) {
	payload := []byte("minecraft fixture")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			writer.Header().Set("Content-Length", fmt.Sprint(len(payload)+8))
			_, _ = writer.Write(payload[:4])
			return
		}
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	var delays []time.Duration
	digest := sha1.Sum(payload)
	downloader := Downloader{Client: rewriteHostClient(t, server, "piston-data.mojang.com"), sleep: recordingSleeper(&delays)}

	result, err := downloader.Download(context.Background(), DownloadSpec{URL: trustedFixtureURL, Size: int64(len(payload)), SHA1: fmt.Sprintf("%x", digest)}, filepath.Join(t.TempDir(), "artifact.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Errorf("made %d requests, want 2", requests)
	}
	if result.Size != int64(len(payload)) {
		t.Errorf("downloaded %d bytes, want %d", result.Size, len(payload))
	}
}

func TestDownloadStopsRetryingOnCancelledContext(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		writer.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	downloader := Downloader{Client: rewriteHostClient(t, server, "piston-data.mojang.com"), sleep: func(context.Context, time.Duration) error {
		cancel()
		return ctx.Err()
	}}
	_, err := downloader.Download(ctx, DownloadSpec{URL: trustedFixtureURL, SHA1: fmt.Sprintf("%x", sha1.Sum(nil))}, filepath.Join(t.TempDir(), "artifact.bin"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want a cancelled context", err)
	}
	if requests != 1 {
		t.Errorf("made %d requests, want 1", requests)
	}
}

func TestDownloadReportsEachRetry(t *testing.T) {
	payload := []byte("minecraft fixture")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			writer.Header().Set("Retry-After", "3")
			writer.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	var reported []string
	var delays []time.Duration
	digest := sha1.Sum(payload)
	downloader := Downloader{
		Client:   rewriteHostClient(t, server, "piston-data.mojang.com"),
		Progress: func(message string) { reported = append(reported, message) },
		sleep:    recordingSleeper(&delays),
	}

	if _, err := downloader.Download(context.Background(), DownloadSpec{URL: trustedFixtureURL, SHA1: fmt.Sprintf("%x", digest)}, filepath.Join(t.TempDir(), "artifact.bin")); err != nil {
		t.Fatal(err)
	}
	if len(reported) != 1 {
		t.Fatalf("reported %d retries, want 1: %v", len(reported), reported)
	}
	for _, want := range []string{"retry 2 of 5", "in 3s", "429"} {
		if !strings.Contains(reported[0], want) {
			t.Errorf("retry report %q does not mention %q", reported[0], want)
		}
	}
}

func TestDownloadReportsNothingWhenNoRetryIsNeeded(t *testing.T) {
	payload := []byte("minecraft fixture")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	digest := sha1.Sum(payload)
	downloader := Downloader{
		Client:   rewriteHostClient(t, server, "piston-data.mojang.com"),
		Progress: func(message string) { t.Errorf("reported %q on a clean download", message) },
	}
	if _, err := downloader.Download(context.Background(), DownloadSpec{URL: trustedFixtureURL, SHA1: fmt.Sprintf("%x", digest)}, filepath.Join(t.TempDir(), "artifact.bin")); err != nil {
		t.Fatal(err)
	}
}
