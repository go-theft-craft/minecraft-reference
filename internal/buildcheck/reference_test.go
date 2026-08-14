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
	workflow := readVersionUpdaterWorkflow(t)

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

func TestVersionUpdaterUsesOneImmutableBaseCommit(t *testing.T) {
	workflow := readVersionUpdaterWorkflow(t)

	if !strings.Contains(workflow, "base_sha: ${{ steps.base.outputs.sha }}") {
		t.Error("discovery must publish its checked-out base commit")
	}
	const immutableCheckout = "ref: ${{ needs.discovery.outputs.base_sha }}"
	if count := strings.Count(workflow, immutableCheckout); count != 2 {
		t.Errorf("candidate testing and acceptance must check out the discovery commit: got %d immutable checkouts", count)
	}
	if !strings.Contains(workflow, `install -m 0600 internal/reference/config/defaults/tools.json "$config_dir/tools.json"`) {
		t.Error("candidate testing must pair candidate versions with tools.json from the immutable checkout")
	}
	for _, required := range []string{
		`remote_main_sha="$(git ls-remote --heads origin refs/heads/main | cut -f1)"`,
		`remote_staging_sha="$(git ls-remote --heads origin "refs/heads/$STAGING_BRANCH" | cut -f1)"`,
		`staging_parent_sha="$(git rev-parse HEAD^)"`,
		`needs.acceptance.outputs.staging_sha`,
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("workflow does not verify immutable promotion identity %q", required)
		}
	}
}

func TestVersionUpdaterSerializesAutomationBranches(t *testing.T) {
	workflow := readVersionUpdaterWorkflow(t)

	if !strings.Contains(workflow, "group: ${{ github.workflow }}-automation-minecraft-versions") {
		t.Error("updater concurrency group must be shared by every workflow ref")
	}
	if strings.Contains(workflow, "group: ${{ github.workflow }}-${{ github.ref }}") {
		t.Error("updater concurrency must not split runs by workflow ref")
	}
}

func TestVersionUpdaterMatchesOnlyRepositoryOwnedPullRequest(t *testing.T) {
	workflow := readVersionUpdaterWorkflow(t)

	for _, required := range []string{
		`gh api --method GET "repos/$GITHUB_REPOSITORY/pulls"`,
		`"head=$GITHUB_REPOSITORY_OWNER:$UPDATE_BRANCH"`,
		`.head.repo.full_name`,
		`.head.ref`,
		`.head.sha`,
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("repository-owned pull request lookup is missing %q", required)
		}
	}
	if strings.Contains(workflow, `gh pr list --head "$UPDATE_BRANCH"`) {
		t.Error("branch-only pull request lookup can select a fork pull request")
	}
}

func readVersionUpdaterWorkflow(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", ".github", "workflows", "update-minecraft-versions.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
