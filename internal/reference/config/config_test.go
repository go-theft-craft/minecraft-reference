package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadToolsRejectsMalformedDigest(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "config")
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"tools":[{"id":"broken","url":"https://example.invalid/tool","sha256":"short"}]}`)
	if err := os.WriteFile(filepath.Join(directory, "tools.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTools(directory); err == nil {
		t.Fatal("expected invalid SHA-256 error")
	}
}

func TestEmbeddedDefaultsLoad(t *testing.T) {
	versions, err := LoadVersions("")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := versions["1.8.9"]; !ok {
		t.Fatal("embedded versions do not contain 1.8.9")
	}
	tools, err := LoadTools("")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tools["vineflower-1.12.0"]; !ok {
		t.Fatal("embedded tools do not contain Vineflower")
	}
}

func TestLoadVersionsRejectsMissingJavaRequirement(t *testing.T) {
	directory := t.TempDir()
	data := []byte(`{"versions":[{"id":"test","java":0,"naming":"identity"}]}`)
	if err := os.WriteFile(filepath.Join(directory, "versions.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadVersions(directory)
	if err == nil || !strings.Contains(err.Error(), "invalid Java requirement") {
		t.Fatalf("got %v", err)
	}
}
