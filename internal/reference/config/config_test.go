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
	if err == nil || !strings.Contains(err.Error(), `version "test" field java`) {
		t.Fatalf("got %v", err)
	}
}

func TestReadVersionFileValidation(t *testing.T) {
	const complete = `{
  "versions": [{
    "id": "1.7.10",
    "family": "1.7",
    "java": 8,
    "naming": "mcp",
    "mapping": {"tool": "mcp-1.7.10-tiny", "format": "tiny-v1"},
    "sides": {
      "client": {"min_sources": 100, "min_symbols": 1000, "required_classes": ["Minecraft"]},
      "server": {"min_sources": 100, "min_symbols": 1000, "required_classes": ["MinecraftServer"]}
    }
  }]
}`
	tests := []struct {
		name    string
		data    string
		wantErr string
	}{
		{name: "complete entry", data: complete},
		{
			name: "duplicate families",
			data: `{
  "versions": [
    {"id":"1.7.10","family":"1.7","java":8,"naming":"identity","sides":{"client":{"min_sources":1,"min_symbols":1}}},
    {"id":"1.8.9","family":"1.7","java":8,"naming":"identity","sides":{"client":{"min_sources":1,"min_symbols":1}}}
  ]
}`,
			wantErr: `version "1.8.9" field family`,
		},
		{
			name:    "unknown side",
			data:    strings.Replace(complete, `"server": {`, `"invalid": {"min_sources": 100, "min_symbols": 1000}, "server": {`, 1),
			wantErr: `version "1.7.10" field sides.invalid`,
		},
		{
			name:    "missing mapping data",
			data:    strings.Replace(complete, `{"tool": "mcp-1.7.10-tiny", "format": "tiny-v1"}`, `{}`, 1),
			wantErr: `version "1.7.10" field mapping.format`,
		},
		{
			name:    "empty validation limits",
			data:    strings.Replace(complete, `"min_sources": 100`, `"min_sources": 0`, 1),
			wantErr: `version "1.7.10" field sides.client.min_sources`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "versions.json")
			if err := os.WriteFile(path, []byte(test.data), 0o600); err != nil {
				t.Fatal(err)
			}
			versions, err := ReadVersionFile(path)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("got %v, want error containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(versions) != 1 || !versions[0].SupportsSide("client") || versions[0].SupportsSide("invalid") {
				t.Fatalf("unexpected versions: %#v", versions)
			}
		})
	}
}

func TestReadVersionFileRejectsDuplicateIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "versions.json")
	data := []byte(`{
  "versions": [
    {"id":"one","family":"1.0","java":8,"naming":"identity","sides":{"client":{"min_sources":1,"min_symbols":1}}},
    {"id":"one","family":"2.0","java":8,"naming":"identity","sides":{"client":{"min_sources":1,"min_symbols":1}}}
  ]
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ReadVersionFile(path)
	if err == nil || !strings.Contains(err.Error(), `version "one" field id`) {
		t.Fatalf("got %v", err)
	}
}

func TestReadVersionFileRejectsInvalidFamilyShape(t *testing.T) {
	const entry = `{"versions":[{"id":"test","family":"26.1","java":25,"naming":"identity","sides":{"client":{"min_sources":1,"min_symbols":1}}}]}`
	for _, family := range []string{"26", "26.1.2"} {
		t.Run(family, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "versions.json")
			data := strings.Replace(entry, `"26.1"`, `"`+family+`"`, 1)
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := ReadVersionFile(path)
			if err == nil || !strings.Contains(err.Error(), `version "test" field family`) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestWriteVersionFileSortsByNumericFamily(t *testing.T) {
	path := filepath.Join(t.TempDir(), "versions.json")
	versions := []Version{
		{ID: "26.1.2", Family: "26.1", Java: 25, Naming: "identity", Sides: map[string]Validation{"client": {MinSources: 1, MinSymbols: 1}}},
		{ID: "1.8.9", Family: "1.8", Java: 8, Naming: "mcp", Mapping: &Mapping{Format: "tiny-v1", Tool: "mcp-1.8.9-tiny"}, Sides: map[string]Validation{"client": {MinSources: 1, MinSymbols: 1}}},
	}
	if err := WriteVersionFile(path, versions); err != nil {
		t.Fatal(err)
	}
	got, err := ReadVersionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Family != "1.8" || got[1].Family != "26.1" {
		t.Fatalf("unexpected order: %#v", got)
	}
}
