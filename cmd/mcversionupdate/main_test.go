package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/catalog"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/pipeline"
)

func TestAcceptCommandUpdatesVersionFileFromReports(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "versions.json")
	reportsPath := filepath.Join(directory, "reports")
	outputPath := configPath
	writeVersions(t, configPath, []config.Version{{
		ID: "26.1.2", Family: "26.1", Java: 25, Naming: "identity",
		Sides: map[string]config.Validation{
			"client": {MinSources: 1, MinSymbols: 1, RequiredClasses: []string{"Minecraft"}},
			"server": {MinSources: 1, MinSymbols: 1, RequiredClasses: []string{"MinecraftServer"}},
		},
	}})
	writeReport(t, filepath.Join(reportsPath, "26.1.2", "client", "compatibility.json"), pipeline.CompatibilityReport{
		Version: "26.1.2", Family: "26.1", Side: "client", JavaMajor: 25, JavapMajor: 25, Naming: "identity",
		NamedClasses: 10, SourceRecords: 101, SymbolRecords: 1001, RequiredClasses: []string{"Minecraft"}, Passed: true,
	})
	writeReport(t, filepath.Join(reportsPath, "26.1.2", "server", "compatibility.json"), pipeline.CompatibilityReport{
		Version: "26.1.2", Family: "26.1", Side: "server", JavaMajor: 25, JavapMajor: 25, Naming: "identity",
		NamedClasses: 10, SourceRecords: 201, SymbolRecords: 2001, RequiredClasses: []string{"MinecraftServer"}, Passed: true,
	})

	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"accept", "--config", configPath, "--reports", reportsPath, "--output", outputPath}, &stdout, &stderr, runDependencies{})
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := config.ReadVersionFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if accepted[0].Sides["client"].MinSources != 90 || accepted[0].Sides["server"].MinSymbols != 1800 {
		t.Fatalf("unexpected accepted thresholds: %#v", accepted)
	}
	if !strings.Contains(stdout.String(), "accepted 1 version") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestAcceptCommandDoesNotFollowReportSymlinks(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "versions.json")
	reportsPath := filepath.Join(directory, "reports")
	outside := filepath.Join(directory, "outside")
	outputPath := filepath.Join(directory, "accepted.json")
	writeVersions(t, configPath, []config.Version{{
		ID: "26.1.2", Family: "26.1", Java: 25, Naming: "identity",
		Sides: map[string]config.Validation{"client": {MinSources: 1, MinSymbols: 1}},
	}})
	writeReport(t, filepath.Join(outside, "compatibility.json"), pipeline.CompatibilityReport{
		Version: "26.1.2", Family: "26.1", Side: "client", JavaMajor: 25, JavapMajor: 25, Naming: "identity",
		NamedClasses: 1, SourceRecords: 1, SymbolRecords: 1, Passed: true,
	})
	if err := os.MkdirAll(reportsPath, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "compatibility.json"), filepath.Join(reportsPath, "compatibility.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(reportsPath, "linked-directory")); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"accept", "--config", configPath, "--reports", reportsPath, "--output", outputPath}, &stdout, &stderr, runDependencies{})
	if err == nil || !strings.Contains(err.Error(), "client") || !strings.Contains(err.Error(), "passing compatibility report") {
		t.Fatalf("got %v", err)
	}
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("output exists after failed acceptance: %v", statErr)
	}
}

func TestAcceptCommandRequiresAllPaths(t *testing.T) {
	var stdout, stderr bytes.Buffer
	for _, missing := range []string{"--config", "--reports", "--output"} {
		arguments := []string{"accept", "--config", "versions.json", "--reports", "reports", "--output", "output.json"}
		for index := 1; index < len(arguments); index += 2 {
			if arguments[index] == missing {
				arguments = append(arguments[:index], arguments[index+2:]...)
				break
			}
		}
		err := run(context.Background(), arguments, &stdout, &stderr, runDependencies{})
		if err == nil || !strings.Contains(err.Error(), missing+" is required") {
			t.Fatalf("missing %s: got %v", missing, err)
		}
	}
}

func TestSubcommandHelpExitsSuccessfully(t *testing.T) {
	for _, command := range []string{"discover", "matrix", "accept", "readme"} {
		t.Run(command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := run(context.Background(), []string{command, "--help"}, &stdout, &stderr, runDependencies{}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stderr.String(), "Usage:") {
				t.Fatalf("unexpected help output: %q", stderr.String())
			}
		})
	}
}

func TestDiscoverCommandWritesEmptyCandidateListWithoutMetadataRequests(t *testing.T) {
	root := t.TempDir()
	requests := make([]string, 0)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request.URL.Path)
		return jsonResponse(`{
  "versions": [
    {"id":"1.20.6","type":"release","releaseTime":"2024-04-29T12:00:00Z","url":"https://piston-meta.mojang.com/v1/1.20.6.json","sha1":"unused"}
  ]
}`), nil
	})}
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"discover", "--output", "candidate.json"}, &stdout, &stderr, runDependencies{HTTPClient: client, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 {
		t.Fatalf("got requests %v, want only the version manifest", requests)
	}
	file, err := catalog.ReadCandidateFile(filepath.Join(root, "candidate.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !file.HasCandidatesField() || file.Candidates == nil || len(file.Candidates) != 0 {
		t.Fatalf("unexpected candidate list: %#v", file.Candidates)
	}
	if len(file.Versions) == 0 {
		t.Fatal("candidate file omitted the current version configuration")
	}
	if !strings.Contains(stdout.String(), "discovered 0 candidate") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestDiscoverCommandKeepsUnavailableJDKAsCandidateMetadata(t *testing.T) {
	root := t.TempDir()
	metadata := []byte(`{
  "id":"99.1",
  "javaVersion":{"majorVersion":99},
  "downloads":{
    "client":{"sha1":"a","size":1,"url":"https://piston-data.mojang.com/client.jar"},
    "server":{"sha1":"b","size":1,"url":"https://piston-data.mojang.com/server.jar"},
    "client_mappings":{"sha1":"c","size":1,"url":"https://piston-data.mojang.com/client.txt"},
    "server_mappings":{"sha1":"d","size":1,"url":"https://piston-data.mojang.com/server.txt"}
  }
}`)
	digest := sha1.Sum(metadata) //nolint:gosec // Mojang metadata declares SHA-1.
	manifest := fmt.Sprintf(`{
  "versions":[{"id":"99.1","type":"release","releaseTime":"2026-08-14T12:00:00Z","url":"https://piston-meta.mojang.com/v1/99.1.json","sha1":"%x"}]
}`, digest)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/mc/game/version_manifest_v2.json":
			return jsonResponse(manifest), nil
		case "/v1/99.1.json":
			return jsonBytesResponse(metadata), nil
		default:
			return nil, fmt.Errorf("unexpected request %s", request.URL)
		}
	})}
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), []string{"discover", "--output", "candidate.json"}, &stdout, &stderr, runDependencies{HTTPClient: client, Root: root}); err != nil {
		t.Fatal(err)
	}
	file, err := catalog.ReadCandidateFile(filepath.Join(root, "candidate.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Candidates) != 1 || file.Candidates[0].New.Java != 99 {
		t.Fatalf("missing JDK candidate metadata: %#v", file.Candidates)
	}
}

func TestMatrixCommandPrintsCandidateSidesAsGitHubMatrix(t *testing.T) {
	root := t.TempDir()
	version := config.Version{
		ID: "26.2", Family: "26.2", Java: 25, Naming: "identity",
		Sides: map[string]config.Validation{
			"server": {MinSources: 1, MinSymbols: 1},
			"client": {MinSources: 1, MinSymbols: 1},
		},
	}
	if err := catalog.WriteCandidateFile(filepath.Join(root, "candidate.json"), []config.Version{version}, []catalog.Candidate{{Family: "26.2", New: version}}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), []string{"matrix", "--config", "candidate.json"}, &stdout, &stderr, runDependencies{Root: root}); err != nil {
		t.Fatal(err)
	}
	const want = `{"include":[{"version":"26.2","family":"26.2","side":"client","java":25},{"version":"26.2","family":"26.2","side":"server","java":25}]}` + "\n"
	if stdout.String() != want {
		t.Fatalf("got %q, want %q", stdout.String(), want)
	}
}

func TestAcceptCommandAcceptsOnlyChangedCandidatesAndPreservesCatalog(t *testing.T) {
	root := t.TempDir()
	unchanged := config.Version{
		ID: "1.19.4", Family: "1.19", Java: 17, Naming: "mojang",
		Sides: map[string]config.Validation{"client": {MinSources: 50, MinSymbols: 60}},
	}
	changed := config.Version{
		ID: "1.20.7", Family: "1.20", Java: 21, Naming: "mojang",
		Sides: map[string]config.Validation{"client": {MinSources: 1, MinSymbols: 1}},
	}
	configPath := filepath.Join(root, "candidate.json")
	if err := catalog.WriteCandidateFile(configPath, []config.Version{unchanged, changed}, []catalog.Candidate{{Family: "1.20", New: changed}}); err != nil {
		t.Fatal(err)
	}
	writeReport(t, filepath.Join(root, "reports", "compatibility.json"), pipeline.CompatibilityReport{
		Version: "1.20.7", Family: "1.20", Side: "client", JavaMajor: 21, JavapMajor: 21,
		Naming: "mojang", NamedClasses: 1, SourceRecords: 100, SymbolRecords: 200, Passed: true,
	})
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"accept", "--config", "candidate.json", "--reports", "reports", "--output", "versions.json"}, &stdout, &stderr, runDependencies{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := config.ReadVersionFile(filepath.Join(root, "versions.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(accepted) != 2 || accepted[0].ID != unchanged.ID || accepted[0].Sides["client"].MinSources != 50 || accepted[1].Sides["client"].MinSources != 90 {
		t.Fatalf("unexpected accepted catalog: %#v", accepted)
	}
}

func TestReadmeCommandWritesAndChecksGeneratedTable(t *testing.T) {
	root := t.TempDir()
	version := config.Version{
		ID: "1.0", Family: "1.0", Java: 8, Naming: "mcp",
		Mapping: &config.Mapping{Format: "tiny-v1", Tool: "legacy"},
		Sides:   map[string]config.Validation{"client": {MinSources: 1, MinSymbols: 1}},
	}
	writeVersions(t, filepath.Join(root, "versions.json"), []config.Version{version})
	readme := "before\n<!-- BEGIN GENERATED SUPPORTED VERSIONS -->\nstale\n<!-- END GENERATED SUPPORTED VERSIONS -->\nafter\n"
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(readme), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), []string{"readme", "--config", "versions.json", "--file", "README.md", "--write"}, &stdout, &stderr, runDependencies{Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"readme", "--config", "versions.json", "--file", "README.md", "--check"}, &stdout, &stderr, runDependencies{Root: root}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "| 1.0 | `1.0` | 8 | Pinned MCP mappings | client |") {
		t.Fatalf("unexpected README: %s", data)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(readme), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"readme", "--config", "versions.json", "--file", "README.md", "--check"}, &stdout, &stderr, runDependencies{Root: root}); err == nil || !strings.Contains(err.Error(), "out of date") {
		t.Fatalf("got %v", err)
	}
}

func writeVersions(t *testing.T, path string, versions []config.Version) {
	t.Helper()
	if err := config.WriteVersionFile(path, versions); err != nil {
		t.Fatal(err)
	}
}

func writeReport(t *testing.T, path string, report pipeline.CompatibilityReport) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func jsonResponse(value string) *http.Response {
	return jsonBytesResponse([]byte(value))
}

func jsonBytesResponse(value []byte) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(value)),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	cloned := request.Clone(request.Context())
	cloned.URL = new(url.URL)
	*cloned.URL = *request.URL
	return roundTrip(cloned)
}
