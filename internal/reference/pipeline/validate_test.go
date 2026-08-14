package pipeline

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/index"
)

func TestValidateOutputRejectsMissingEvidence(t *testing.T) {
	tests := []struct {
		name       string
		classes    []string
		sources    []index.SourceFile
		symbols    []index.Symbol
		validation config.Validation
		want       []string
	}{
		{
			name:       "named Minecraft class",
			classes:    []string{"com/example/Minecraft.class"},
			sources:    []index.SourceFile{{Path: "net/minecraft/Minecraft.java"}},
			symbols:    []index.Symbol{validSymbol("net.minecraft.Minecraft", "1.8.9", "client")},
			validation: config.Validation{MinSources: 1, MinSymbols: 1},
			want:       []string{"version 1.8.9", "side client", "named classes", "observed 0", "required 1"},
		},
		{
			name:       "Minecraft source",
			classes:    []string{"net/minecraft/Minecraft.class"},
			sources:    []index.SourceFile{{Path: "com/example/Minecraft.java"}},
			symbols:    []index.Symbol{validSymbol("net.minecraft.Minecraft", "1.8.9", "client")},
			validation: config.Validation{MinSources: 1, MinSymbols: 1},
			want:       []string{"version 1.8.9", "side client", "Minecraft source records", "observed 0", "required 1"},
		},
		{
			name:       "Minecraft symbol owner",
			classes:    []string{"net/minecraft/Minecraft.class"},
			sources:    []index.SourceFile{{Path: "net/minecraft/Minecraft.java"}},
			symbols:    []index.Symbol{validSymbol("com.example.Minecraft", "1.8.9", "client")},
			validation: config.Validation{MinSources: 1, MinSymbols: 1},
			want:       []string{"version 1.8.9", "side client", "Minecraft symbol records", "observed 0", "required 1"},
		},
		{
			name:       "minimum sources",
			classes:    []string{"net/minecraft/Minecraft.class"},
			sources:    []index.SourceFile{{Path: "net/minecraft/Minecraft.java"}},
			symbols:    []index.Symbol{validSymbol("net.minecraft.Minecraft", "1.8.9", "client")},
			validation: config.Validation{MinSources: 2, MinSymbols: 1},
			want:       []string{"version 1.8.9", "side client", "source records", "observed 1", "required 2"},
		},
		{
			name:       "minimum symbols",
			classes:    []string{"net/minecraft/Minecraft.class"},
			sources:    []index.SourceFile{{Path: "net/minecraft/Minecraft.java"}},
			symbols:    []index.Symbol{validSymbol("net.minecraft.Minecraft", "1.8.9", "client")},
			validation: config.Validation{MinSources: 1, MinSymbols: 2},
			want:       []string{"version 1.8.9", "side client", "symbol records", "observed 1", "required 2"},
		},
		{
			name:       "required final class segment",
			classes:    []string{"net/minecraft/src/Game.class"},
			sources:    []index.SourceFile{{Path: "net/minecraft/Game.java"}},
			symbols:    []index.Symbol{validSymbol("net.minecraft.Game", "1.8.9", "client")},
			validation: config.Validation{MinSources: 1, MinSymbols: 1, RequiredClasses: []string{"Minecraft"}},
			want:       []string{"version 1.8.9", "side client", "required class Minecraft", "observed 0", "required 1"},
		},
		{
			name:       "required MinecraftServer",
			classes:    []string{"net/minecraft/server/GameServer.class"},
			sources:    []index.SourceFile{{Path: "net/minecraft/server/GameServer.java"}},
			symbols:    []index.Symbol{validSymbol("net.minecraft.server.GameServer", "1.8.9", "client")},
			validation: config.Validation{MinSources: 1, MinSymbols: 1, RequiredClasses: []string{"MinecraftServer"}},
			want:       []string{"version 1.8.9", "side client", "required class MinecraftServer", "observed 0", "required 1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newValidationFixture(t, test.classes, test.sources, test.symbols)
			_, err := validateOutput(validationOptions{
				Version: config.Version{ID: "1.8.9", Family: "1.8", Java: 8, Naming: "mcp"},
				Side:    "client", Validation: test.validation, NamedJar: fixture.jar,
				SourcesIndex: fixture.sources, SymbolsIndex: fixture.symbols,
				ReportPath: fixture.report, JavaMajor: 25, JavapMajor: 25,
			})
			if err == nil {
				t.Fatal("expected validation error")
			}
			for _, part := range test.want {
				if !strings.Contains(err.Error(), part) {
					t.Fatalf("error %q does not contain %q", err, part)
				}
			}
		})
	}
}

func TestValidateOutputRejectsMalformedSymbolFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*index.Symbol)
		want   string
	}{
		{name: "empty kind", mutate: func(symbol *index.Symbol) { symbol.Kind = "" }, want: "kind"},
		{name: "unknown kind", mutate: func(symbol *index.Symbol) { symbol.Kind = "property" }, want: "kind"},
		{name: "empty name", mutate: func(symbol *index.Symbol) { symbol.Name = "" }, want: "name"},
		{name: "malformed descriptor", mutate: func(symbol *index.Symbol) { symbol.Descriptor = "not-a-descriptor" }, want: "descriptor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			symbol := validSymbol("net.minecraft.Minecraft", "1.8.9", "client")
			test.mutate(&symbol)
			fixture := newValidationFixture(
				t,
				[]string{"net/minecraft/Minecraft.class"},
				[]index.SourceFile{{Path: "net/minecraft/Minecraft.java"}},
				[]index.Symbol{symbol, symbol},
			)
			_, err := validateOutput(validationOptions{
				Version: config.Version{ID: "1.8.9", Family: "1.8", Java: 8, Naming: "mcp"},
				Side:    "client", Validation: config.Validation{MinSources: 1, MinSymbols: 2},
				NamedJar: fixture.jar, SourcesIndex: fixture.sources, SymbolsIndex: fixture.symbols,
				ReportPath: fixture.report, JavaMajor: 25, JavapMajor: 25,
			})
			if err == nil || !strings.Contains(err.Error(), "symbol record 1") || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want malformed %s error", err, test.want)
			}
		})
	}
}

func TestValidateOutputRejectsRepeatedOwnerOnlySymbols(t *testing.T) {
	fixture := newValidationFixture(
		t,
		[]string{"net/minecraft/Foo.class"},
		[]index.SourceFile{{Path: "net/minecraft/Foo.java"}},
		[]index.Symbol{{Owner: "net.minecraft.Foo"}, {Owner: "net.minecraft.Foo"}},
	)
	_, err := validateOutput(validationOptions{
		Version: config.Version{ID: "1.8.9", Family: "1.8", Java: 8, Naming: "mcp"},
		Side:    "client", Validation: config.Validation{MinSources: 1, MinSymbols: 2},
		NamedJar: fixture.jar, SourcesIndex: fixture.sources, SymbolsIndex: fixture.symbols,
		ReportPath: fixture.report, JavaMajor: 25, JavapMajor: 25,
	})
	if err == nil || !strings.Contains(err.Error(), "symbol record 1") {
		t.Fatalf("got %v, want malformed owner-only symbol error", err)
	}
}

func TestValidateOutputRejectsSymbolFromDifferentRun(t *testing.T) {
	for _, field := range []string{"version", "side"} {
		t.Run(field, func(t *testing.T) {
			symbol := validSymbol("net.minecraft.Minecraft", "1.8.9", "client")
			if field == "version" {
				symbol.Version = "1.8.8"
			} else {
				symbol.Side = "server"
			}
			fixture := newValidationFixture(
				t,
				[]string{"net/minecraft/Minecraft.class"},
				[]index.SourceFile{{Path: "net/minecraft/Minecraft.java"}},
				[]index.Symbol{symbol},
			)
			_, err := validateOutput(validationOptions{
				Version: config.Version{ID: "1.8.9", Family: "1.8", Java: 8, Naming: "mcp"},
				Side:    "client", Validation: config.Validation{MinSources: 1, MinSymbols: 1},
				NamedJar: fixture.jar, SourcesIndex: fixture.sources, SymbolsIndex: fixture.symbols,
				ReportPath: fixture.report, JavaMajor: 25, JavapMajor: 25,
			})
			if err == nil || !strings.Contains(err.Error(), "symbol record 1") || !strings.Contains(err.Error(), field) {
				t.Fatalf("got %v, want mismatched %s error", err, field)
			}
		})
	}
}

func TestValidateOutputRejectsDottedMinecraftZIPPath(t *testing.T) {
	fixture := newValidationFixture(
		t,
		[]string{"net.minecraft.Minecraft.class"},
		[]index.SourceFile{{Path: "net/minecraft/Minecraft.java"}},
		[]index.Symbol{validSymbol("net.minecraft.Minecraft", "1.8.9", "client")},
	)
	_, err := validateOutput(validationOptions{
		Version: config.Version{ID: "1.8.9", Family: "1.8", Java: 8, Naming: "mcp"},
		Side:    "client", Validation: config.Validation{MinSources: 1, MinSymbols: 1, RequiredClasses: []string{"Minecraft"}},
		NamedJar: fixture.jar, SourcesIndex: fixture.sources, SymbolsIndex: fixture.symbols,
		ReportPath: fixture.report, JavaMajor: 25, JavapMajor: 25,
	})
	if err == nil || !strings.Contains(err.Error(), "named classes") || !strings.Contains(err.Error(), "observed 0") {
		t.Fatalf("got %v, want invalid ZIP class path error", err)
	}
}

func TestValidateOutputAcceptsSlashMinecraftZIPPath(t *testing.T) {
	fixture := newValidationFixture(
		t,
		[]string{"net/minecraft/Minecraft.class"},
		[]index.SourceFile{{Path: "net/minecraft/Minecraft.java"}},
		[]index.Symbol{validSymbol("net.minecraft.Minecraft", "1.8.9", "client")},
	)
	report, err := validateOutput(validationOptions{
		Version: config.Version{ID: "1.8.9", Family: "1.8", Java: 8, Naming: "mcp"},
		Side:    "client", Validation: config.Validation{MinSources: 1, MinSymbols: 1, RequiredClasses: []string{"Minecraft"}},
		NamedJar: fixture.jar, SourcesIndex: fixture.sources, SymbolsIndex: fixture.symbols,
		ReportPath: fixture.report, JavaMajor: 25, JavapMajor: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || report.NamedClasses != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestValidateOutputWritesDeterministicPrivacySafeReport(t *testing.T) {
	fixture := newValidationFixture(
		t,
		[]string{"net/minecraft/src/Minecraft.class", "net/minecraft/world/World.class", "com/example/Other.class"},
		[]index.SourceFile{{Path: "net/minecraft/src/Minecraft.java"}, {Path: "net/minecraft/world/World.java"}},
		[]index.Symbol{validSymbol("net.minecraft.src.Minecraft", "1.8.9", "client"), validSymbol("net.minecraft.world.World", "1.8.9", "client")},
	)
	options := validationOptions{
		Version: config.Version{ID: "1.8.9", Family: "1.8", Java: 8, Naming: "mcp"},
		Side:    "client", Validation: config.Validation{MinSources: 2, MinSymbols: 2, RequiredClasses: []string{"Minecraft"}},
		NamedJar: fixture.jar, SourcesIndex: fixture.sources, SymbolsIndex: fixture.symbols,
		ReportPath: fixture.report, JavaMajor: 25, JavapMajor: 24,
	}
	report, err := validateOutput(options)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || report.NamedClasses != 2 || report.SourceRecords != 2 || report.SymbolRecords != 2 {
		t.Fatalf("unexpected report: %#v", report)
	}
	first, err := os.ReadFile(fixture.report)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validateOutput(options); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(fixture.report)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("report changed between identical runs:\n%s\n%s", first, second)
	}
	want := "" +
		"{\n" +
		"  \"version\": \"1.8.9\",\n" +
		"  \"family\": \"1.8\",\n" +
		"  \"side\": \"client\",\n" +
		"  \"java_major\": 25,\n" +
		"  \"javap_major\": 24,\n" +
		"  \"naming\": \"mcp\",\n" +
		"  \"named_classes\": 2,\n" +
		"  \"source_records\": 2,\n" +
		"  \"symbol_records\": 2,\n" +
		"  \"required_classes\": [\n" +
		"    \"Minecraft\"\n" +
		"  ],\n" +
		"  \"passed\": true\n" +
		"}\n"
	if string(first) != want {
		t.Fatalf("report mismatch:\n%s", first)
	}
	if strings.Contains(string(first), filepath.Dir(fixture.report)) || strings.Contains(string(first), "http://") || strings.Contains(string(first), "https://") {
		t.Fatalf("report contains private or remote location: %s", first)
	}
}

func TestValidateOutputStreamsLargeIndexRecords(t *testing.T) {
	largeDeclaration := strings.Repeat("x", 256<<10)
	fixture := newValidationFixture(
		t,
		[]string{"net/minecraft/server/MinecraftServer.class"},
		[]index.SourceFile{{Path: "net/minecraft/server/MinecraftServer.java"}},
		[]index.Symbol{{Version: "26.1.2", Side: "server", Owner: "net.minecraft.server.MinecraftServer", Kind: "method", Name: "tick", Descriptor: "()V", Declaration: largeDeclaration}},
	)
	_, err := validateOutput(validationOptions{
		Version: config.Version{ID: "26.1.2", Family: "26.1", Java: 25, Naming: "identity"},
		Side:    "server", Validation: config.Validation{MinSources: 1, MinSymbols: 1, RequiredClasses: []string{"MinecraftServer"}},
		NamedJar: fixture.jar, SourcesIndex: fixture.sources, SymbolsIndex: fixture.symbols,
		ReportPath: fixture.report, JavaMajor: 25, JavapMajor: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateOutputRemovesStaleReportBeforeFailure(t *testing.T) {
	fixture := newValidationFixture(
		t,
		[]string{"com/example/Game.class"},
		[]index.SourceFile{{Path: "net/minecraft/Game.java"}},
		[]index.Symbol{validSymbol("net.minecraft.Game", "1.8.9", "client")},
	)
	if err := os.MkdirAll(filepath.Dir(fixture.report), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.report, []byte(`{"passed":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := validateOutput(validationOptions{
		Version: config.Version{ID: "1.8.9", Family: "1.8", Java: 8, Naming: "mcp"},
		Side:    "client", Validation: config.Validation{MinSources: 1, MinSymbols: 1},
		NamedJar: fixture.jar, SourcesIndex: fixture.sources, SymbolsIndex: fixture.symbols,
		ReportPath: fixture.report, JavaMajor: 25, JavapMajor: 25,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if _, statErr := os.Stat(fixture.report); !os.IsNotExist(statErr) {
		t.Fatalf("stale report remains after failed validation: %v", statErr)
	}
}

func validSymbol(owner, version, side string) index.Symbol {
	return index.Symbol{Version: version, Side: side, Owner: owner, Kind: "method", Name: "tick", Descriptor: "()V", Declaration: "public void tick();"}
}

type validationFixture struct {
	jar, sources, symbols, report string
}

func newValidationFixture(t *testing.T, classes []string, sources []index.SourceFile, symbols []index.Symbol) validationFixture {
	t.Helper()
	directory := t.TempDir()
	fixture := validationFixture{
		jar: filepath.Join(directory, "named.jar"), sources: filepath.Join(directory, "sources.jsonl"),
		symbols: filepath.Join(directory, "symbols.jsonl"), report: filepath.Join(directory, "compatibility.json"),
	}
	file, err := os.Create(fixture.jar)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, name := range classes {
		entry, createErr := writer.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := entry.Write([]byte{0}); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	writeJSONLines(t, fixture.sources, sources)
	writeJSONLines(t, fixture.symbols, symbols)
	return fixture
}

func writeJSONLines[T any](t *testing.T, path string, records []T) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		data, marshalErr := json.Marshal(record)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if _, writeErr := file.Write(append(data, '\n')); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
