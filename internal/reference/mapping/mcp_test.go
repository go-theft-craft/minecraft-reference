package mapping

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeMCPReplacesStableMemberNames(t *testing.T) {
	directory := t.TempDir()
	joined := filepath.Join(directory, "joined.srg")
	fields := filepath.Join(directory, "fields.csv")
	methods := filepath.Join(directory, "methods.csv")
	output := filepath.Join(directory, "stable.srg")
	mustWrite(t, joined, "CL: a net/minecraft/Example\nFD: a/a net/minecraft/Example/field_1_a\nMD: a/a (I)V net/minecraft/Example/func_2_a (I)V\n")
	mustWrite(t, fields, "searge,name,side,desc\nfield_1_a,health,2,\n")
	mustWrite(t, methods, "searge,name,side,desc\nfunc_2_a,tick,2,\n")
	if err := ComposeMCP(joined, fields, methods, output); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "net/minecraft/Example/health") || !strings.Contains(got, "net/minecraft/Example/tick") {
		t.Fatalf("unexpected mapping:\n%s", got)
	}
}

func mustWrite(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}
