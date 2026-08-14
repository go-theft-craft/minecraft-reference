package buildcheck

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

func TestVersionUpdaterWorkflowIsSafe(t *testing.T) {
	repository := filepath.Join("..", "..")
	path := filepath.Join(repository, ".github", "workflows", "update-minecraft-versions.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)

	if !strings.Contains(workflow, "permissions:\n  contents: read\n") {
		t.Error("updater workflow must default to read-only repository contents")
	}
	for _, forbidden := range []string{
		"pull_request_target:",
		"secrets.",
		"task release",
		"goreleaser release",
		"gh release",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("updater workflow contains forbidden text %q", forbidden)
		}
	}
	if !strings.Contains(workflow, "GH_TOKEN: ${{ github.token }}") {
		t.Error("updater workflow must pass the GitHub token to gh through GH_TOKEN")
	}
	for lineNumber, line := range strings.Split(workflow, "\n") {
		if strings.Contains(line, "github.token") && strings.TrimSpace(line) != "GH_TOKEN: ${{ github.token }}" {
			t.Errorf("GitHub token appears outside the GH_TOKEN environment at line %d", lineNumber+1)
		}
	}

	pinnedAction := regexp.MustCompile(`^[0-9a-f]{40}(?:\s+#.*)?$`)
	for lineNumber, line := range strings.Split(workflow, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "uses:") && !strings.HasPrefix(trimmed, "- uses:") {
			continue
		}
		reference, found := strings.CutPrefix(trimmed, "- ")
		if found {
			trimmed = reference
		}
		_, version, found := strings.Cut(trimmed, "@")
		if !found || !pinnedAction.MatchString(version) {
			t.Errorf("workflow action is not SHA-pinned at line %d: %s", lineNumber+1, strings.TrimSpace(line))
		}
	}
}
