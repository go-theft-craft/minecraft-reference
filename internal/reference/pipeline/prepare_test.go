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
