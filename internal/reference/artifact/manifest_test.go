package artifact

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestListReleasesPreservesMojangManifestEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{
  "versions": [
    {"id":"1.20.6","type":"release","releaseTime":"2024-04-29T12:00:00+00:00","url":"https://piston-meta.mojang.com/v1/packages/release.json","sha1":"release-sha1"},
    {"id":"26.1-snapshot-1","type":"snapshot","releaseTime":"2026-08-12T12:00:00+00:00","url":"https://piston-meta.mojang.com/v1/packages/snapshot.json","sha1":"snapshot-sha1"},
    {"id":"b1.7.3","type":"old_beta","releaseTime":"2011-07-08T12:00:00+00:00","url":"https://piston-meta.mojang.com/v1/packages/beta.json","sha1":"beta-sha1"}
  ]
}`))
	}))
	defer server.Close()

	resolver := Resolver{
		Client:      rewriteHostClient(t, server, "piston-meta.mojang.com"),
		ManifestURL: "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json",
	}
	releases, err := resolver.ListReleases(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	wantTime, err := time.Parse(time.RFC3339, "2024-04-29T12:00:00+00:00")
	if err != nil {
		t.Fatal(err)
	}
	want := []Release{
		{ID: "1.20.6", Type: "release", ReleaseTime: wantTime, URL: "https://piston-meta.mojang.com/v1/packages/release.json", SHA1: "release-sha1"},
		{ID: "26.1-snapshot-1", Type: "snapshot", ReleaseTime: mustParseReleaseTime(t, "2026-08-12T12:00:00+00:00"), URL: "https://piston-meta.mojang.com/v1/packages/snapshot.json", SHA1: "snapshot-sha1"},
		{ID: "b1.7.3", Type: "old_beta", ReleaseTime: mustParseReleaseTime(t, "2011-07-08T12:00:00+00:00"), URL: "https://piston-meta.mojang.com/v1/packages/beta.json", SHA1: "beta-sha1"},
	}
	if len(releases) != len(want) {
		t.Fatalf("got %d releases, want %d", len(releases), len(want))
	}
	for i := range want {
		if releases[i] != want[i] {
			t.Errorf("release %d: got %#v, want %#v", i, releases[i], want[i])
		}
	}
}

func TestListReleasesRejectsUntrustedManifestURLBeforeRequest(t *testing.T) {
	requests := 0
	resolver := Resolver{
		Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			requests++
			return nil, nil
		})},
		ManifestURL: "https://example.invalid/mc/game/version_manifest_v2.json",
	}
	_, err := resolver.ListReleases(context.Background())
	if err == nil {
		t.Fatal("expected untrusted manifest URL error")
	}
	if requests != 0 {
		t.Fatalf("got %d requests, want none", requests)
	}
}

func TestListReleasesRejectsRedirectToUntrustedHost(t *testing.T) {
	allowedRequests := 0
	allowedServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		allowedRequests++
		http.Redirect(writer, request, "https://example.invalid/mc/game/version_manifest_v2.json", http.StatusFound)
	}))
	defer allowedServer.Close()
	deniedRequests := 0
	deniedServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		deniedRequests++
		_, _ = writer.Write([]byte(`{"versions":[]}`))
	}))
	defer deniedServer.Close()

	resolver := Resolver{
		Client: rewriteHostsClient(t, map[string]*httptest.Server{
			"example.invalid":        deniedServer,
			"piston-meta.mojang.com": allowedServer,
		}),
		ManifestURL: "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json",
	}
	_, err := resolver.ListReleases(context.Background())
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

func TestListReleasesAllowsLauncherMetadataHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"versions":[]}`))
	}))
	defer server.Close()

	resolver := Resolver{
		Client:      rewriteHostClient(t, server, "launcher.mojang.com"),
		ManifestURL: "https://launcher.mojang.com/mc/game/version_manifest_v2.json",
	}
	if _, err := resolver.ListReleases(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeVersionFillsLibraryURL(t *testing.T) {
	resolver := Resolver{}
	metadata, err := resolver.DecodeVersion([]byte(`{
  "id":"1.8.9",
  "downloads":{"client":{"sha1":"abc","size":1,"url":"https://example/client.jar"}},
  "libraries":[{"name":"example:test:1","downloads":{"artifact":{"path":"example/test/1/test-1.jar","sha1":"def","size":2,"url":""}}}]
}`), "1.8.9")
	if err != nil {
		t.Fatal(err)
	}
	got := metadata.Libraries[0].Downloads.Artifact.URL
	want := "https://libraries.minecraft.net/example/test/1/test-1.jar"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDecodeVersionReadsJavaMajor(t *testing.T) {
	metadata, err := (Resolver{}).DecodeVersion([]byte(`{"id":"1.20.6","javaVersion":{"majorVersion":21},"downloads":{},"libraries":[]}`), "1.20.6")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.JavaVersion.MajorVersion != 21 {
		t.Fatalf("got Java major %d, want 21", metadata.JavaVersion.MajorVersion)
	}
}

func TestDecodeVersionRejectsWrongID(t *testing.T) {
	_, err := (Resolver{}).DecodeVersion([]byte(`{"id":"other"}`), "1.8.9")
	if err == nil {
		t.Fatal("expected id mismatch")
	}
}

func TestLibraryDetectsNativeClassifierCoordinate(t *testing.T) {
	if !(Library{Name: "org.lwjgl:lwjgl:3.4.1:natives-linux"}).HasClassifier() {
		t.Fatal("expected native classifier")
	}
	if !(Library{Name: "io.netty:netty-transport-native-epoll:4.2.7.Final:linux-x86_64"}).HasClassifier() {
		t.Fatal("expected platform classifier")
	}
	if (Library{Name: "org.lwjgl:lwjgl:3.4.1"}).HasClassifier() {
		t.Fatal("plain Java artifact detected as native")
	}
}

func mustParseReleaseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
