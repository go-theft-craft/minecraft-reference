package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	err := run([]string{"accept", "--config", configPath, "--reports", reportsPath, "--output", outputPath}, &stdout, &stderr)
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
	err := run([]string{"accept", "--config", configPath, "--reports", reportsPath, "--output", outputPath}, &stdout, &stderr)
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
		err := run(arguments, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), missing+" is required") {
			t.Fatalf("missing %s: got %v", missing, err)
		}
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
