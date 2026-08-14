package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/artifact"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
)

func TestCleanRemovesOnlyValidatedChild(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "reference", "work")
	if err := os.MkdirAll(target, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "fixture"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	removed, err := Clean(root, "reference/work")
	if err != nil {
		t.Fatal(err)
	}
	if removed != target {
		t.Fatalf("got %q, want %q", removed, target)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target still exists: %v", err)
	}
}

func TestCleanRejectsRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	_, err := Clean(root, root)
	if !errors.Is(err, artifact.ErrUnsafeReferenceDir) {
		t.Fatalf("got %v, want ErrUnsafeReferenceDir", err)
	}
}

func TestPrepareNamedJarRejectsUnconfiguredSide(t *testing.T) {
	_, _, err := prepareNamedJar(context.Background(), namingOptions{
		Version:     config.Version{ID: "1.1", Naming: "identity", Sides: configuredSides("client")},
		Side:        "server",
		AnalysisJar: filepath.Join(t.TempDir(), "analysis.jar"),
	})
	if err == nil || !strings.Contains(err.Error(), `version "1.1" does not support side "server"`) {
		t.Fatalf("got %v, want version and side error", err)
	}
}

func TestAppendArtifactResultsKeepsOneRecordPerPath(t *testing.T) {
	existing := []artifact.DownloadResult{
		{Path: "metadata.json"},
		{Path: "mapping.zip", Cached: false},
	}
	got := appendArtifactResults(
		existing,
		artifact.DownloadResult{Path: "mapping.zip", Cached: true},
		artifact.DownloadResult{Path: "client.txt"},
		artifact.DownloadResult{Path: "server.txt"},
	)
	if len(got) != 4 {
		t.Fatalf("got %d artifacts, want 4: %#v", len(got), got)
	}
	if got[1].Path != "mapping.zip" || got[1].Cached {
		t.Fatalf("shared artifact record was replaced: %#v", got[1])
	}
}

func TestPrepareInvalidatesPriorSuccessBeforePreflightFailure(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteVersionFile(filepath.Join(configDir, "versions.json"), []config.Version{{
		ID: "test", Family: "1.0", Java: 8, Naming: "identity",
		Sides: map[string]config.Validation{"client": {MinSources: 1, MinSymbols: 1}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "tools.json"), []byte(`{"tools":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	versionDir := filepath.Join(root, "reference", "work", "versions", "test")
	for _, path := range []string{
		filepath.Join(versionDir, "manifest.lock.json"),
		filepath.Join(versionDir, "client", "compatibility.json"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"passed":true}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	err := Prepare(context.Background(), Options{
		WorkspaceRoot: root,
		ConfigDir:     configDir,
		ReferenceDir:  "reference/work",
		Versions:      []string{"test"},
		Sides:         []string{"client"},
		Java:          filepath.Join(root, "missing-java"),
		Javap:         filepath.Join(root, "missing-javap"),
	})
	if err == nil || !strings.Contains(err.Error(), "find") {
		t.Fatalf("got %v, want preflight executable error", err)
	}
	for _, path := range []string{
		filepath.Join(versionDir, "manifest.lock.json"),
		filepath.Join(versionDir, "client", "compatibility.json"),
	} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("stale success marker remains at %s: %v", path, statErr)
		}
	}
}

func TestInvalidateVersionOutputsLimitsReportsToRequestedSides(t *testing.T) {
	versionDir := t.TempDir()
	paths := map[string]bool{
		filepath.Join(versionDir, "manifest.lock.json"):           false,
		filepath.Join(versionDir, "client", "compatibility.json"): false,
		filepath.Join(versionDir, "server", "compatibility.json"): true,
	}
	for path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"passed":true}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := invalidateVersionOutputs(versionDir, []string{"client"}); err != nil {
		t.Fatal(err)
	}
	for path, wantExists := range paths {
		_, err := os.Stat(path)
		if wantExists && err != nil {
			t.Fatalf("expected %s to remain: %v", path, err)
		}
		if !wantExists && !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed: %v", path, err)
		}
	}
}

func TestPrepareInvalidatesRequestedOutputsBeforeMalformedCatalogFailure(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "versions.json"), []byte(`{"versions":`), 0o600); err != nil {
		t.Fatal(err)
	}
	writeSuccessMarkers(t, root, "test", "client")
	writeSuccessMarkers(t, root, "unrequested", "client")

	err := Prepare(context.Background(), Options{
		WorkspaceRoot: root, ConfigDir: configDir, ReferenceDir: "reference/work",
		Versions: []string{"test"}, Sides: []string{"client"},
	})
	if err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("got %v, want malformed catalog error", err)
	}
	assertSuccessMarkersRemoved(t, root, "test", "client")
	assertSuccessMarkersExist(t, root, "unrequested", "client")
}

func TestPrepareInvalidatesAllSafeRequestsBeforeUnknownVersionFailure(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteVersionFile(filepath.Join(configDir, "versions.json"), []config.Version{{
		ID: "known", Family: "1.0", Java: 8, Naming: "identity",
		Sides: map[string]config.Validation{"client": {MinSources: 1, MinSymbols: 1}},
	}}); err != nil {
		t.Fatal(err)
	}
	writeSuccessMarkers(t, root, "known", "client")
	writeSuccessMarkers(t, root, "unknown", "client")

	err := Prepare(context.Background(), Options{
		WorkspaceRoot: root, ConfigDir: configDir, ReferenceDir: "reference/work",
		Versions: []string{"known", "unknown"}, Sides: []string{"client"},
	})
	if !errors.Is(err, config.ErrUnsupportedVersion) {
		t.Fatalf("got %v, want unsupported version error", err)
	}
	assertSuccessMarkersRemoved(t, root, "known", "client")
	assertSuccessMarkersRemoved(t, root, "unknown", "client")
}

func TestPrepareRejectsUnsafeVersionIDsBeforeCleanup(t *testing.T) {
	root := t.TempDir()
	writeSuccessMarkers(t, root, "safe", "client")
	escapeDir := filepath.Join(root, "reference", "work", "escape")
	for _, path := range []string{filepath.Join(escapeDir, "manifest.lock.json"), filepath.Join(escapeDir, "client", "compatibility.json")} {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"passed":true}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	err := Prepare(context.Background(), Options{
		WorkspaceRoot: root, ReferenceDir: "reference/work",
		Versions: []string{"safe", "../escape"}, Sides: []string{"client"},
	})
	if err == nil || !strings.Contains(err.Error(), "unsafe version") {
		t.Fatalf("got %v, want unsafe version error", err)
	}
	assertSuccessMarkersExist(t, root, "safe", "client")
	for _, path := range []string{filepath.Join(escapeDir, "manifest.lock.json"), filepath.Join(escapeDir, "client", "compatibility.json")} {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("unsafe cleanup target changed at %s: %v", path, statErr)
		}
	}
}

func writeSuccessMarkers(t *testing.T, root, version, side string) {
	t.Helper()
	for _, path := range successMarkerPaths(root, version, side) {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"passed":true}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func assertSuccessMarkersRemoved(t *testing.T, root, version, side string) {
	t.Helper()
	for _, path := range successMarkerPaths(root, version, side) {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("success marker remains at %s: %v", path, err)
		}
	}
}

func assertSuccessMarkersExist(t *testing.T, root, version, side string) {
	t.Helper()
	for _, path := range successMarkerPaths(root, version, side) {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("success marker changed at %s: %v", path, err)
		}
	}
}

func successMarkerPaths(root, version, side string) []string {
	versionDir := filepath.Join(root, "reference", "work", "versions", version)
	return []string{
		filepath.Join(versionDir, "manifest.lock.json"),
		filepath.Join(versionDir, side, "compatibility.json"),
	}
}
