package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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

func TestEffectiveJavaMajorIncludesReferenceToolchain(t *testing.T) {
	tests := []struct {
		minecraft int
		want      int
	}{
		{minecraft: 8, want: 17},
		{minecraft: 16, want: 17},
		{minecraft: 17, want: 17},
		{minecraft: 21, want: 21},
		{minecraft: 25, want: 25},
	}
	for _, test := range tests {
		if got := EffectiveJavaMajor(test.minecraft); got != test.want {
			t.Errorf("EffectiveJavaMajor(%d) = %d, want %d", test.minecraft, got, test.want)
		}
	}
}

func TestEmbeddedDefaultsContainInitialStableFamilyCatalog(t *testing.T) {
	versions, err := ReadVersionFile(filepath.Join("defaults", "versions.json"))
	if err != nil {
		t.Fatal(err)
	}

	type expectedVersion struct {
		id      string
		family  string
		java    int
		naming  string
		mapping *Mapping
		sides   []string
	}
	tiny := func(id string) *Mapping {
		return &Mapping{Format: "tiny-v1", Tool: "mcp-" + id + "-tiny"}
	}
	mojangSides := []string{"client", "server"}
	expected := []expectedVersion{
		{id: "1.0", family: "1.0", java: 8, naming: "mcp", mapping: tiny("1.0"), sides: []string{"client"}},
		{id: "1.1", family: "1.1", java: 8, naming: "mcp", mapping: tiny("1.1"), sides: []string{"client"}},
		{id: "1.2.5", family: "1.2", java: 8, naming: "mcp", mapping: tiny("1.2.5"), sides: mojangSides},
		{id: "1.3.2", family: "1.3", java: 8, naming: "mcp", mapping: tiny("1.3.2"), sides: mojangSides},
		{id: "1.4.7", family: "1.4", java: 8, naming: "mcp", mapping: tiny("1.4.7"), sides: mojangSides},
		{id: "1.5.2", family: "1.5", java: 8, naming: "mcp", mapping: tiny("1.5.2"), sides: mojangSides},
		{id: "1.6.4", family: "1.6", java: 8, naming: "mcp", mapping: tiny("1.6.4"), sides: mojangSides},
		{id: "1.7.10", family: "1.7", java: 8, naming: "mcp", mapping: tiny("1.7.10"), sides: mojangSides},
		{
			id: "1.8.9", family: "1.8", java: 8, naming: "mcp",
			mapping: &Mapping{Format: "srg-csv", SRGTool: "mcp-1.8.9-srg", NamesTool: "mcp-stable-22-1.8.9"}, sides: mojangSides,
		},
		{id: "1.9.4", family: "1.9", java: 8, naming: "mcp", mapping: tiny("1.9.4"), sides: mojangSides},
		{
			id: "1.10.2", family: "1.10", java: 8, naming: "mcp",
			mapping: &Mapping{Format: "srg-csv", SRGTool: "mcp-1.10.2-srg", NamesTool: "mcp-stable-29-1.10.2"}, sides: mojangSides,
		},
		{id: "1.11.2", family: "1.11", java: 8, naming: "mcp", mapping: tiny("1.11.2"), sides: mojangSides},
		{id: "1.12.2", family: "1.12", java: 8, naming: "mcp", mapping: tiny("1.12.2"), sides: mojangSides},
		{id: "1.13.2", family: "1.13", java: 8, naming: "mcp", mapping: tiny("1.13.2"), sides: mojangSides},
		{id: "1.14.4", family: "1.14", java: 8, naming: "mojang", sides: mojangSides},
		{id: "1.15.2", family: "1.15", java: 8, naming: "mojang", sides: mojangSides},
		{id: "1.16.5", family: "1.16", java: 8, naming: "mojang", sides: mojangSides},
		{id: "1.17.1", family: "1.17", java: 16, naming: "mojang", sides: mojangSides},
		{id: "1.18.2", family: "1.18", java: 17, naming: "mojang", sides: mojangSides},
		{id: "1.19.4", family: "1.19", java: 17, naming: "mojang", sides: mojangSides},
		{id: "1.20.6", family: "1.20", java: 21, naming: "mojang", sides: mojangSides},
		{id: "1.21.11", family: "1.21", java: 21, naming: "mojang", sides: mojangSides},
		{id: "26.1.2", family: "26.1", java: 25, naming: "identity", sides: mojangSides},
		{id: "26.2", family: "26.2", java: 25, naming: "identity", sides: mojangSides},
	}
	if len(versions) != len(expected) {
		t.Fatalf("got %d configured versions, want %d", len(versions), len(expected))
	}
	for index, want := range expected {
		got := versions[index]
		if got.ID != want.id || got.Family != want.family || got.Java != want.java || got.Naming != want.naming || !reflect.DeepEqual(got.Mapping, want.mapping) {
			t.Errorf("version %d: got %#v, want %#v", index, got, want)
			continue
		}
		released, verified := parseEvidenceDate(t, got.ID, "release_date", got.ReleaseDate), parseEvidenceDate(t, got.ID, "verified_date", got.VerifiedDate)
		if !released.IsZero() && !verified.IsZero() && verified.Before(released) {
			t.Errorf("version %s was verified %q before it was released %q", got.ID, got.VerifiedDate, got.ReleaseDate)
		}
		gotSides := make([]string, 0, len(got.Sides))
		for _, side := range []string{"client", "server"} {
			validation, ok := got.Sides[side]
			if !ok {
				continue
			}
			gotSides = append(gotSides, side)
			wantClass := "Minecraft"
			if side == "server" {
				wantClass = "MinecraftServer"
			}
			if validation.MinSources < 1 || validation.MinSymbols < 1 || !reflect.DeepEqual(validation.RequiredClasses, []string{wantClass}) {
				t.Errorf("version %s side %s has unexpected validation: %#v", got.ID, side, validation)
			}
		}
		if !reflect.DeepEqual(gotSides, want.sides) {
			t.Errorf("version %s sides: got %v, want %v", got.ID, gotSides, want.sides)
		}
	}
}

// parseEvidenceDate requires a release evidence date to be present and to use
// the YYYY-MM-DD form. Each version carries its own dates, so a re-verified
// version moves independently of the rest of the catalog.
func parseEvidenceDate(t *testing.T, versionID, field, value string) time.Time {
	t.Helper()
	if value == "" {
		t.Errorf("version %s is missing %s", versionID, field)
		return time.Time{}
	}
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		t.Errorf("version %s field %s = %q, want YYYY-MM-DD", versionID, field, value)
		return time.Time{}
	}
	return parsed
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
			name:    "invalid release date",
			data:    strings.Replace(complete, `"java": 8,`, `"java": 8, "release_date": "2011-1-7",`, 1),
			wantErr: `version "1.7.10" field release_date must use YYYY-MM-DD`,
		},
		{
			name:    "missing mapping data",
			data:    strings.Replace(complete, `{"tool": "mcp-1.7.10-tiny", "format": "tiny-v1"}`, `{}`, 1),
			wantErr: `version "1.7.10" field mapping.format`,
		},
		{
			name:    "unknown mapping format",
			data:    strings.Replace(complete, `"format": "tiny-v1"`, `"format": "unknown"`, 1),
			wantErr: `version "1.7.10" field mapping.format has unsupported format "unknown"`,
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
