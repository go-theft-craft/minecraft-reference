package physics

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNamedJarPath(t *testing.T) {
	got := namedJarPath(filepath.Join("reference", "work"), "1.8.9", "server")
	want := filepath.Join("reference", "work", "versions", "1.8.9", "server", "named.jar")
	if got != want {
		t.Fatalf("namedJarPath = %q, want %q", got, want)
	}
}

func TestDumpRejectsMissingJar(t *testing.T) {
	err := Dump(context.Background(), Options{
		ReferenceDir: t.TempDir(),
		Version:      "1.8.9",
		Side:         "server",
		Output:       filepath.Join(t.TempDir(), "physics.json"),
	})
	if err == nil {
		t.Fatal("Dump accepted a missing named jar")
	}
}

func TestDumpWritesDocumentFromStubbedJava(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub toolchain uses a POSIX shell script")
	}
	root := t.TempDir()
	jar := namedJarPath(root, "1.8.9", "server")
	if err := os.MkdirAll(filepath.Dir(jar), 0o750); err != nil {
		t.Fatalf("create jar directory: %v", err)
	}
	if err := os.WriteFile(jar, []byte("not a real jar"), 0o600); err != nil {
		t.Fatalf("write stub jar: %v", err)
	}

	payload := `{"defaultSlipperiness":0.6,"blockSlipperiness":{"ice":0.98},"sinTableBase64":"AAAAAA=="}`
	javac := writeStubScript(t, root, "javac", "")
	java := writeStubScript(t, root, "java", payload)

	output := filepath.Join(root, "physics.json")
	if err := Dump(context.Background(), Options{
		ReferenceDir: root,
		Version:      "1.8.9",
		Side:         "server",
		Output:       output,
		Java:         java,
		Javac:        javac,
	}); err != nil {
		t.Fatalf("Dump: %v", err)
	}

	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	document, err := ParseDocument(raw)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if document.Version != "1.8.9" || document.Side != "server" {
		t.Fatalf("identity not stamped: %+v", document)
	}
	if document.BlockSlipperiness["ice"] != 0.98 {
		t.Fatalf("slipperiness not carried through: %+v", document.BlockSlipperiness)
	}
	if document.JarSHA256 == "" {
		t.Fatal("jar digest was not recorded")
	}
}

func TestDumpRejectsUnsupportedVersionAndSide(t *testing.T) {
	if err := Dump(context.Background(), Options{
		ReferenceDir: t.TempDir(),
		Version:      "1.7.10",
		Side:         "server",
		Output:       filepath.Join(t.TempDir(), "physics.json"),
	}); err == nil {
		t.Fatal("Dump accepted a version with no dumper")
	}

	if err := Dump(context.Background(), Options{
		ReferenceDir: t.TempDir(),
		Version:      "1.8.9",
		Side:         "client",
		Output:       filepath.Join(t.TempDir(), "physics.json"),
	}); err == nil {
		t.Fatal("Dump accepted an unsupported side")
	}

	if err := Dump(context.Background(), Options{
		ReferenceDir: t.TempDir(),
		Version:      "1.8.9",
		Side:         "server",
	}); err == nil {
		t.Fatal("Dump accepted an empty output path")
	}
}

func writeStubScript(t *testing.T, directory, name, stdout string) string {
	t.Helper()

	path := filepath.Join(directory, name)
	script := "#!/bin/sh\nprintf '%s' " + shellQuote(stdout) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write stub %s: %v", name, err)
	}

	return path
}

func shellQuote(value string) string {
	return "'" + value + "'"
}

// TestBuildClasspathFindsNestedLibraries pins the Maven-style layout that
// mcreference prepare writes: jars sit several directories below libraries/,
// not one.
func TestBuildClasspathFindsNestedLibraries(t *testing.T) {
	root := t.TempDir()
	libraries := librariesPath(root, "1.8.9")
	nested := filepath.Join(libraries, "com", "google", "guava", "guava", "17.0")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatalf("create nested library directory: %v", err)
	}
	jar := filepath.Join(nested, "guava-17.0.jar")
	if err := os.WriteFile(jar, []byte("stub"), 0o600); err != nil {
		t.Fatalf("write nested jar: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "guava-17.0.pom"), []byte("stub"), 0o600); err != nil {
		t.Fatalf("write non-jar sibling: %v", err)
	}

	classpath := buildClasspath("named.jar", libraries)

	entries := filepath.SplitList(classpath)
	if len(entries) != 2 {
		t.Fatalf("classpath has %d entries, want the jar plus one library: %q", len(entries), classpath)
	}
	if entries[0] != "named.jar" {
		t.Fatalf("classpath does not lead with the named jar: %q", classpath)
	}
	if entries[1] != jar {
		t.Fatalf("classpath entry = %q, want the nested jar %q", entries[1], jar)
	}
}

// TestDumperSourceHasNoUnreplacedTokens guards the substitution step: an
// identity left as its placeholder compiles fine and then fails at runtime with
// ClassNotFoundException against a real jar.
func TestDumperSourceHasNoUnreplacedTokens(t *testing.T) {
	for _, token := range []string{
		"BLOCK_OWNER",
		"BLOCK_REGISTRY_FIELD",
		"SLIPPERINESS_FIELD",
		"REGISTRY_NAME_METHOD",
		"MATH_HELPER_OWNER",
		"SIN_TABLE_FIELD",
		"BOOTSTRAP_OWNER",
		"BOOTSTRAP_METHOD",
	} {
		if strings.Contains(dump18Source, token) {
			t.Errorf("dumper source still contains the placeholder %q", token)
		}
	}
}

// TestDumperSourceDeclaresTheCompiledClass keeps the embedded program and the
// class name Dump runs in agreement.
func TestDumperSourceDeclaresTheCompiledClass(t *testing.T) {
	if !strings.Contains(dump18Source, "public final class Dump1_8") {
		t.Fatal("dumper source does not declare Dump1_8, the class Dump invokes")
	}
	for _, identity := range []string{
		"net.minecraft.block.Block",
		"blockRegistry",
		"slipperiness",
		"getNameForObject",
		"net.minecraft.util.MathHelper",
		"SIN_TABLE",
		"net.minecraft.init.Bootstrap",
	} {
		if !strings.Contains(dump18Source, identity) {
			t.Errorf("dumper source no longer names the reviewed identity %q", identity)
		}
	}
}

func TestBuildClasspathToleratesMissingLibraries(t *testing.T) {
	classpath := buildClasspath("named.jar", filepath.Join(t.TempDir(), "absent"))

	if classpath != "named.jar" {
		t.Fatalf("classpath = %q, want just the named jar", classpath)
	}
}
