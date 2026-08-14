package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestParseJavaMajor(t *testing.T) {
	tests := map[string]struct {
		output string
		want   int
	}{
		"legacy java":  {output: `java version "1.8.0_472"`, want: 8},
		"modern java":  {output: `openjdk version "25.0.4" 2026-04-21`, want: 25},
		"legacy javap": {output: "1.8.0_472\n", want: 8},
		"modern javap": {output: "25.0.4\n", want: 25},
		"noisy java":   {output: "Picked up JAVA_TOOL_OPTIONS: -Xmx2g\nopenjdk version \"21.0.8\"", want: 21},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := parseJavaMajor(test.output)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %d, want %d", got, test.want)
			}
		})
	}
}

func TestParseJavaMajorRejectsUnknownOutput(t *testing.T) {
	_, err := parseJavaMajor("unknown")
	if err == nil || !strings.Contains(err.Error(), "cannot parse") {
		t.Fatalf("got %v", err)
	}
}

func TestValidateJavaMajorRejectsOldTool(t *testing.T) {
	err := validateJavaMajor("javap", "/usr/bin/javap", 17, 21, "1.21.8")
	if err == nil || !strings.Contains(err.Error(), "Minecraft 1.21.8 requires Java 21 or newer") {
		t.Fatalf("got %v", err)
	}
}

func TestPreflightJavaAcceptsSymlinkedToolsFromOneJDK(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test fixtures use POSIX executable scripts")
	}
	root := t.TempDir()
	effectiveBin := filepath.Join(root, "jdk", "lib", "openjdk", "bin")
	java := writeJavaTool(t, effectiveBin, "java", 25)
	javap := writeJavaTool(t, effectiveBin, "javap", 25)
	if err := os.Symlink(filepath.Join("lib", "openjdk", "bin"), filepath.Join(root, "jdk", "bin")); err != nil {
		t.Fatal(err)
	}
	profileBin := filepath.Join(root, "profile", "bin")
	if err := os.MkdirAll(profileBin, 0o750); err != nil {
		t.Fatal(err)
	}
	javaLink := filepath.Join(profileBin, "java")
	javapLink := filepath.Join(profileBin, "javap")
	if err := os.Symlink(filepath.Join(root, "jdk", "bin", "java"), javaLink); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "jdk", "bin", "javap"), javapLink); err != nil {
		t.Fatal(err)
	}

	toolchain, err := preflightJava(context.Background(), javaLink, javapLink, 25, "26.2")
	if err != nil {
		t.Fatal(err)
	}
	if toolchain.javaPath != java || toolchain.javapPath != javap {
		t.Fatalf("did not resolve effective tools: %#v", toolchain)
	}
}

func TestPreflightJavaRejectsToolsFromDifferentJDKs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test fixtures use POSIX executable scripts")
	}
	root := t.TempDir()
	java := writeJavaTool(t, filepath.Join(root, "jdk-one", "bin"), "java", 25)
	javap := writeJavaTool(t, filepath.Join(root, "jdk-two", "bin"), "javap", 25)

	_, err := preflightJava(context.Background(), java, javap, 25, "26.2")
	if err == nil || !strings.Contains(err.Error(), "same JDK bin directory") {
		t.Fatalf("got %v", err)
	}
}

func writeJavaTool(t *testing.T, directory, name string, major int) string {
	t.Helper()
	if err := os.MkdirAll(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '"+strconv.Itoa(major)+".0.0\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
