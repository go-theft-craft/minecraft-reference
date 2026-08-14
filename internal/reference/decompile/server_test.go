package decompile

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestExecutableServerExtractsVerifiedBundledJar(t *testing.T) {
	directory := t.TempDir()
	bundle := filepath.Join(directory, "bundle.jar")
	executable := []byte("executable server")
	if err := createBundledServerFixture(bundle, "server.jar", executable); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(directory, "executable.jar")
	gotPath, err := ExecutableServer(bundle, destination)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, executable) {
		t.Fatalf("got %q, want %q", got, executable)
	}
}

func TestExecutableServerUsesPlainJar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.jar")
	if err := os.WriteFile(path, []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecutableServer(path, filepath.Join(t.TempDir(), "unused.jar")); err == nil {
		t.Fatal("expected invalid jar error")
	}
}

func createBundledServerFixture(path, executableName string, executable []byte) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	digest := sha256.Sum256(executable)
	manifest, err := writer.Create(versionsList)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(manifest, "%x\tfixture\t%s\n", digest, executableName); err != nil {
		return err
	}
	entry, err := writer.Create("META-INF/versions/" + executableName)
	if err != nil {
		return err
	}
	if _, err := entry.Write(executable); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return file.Close()
}
