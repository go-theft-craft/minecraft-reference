package catalog

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/pipeline"
)

func TestAcceptSetsFlooredNinetyPercentThresholdsForEverySide(t *testing.T) {
	versions := []config.Version{{
		ID: "1.8.9", Family: "1.8", Java: 8, Naming: "mcp",
		Mapping: &config.Mapping{Format: "srg-csv", SRGTool: "srg", NamesTool: "names"},
		Sides: map[string]config.Validation{
			"client": {MinSources: 1, MinSymbols: 1, RequiredClasses: []string{"Minecraft"}},
			"server": {MinSources: 1, MinSymbols: 1, RequiredClasses: []string{"MinecraftServer"}},
		},
	}}
	reports := []pipeline.CompatibilityReport{
		{Version: "1.8.9", Family: "1.8", Side: "server", JavaMajor: 25, JavapMajor: 25, Naming: "mcp", NamedClasses: 1, SourceRecords: 1, SymbolRecords: 10, RequiredClasses: []string{"MinecraftServer"}, Passed: true},
		{Version: "1.8.9", Family: "1.8", Side: "client", JavaMajor: 25, JavapMajor: 25, Naming: "mcp", NamedClasses: 100, SourceRecords: 101, SymbolRecords: 999, RequiredClasses: []string{"Minecraft"}, Passed: true},
	}

	accepted, err := Accept(versions, reports)
	if err != nil {
		t.Fatal(err)
	}
	client := accepted[0].Sides["client"]
	if client.MinSources != 90 || client.MinSymbols != 899 {
		t.Fatalf("unexpected client thresholds: %#v", client)
	}
	server := accepted[0].Sides["server"]
	if server.MinSources != 1 || server.MinSymbols != 9 {
		t.Fatalf("unexpected server thresholds: %#v", server)
	}
	if !reflect.DeepEqual(client.RequiredClasses, []string{"Minecraft"}) || !reflect.DeepEqual(server.RequiredClasses, []string{"MinecraftServer"}) {
		t.Fatalf("required classes changed: %#v", accepted[0].Sides)
	}
	if versions[0].Sides["client"].MinSources != 1 {
		t.Fatalf("Accept mutated input: %#v", versions)
	}
}

func TestAcceptRequiresPassingReportForEveryConfiguredSide(t *testing.T) {
	version := config.Version{
		ID: "26.1.2", Family: "26.1", Java: 25, Naming: "identity",
		Sides: map[string]config.Validation{
			"client": {MinSources: 1, MinSymbols: 1},
			"server": {MinSources: 1, MinSymbols: 1},
		},
	}
	report := pipeline.CompatibilityReport{
		Version: "26.1.2", Family: "26.1", Side: "client", JavaMajor: 25, JavapMajor: 25,
		Naming: "identity", NamedClasses: 1, SourceRecords: 10, SymbolRecords: 10, Passed: true,
	}

	_, err := Accept([]config.Version{version}, []pipeline.CompatibilityReport{report})
	if err == nil || !strings.Contains(err.Error(), "26.1.2") || !strings.Contains(err.Error(), "server") || !strings.Contains(err.Error(), "passing compatibility report") {
		t.Fatalf("got %v", err)
	}

	report.Side = "server"
	report.Passed = false
	clientReport := report
	clientReport.Side = "client"
	clientReport.Passed = true
	_, err = Accept([]config.Version{version}, []pipeline.CompatibilityReport{clientReport, report})
	if err == nil || !strings.Contains(err.Error(), "server") || !strings.Contains(err.Error(), "did not pass") {
		t.Fatalf("got %v", err)
	}
}

func TestAcceptRejectsReportForDifferentConfiguration(t *testing.T) {
	version := config.Version{
		ID: "1.8.9", Family: "1.8", Java: 8, Naming: "mcp",
		Sides: map[string]config.Validation{"client": {MinSources: 1, MinSymbols: 1, RequiredClasses: []string{"Minecraft"}}},
	}
	base := pipeline.CompatibilityReport{
		Version: "1.8.9", Family: "1.8", Side: "client", JavaMajor: 8, JavapMajor: 8,
		Naming: "mcp", NamedClasses: 1, SourceRecords: 10, SymbolRecords: 10, RequiredClasses: []string{"Minecraft"}, Passed: true,
	}
	tests := []struct {
		name   string
		change func(*pipeline.CompatibilityReport)
		want   string
	}{
		{name: "family", change: func(report *pipeline.CompatibilityReport) { report.Family = "1.7" }, want: "family"},
		{name: "naming", change: func(report *pipeline.CompatibilityReport) { report.Naming = "identity" }, want: "naming"},
		{name: "java", change: func(report *pipeline.CompatibilityReport) { report.JavaMajor = 7 }, want: "Java"},
		{name: "javap", change: func(report *pipeline.CompatibilityReport) { report.JavapMajor = 7 }, want: "javap"},
		{name: "required classes", change: func(report *pipeline.CompatibilityReport) { report.RequiredClasses = nil }, want: "required classes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := base
			test.change(&report)
			_, err := Accept([]config.Version{version}, []pipeline.CompatibilityReport{report})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want error containing %q", err, test.want)
			}
		})
	}
}

func TestAcceptDoesNotLowerExistingThresholds(t *testing.T) {
	version := config.Version{
		ID: "1.20.6", Family: "1.20", Java: 21, Naming: "mojang",
		Sides: map[string]config.Validation{
			"client": {MinSources: 100, MinSymbols: 200},
		},
	}
	report := pipeline.CompatibilityReport{
		Version: "1.20.6", Family: "1.20", Side: "client", JavaMajor: 21, JavapMajor: 21,
		Naming: "mojang", NamedClasses: 1, SourceRecords: 100, SymbolRecords: 200, Passed: true,
	}
	accepted, err := Accept([]config.Version{version}, []pipeline.CompatibilityReport{report})
	if err != nil {
		t.Fatal(err)
	}
	validation := accepted[0].Sides["client"]
	if validation.MinSources != 100 || validation.MinSymbols != 200 {
		t.Fatalf("lowered existing thresholds: %#v", validation)
	}
}
