package catalog

import (
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
)

func TestUpdateREADMEGeneratesStableReadableSupportTable(t *testing.T) {
	source := []byte("before\n" + readmeBeginMarker + "\nstale table\n" + readmeEndMarker + "\nafter\n")
	versions := []config.Version{
		{ID: "26.1.2", Family: "26.1", Java: 25, ReleaseDate: "2026-04-09", VerifiedDate: "2026-08-14", Naming: "identity", Sides: testSides("client", "server")},
		{ID: "1.10.2", Family: "1.10", Java: 8, ReleaseDate: "2016-06-23", VerifiedDate: "2026-08-14", Naming: "mcp", Mapping: &config.Mapping{Format: "srg-csv"}, Sides: testSides("server", "client")},
		{ID: "1.0", Family: "1.0", Java: 8, ReleaseDate: "2011-11-17", VerifiedDate: "2026-08-14", Naming: "mcp", Mapping: &config.Mapping{Format: "tiny-v1"}, Sides: testSides("client")},
		{ID: "1.14.4", Family: "1.14", Java: 8, ReleaseDate: "2019-07-19", VerifiedDate: "2026-08-14", Naming: "mojang", Sides: testSides("client", "server")},
	}

	updated, err := UpdateREADME(source, versions)
	if err != nil {
		t.Fatal(err)
	}
	want := "before\n" + readmeBeginMarker + `
| Family | Tested release | Released | Verified | Minimum JDK | Mapping source | Tested sides |
| --- | --- | --- | --- | ---: | --- | --- |
| 26.1 | ` + "`26.1.2`" + ` | 2026-04-09 | 2026-08-14 | 25 | Names distributed with the game | client and server |
| 1.14 | ` + "`1.14.4`" + ` | 2019-07-19 | 2026-08-14 | 17 | Mojang client and server mappings | client and server |
| 1.10 | ` + "`1.10.2`" + ` | 2016-06-23 | 2026-08-14 | 17 | Pinned MCP mappings | client and server |
| 1.0 | ` + "`1.0`" + ` | 2011-11-17 | 2026-08-14 | 17 | Pinned MCP mappings | client |
` + readmeEndMarker + "\nafter\n"
	if string(updated) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", updated, want)
	}
	if !strings.HasPrefix(string(updated), "before\n") || !strings.HasSuffix(string(updated), "after\n") {
		t.Fatalf("text outside markers changed: %q", updated)
	}
}

func TestCheckREADMERejectsStaleGeneratedContent(t *testing.T) {
	versions := []config.Version{{ID: "1.0", Family: "1.0", Java: 8, Naming: "identity", Sides: testSides("client")}}
	stale := []byte(readmeBeginMarker + "\nstale\n" + readmeEndMarker + "\n")
	if err := CheckREADME(stale, versions); err == nil || !strings.Contains(err.Error(), "out of date") {
		t.Fatalf("got %v", err)
	}
	current, err := UpdateREADME(stale, versions)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckREADME(current, versions); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateREADMERequiresExactlyOneMarkerPair(t *testing.T) {
	for _, source := range []string{
		"no markers",
		readmeBeginMarker + "\n" + readmeEndMarker + "\n" + readmeEndMarker,
	} {
		if _, err := UpdateREADME([]byte(source), nil); err == nil {
			t.Fatalf("expected marker error for %q", source)
		}
	}
}

func testSides(names ...string) map[string]config.Validation {
	result := make(map[string]config.Validation, len(names))
	for _, name := range names {
		result[name] = config.Validation{MinSources: 1, MinSymbols: 1}
	}
	return result
}
