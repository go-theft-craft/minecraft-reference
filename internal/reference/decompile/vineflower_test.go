package decompile

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestVineflowerArgsPreserveResearchSymbols(t *testing.T) {
	got := VineflowerArgs("tool.jar", "input.jar", "sources", []string{"one.jar", "two with spaces.jar"})
	for _, expected := range []string{
		"--thread-count=1",
		"--log-level=warn",
		"--remove-bridge=false",
		"--remove-synthetic=false",
		"--add-external=one.jar",
		"--add-external=two with spaces.jar",
		"--only=net/minecraft/",
	} {
		if !slices.Contains(got, expected) {
			t.Errorf("arguments lack %q: %#v", expected, got)
		}
	}
}

func TestPruneSourcesKeepsOnlyMinecraftPackage(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{
		filepath.Join(root, "net", "minecraft", "Entity.java"),
		filepath.Join(root, "net", "thirdparty", "Library.java"),
		filepath.Join(root, "javax", "annotation", "Nullable.java"),
		filepath.Join(root, "META-INF", "MANIFEST.MF"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := pruneSources(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "net", "minecraft", "Entity.java")); err != nil {
		t.Fatalf("Minecraft source missing: %v", err)
	}
	for _, path := range []string{filepath.Join(root, "net", "thirdparty"), filepath.Join(root, "javax"), filepath.Join(root, "META-INF")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("non-Minecraft source remains at %s: %v", path, err)
		}
	}
}
