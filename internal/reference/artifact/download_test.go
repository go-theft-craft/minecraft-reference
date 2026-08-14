package artifact

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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
	spec := DownloadSpec{URL: server.URL, Size: int64(len(payload)), SHA1: fmt.Sprintf("%x", sha1Value)}
	downloader := Downloader{Client: server.Client()}

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
	want := sha256.Sum256([]byte("right"))
	_, err := (Downloader{Client: server.Client()}).Download(context.Background(), DownloadSpec{URL: server.URL, SHA256: fmt.Sprintf("%x", want)}, destination)
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
	digest := sha256.Sum256(payload)
	if _, err := (Downloader{Client: server.Client()}).Download(context.Background(), DownloadSpec{URL: server.URL, Size: int64(len(payload)), SHA256: fmt.Sprintf("%x", digest)}, destination); err != nil {
		t.Fatal(err)
	}
}
