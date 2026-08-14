package catalog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/artifact"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
)

func TestDiscoverSelectsNewestStableReleaseByReleaseTime(t *testing.T) {
	times := func(day int) time.Time { return time.Date(2026, time.August, day, 0, 0, 0, 0, time.UTC) }
	releases := []artifact.Release{
		{ID: "1.20.5", Type: "release", ReleaseTime: times(2)},
		{ID: "1.20.6", Type: "release", ReleaseTime: times(3)},
		{ID: "1.21", Type: "release", ReleaseTime: times(5)},
		{ID: "1.21.11", Type: "release", ReleaseTime: times(4)},
		{ID: "26.1", Type: "release", ReleaseTime: times(6)},
		{ID: "26.1.2", Type: "release", ReleaseTime: times(7)},
		{ID: "26.2", Type: "release", ReleaseTime: times(1)},
		{ID: "1.22-pre1", Type: "release", ReleaseTime: times(14)},
		{ID: "1.22-rc1", Type: "snapshot", ReleaseTime: times(14)},
		{ID: "1.22-snapshot", Type: "snapshot", ReleaseTime: times(14)},
		{ID: "a1.2.6", Type: "old_alpha", ReleaseTime: times(14)},
		{ID: "b1.7.3", Type: "old_beta", ReleaseTime: times(14)},
	}
	current := []config.Version{
		testVersion("1.20.5", "1.20", 21, "mojang"),
		testVersion("1.21.11", "1.21", 21, "mojang"),
		testVersion("26.1", "26.1", 25, "identity"),
	}
	resolved := make([]string, 0)
	candidates, err := Discover(context.Background(), releases, current, func(_ context.Context, release artifact.Release) (artifact.VersionMetadata, error) {
		resolved = append(resolved, release.ID)
		return artifact.VersionMetadata{
			ID:          release.ID,
			JavaVersion: artifact.JavaVersion{MajorVersion: 99},
			Downloads: map[string]artifact.RemoteFile{
				"client":          {URL: "client"},
				"server":          {URL: "server"},
				"client_mappings": {URL: "client mappings"},
				"server_mappings": {URL: "server mappings"},
			},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	wantResolved := []string{"1.20.6", "1.21", "26.1.2", "26.2"}
	if !reflect.DeepEqual(resolved, wantResolved) {
		t.Fatalf("resolved metadata for %v, want %v", resolved, wantResolved)
	}
	wantFamilies := []string{"1.20", "1.21", "26.1", "26.2"}
	wantVersions := []string{"1.20.6", "1.21", "26.1.2", "26.2"}
	wantReleaseDates := []string{"2026-08-03", "2026-08-05", "2026-08-07", "2026-08-01"}
	if len(candidates) != len(wantFamilies) {
		t.Fatalf("got %d candidates, want %d: %#v", len(candidates), len(wantFamilies), candidates)
	}
	for index, family := range wantFamilies {
		candidate := candidates[index]
		if candidate.Family != family || candidate.New.Family != family || candidate.New.ID != wantVersions[index] || candidate.New.Java != 99 || candidate.New.Naming != "mojang" || candidate.New.ReleaseDate != wantReleaseDates[index] {
			t.Errorf("candidate %d: %#v", index, candidate)
		}
		if !candidate.New.SupportsSide("client") || !candidate.New.SupportsSide("server") {
			t.Errorf("candidate %s sides: %#v", family, candidate.New.Sides)
		}
	}
	if candidates[3].Old != nil {
		t.Fatalf("new family has old representative: %#v", candidates[3].Old)
	}
}

func TestDiscoverUsesIdentityWhenRequestedMappingsAreMissing(t *testing.T) {
	old := testVersion("1.13.2", "1.13", 8, "mcp")
	old.Mapping = &config.Mapping{Format: "tiny-v1", Tool: "legacy"}
	old.Sides["client"] = config.Validation{
		MinSources:      10,
		MinSymbols:      20,
		RequiredClasses: []string{"LegacyMinecraft"},
	}
	release := artifact.Release{ID: "1.13.3", Type: "release", ReleaseTime: time.Now()}
	candidates, err := Discover(context.Background(), []artifact.Release{release}, []config.Version{old}, func(context.Context, artifact.Release) (artifact.VersionMetadata, error) {
		return artifact.VersionMetadata{
			ID:          release.ID,
			JavaVersion: artifact.JavaVersion{MajorVersion: 8},
			Downloads: map[string]artifact.RemoteFile{
				"client":          {URL: "client"},
				"server":          {URL: "server"},
				"client_mappings": {URL: "client mappings"},
			},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("got %#v", candidates)
	}
	if candidates[0].New.Naming != "identity" || candidates[0].New.Mapping != nil {
		t.Fatalf("automatic discovery generated legacy mappings: %#v", candidates[0].New)
	}
	validation := candidates[0].New.Sides["client"]
	if validation.MinSources != 1 || validation.MinSymbols != 1 {
		t.Fatalf("replacement retained old validation floors: %#v", validation)
	}
	if !reflect.DeepEqual(validation.RequiredClasses, old.Sides["client"].RequiredClasses) {
		t.Fatalf("replacement did not retain required classes: %#v", validation)
	}
}

func TestDiscoverDoesNotResolveUnchangedRepresentative(t *testing.T) {
	version := testVersion("1.20.6", "1.20", 21, "mojang")
	candidates, err := Discover(context.Background(), []artifact.Release{{
		ID: "1.20.6", Type: "release", ReleaseTime: time.Now(),
	}}, []config.Version{version}, func(context.Context, artifact.Release) (artifact.VersionMetadata, error) {
		t.Fatal("resolved metadata for unchanged representative")
		return artifact.VersionMetadata{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if candidates == nil || len(candidates) != 0 {
		t.Fatalf("got %#v, want non-nil empty candidate list", candidates)
	}
}

func TestReadCandidateFileRejectsCandidateThatDiffersFromProposedVersion(t *testing.T) {
	version := testVersion("1.20.6", "1.20", 21, "mojang")
	different := cloneVersion(version)
	different.Java = 99
	data, err := json.Marshal(CandidateFile{
		Versions:   []config.Version{version},
		Candidates: []Candidate{{Family: version.Family, New: different}},
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "candidate.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCandidateFile(path); err == nil || !strings.Contains(err.Error(), "absent from proposed versions") {
		t.Fatalf("got %v", err)
	}
}

func testVersion(id, family string, java int, naming string) config.Version {
	return config.Version{
		ID: id, Family: family, Java: java, Naming: naming,
		Sides: map[string]config.Validation{
			"client": {MinSources: 10, MinSymbols: 20, RequiredClasses: []string{"Minecraft"}},
			"server": {MinSources: 5, MinSymbols: 10, RequiredClasses: []string{"MinecraftServer"}},
		},
	}
}
