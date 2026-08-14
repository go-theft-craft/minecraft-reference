package buildcheck

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/catalog"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
)

func TestRestrictedReferenceArtifactsAreNotTracked(t *testing.T) {
	command := exec.Command("git", "ls-files", "-z")
	command.Dir = filepath.Join("..", "..")
	data, err := command.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	for _, raw := range bytes.Split(data, []byte{0}) {
		path := string(raw)
		if path == "" {
			continue
		}
		if strings.HasPrefix(filepath.ToSlash(path), "reference/work/") {
			t.Errorf("restricted workspace file is tracked: %s", path)
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jar", ".class", ".java", ".tiny", ".tsrg", ".srg", ".csrg":
			t.Errorf("restricted artifact is tracked: %s", path)
		}
	}
}

func TestCustomReferenceGeneratedPathsAreIgnored(t *testing.T) {
	repository := filepath.Join("..", "..")
	tests := []struct {
		path        string
		wantIgnored bool
	}{
		{path: "custom/reference/cache/tools/mcp.zip", wantIgnored: true},
		{path: "custom/reference/versions/1.10.2/mappings/fields.csv", wantIgnored: true},
		{path: "custom/reference/versions/1.20.6/mappings/server.txt", wantIgnored: true},
		{path: "custom/reference/docs/server.txt", wantIgnored: false},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			command := exec.Command("git", "check-ignore", "--quiet", "--", test.path)
			command.Dir = repository
			err := command.Run()
			if test.wantIgnored {
				if err != nil {
					t.Fatalf("expected %q to be ignored: %v", test.path, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected %q not to be ignored", test.path)
			}
			exitError, ok := err.(*exec.ExitError)
			if !ok || exitError.ExitCode() != 1 {
				t.Fatalf("git check-ignore %q: %v", test.path, err)
			}
		})
	}
}

func TestReadmeSupportedVersionsMatchTrackedCatalog(t *testing.T) {
	repository := filepath.Join("..", "..")
	versions, err := config.ReadVersionFile(filepath.Join(repository, "internal", "reference", "config", "defaults", "versions.json"))
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile(filepath.Join(repository, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.CheckREADME(readme, versions); err != nil {
		t.Fatal(err)
	}
}
