package physics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// prepareWorkspace writes what a prepared workspace holds for one version: the
// jar the dumper compiles against, Mojang's own jar for provenance, and the
// compatibility report that says which of the two is which.
//
// The naming decides the analysis jar's name. A version remapped from mappings
// leaves named.jar; a version Mojang ships under its own names leaves the
// unpacked executable.jar and no mapping step at all.
func prepareWorkspace(t *testing.T, root, version, naming string) string {
	t.Helper()

	side := filepath.Join(root, "versions", version, "server")
	if err := os.MkdirAll(side, 0o750); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	name := "named.jar"
	if naming == "identity" {
		name = "executable.jar"
	}
	jar := filepath.Join(side, name)
	if err := os.WriteFile(jar, []byte("not a real jar"), 0o600); err != nil {
		t.Fatalf("write stub jar: %v", err)
	}
	if err := os.WriteFile(filepath.Join(side, "original.jar"),
		[]byte("not a real mojang jar"), 0o600); err != nil {
		t.Fatalf("write stub original jar: %v", err)
	}

	report := fmt.Sprintf(`{"naming":%q,"passed":true}`, naming)
	if err := os.WriteFile(filepath.Join(side, "compatibility.json"), []byte(report), 0o600); err != nil {
		t.Fatalf("write compatibility report: %v", err)
	}

	return jar
}

func TestTheAnalysisJarFollowsHowTheVersionWasNamed(t *testing.T) {
	// Guessing between the two would be worse than failing: the obfuscated jar
	// compiles nothing, and a run against the wrong file is a confusing compile
	// error rather than a clear missing-workspace one.
	root := t.TempDir()

	want := prepareWorkspace(t, root, "1.8.9", "mcp")
	got, err := analysisJarPath(root, "1.8.9", "server")
	if err != nil {
		t.Fatalf("analysisJarPath: %v", err)
	}
	if got != want || !strings.HasSuffix(got, "named.jar") {
		t.Fatalf("analysisJarPath = %q, want the remapped %q", got, want)
	}

	want = prepareWorkspace(t, root, "26.1.2", "identity")
	got, err = analysisJarPath(root, "26.1.2", "server")
	if err != nil {
		t.Fatalf("analysisJarPath: %v", err)
	}
	if got != want || !strings.HasSuffix(got, "executable.jar") {
		t.Fatalf("analysisJarPath = %q, want the shipped %q", got, want)
	}
}

func TestAWorkspaceThatDidNotPassValidationIsRefused(t *testing.T) {
	root := t.TempDir()
	prepareWorkspace(t, root, "26.1.2", "identity")

	side := filepath.Join(root, "versions", "26.1.2", "server")
	if err := os.WriteFile(filepath.Join(side, "compatibility.json"),
		[]byte(`{"naming":"identity","passed":false}`), 0o600); err != nil {
		t.Fatalf("write compatibility report: %v", err)
	}

	if _, err := analysisJarPath(root, "26.1.2", "server"); err == nil {
		t.Fatal("a workspace that failed validation was accepted")
	}
}

func TestAWorkspaceWithNoCompatibilityReportSaysSo(t *testing.T) {
	if _, err := analysisJarPath(t.TempDir(), "26.1.2", "server"); err == nil {
		t.Fatal("a workspace with no compatibility report was accepted")
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
		t.Fatal("Dump accepted a workspace with no prepared jar")
	}
}

func TestDumpWritesDocumentFromStubbedJava(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub toolchain uses a POSIX shell script")
	}
	root := t.TempDir()
	prepareWorkspace(t, root, "1.8.9", "mcp")

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

	// The recorded digest must be Mojang's published jar, not the local remap.
	wantDigest := sha256.Sum256([]byte("not a real mojang jar"))
	if document.JarSHA256 != hex.EncodeToString(wantDigest[:]) {
		t.Fatalf("jarSha256 = %q, want the original jar digest", document.JarSHA256)
	}
}

func TestOriginalJarPath(t *testing.T) {
	got := originalJarPath(filepath.Join("reference", "work"), "1.8.9", "server")
	want := filepath.Join("reference", "work", "versions", "1.8.9", "server", "original.jar")
	if got != want {
		t.Fatalf("originalJarPath = %q, want %q", got, want)
	}
}

func TestDumpRejectsMissingOriginalJar(t *testing.T) {
	root := t.TempDir()
	prepareWorkspace(t, root, "1.8.9", "mcp")
	if err := os.Remove(originalJarPath(root, "1.8.9", "server")); err != nil {
		t.Fatalf("remove the original jar: %v", err)
	}

	err := Dump(context.Background(), Options{
		ReferenceDir: root,
		Version:      "1.8.9",
		Side:         "server",
		Output:       filepath.Join(t.TempDir(), "physics.json"),
	})
	if err == nil {
		t.Fatal("Dump accepted a workspace with no original jar")
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

func TestAVersionWithNoDumperNamesWhatIsImplemented(t *testing.T) {
	// An error that says only what cannot be done leaves the reader to guess
	// what can. Both implemented versions have to appear, so adding a third
	// without listing it fails here.
	err := Dump(context.Background(), Options{
		ReferenceDir: t.TempDir(),
		Version:      "1.7.10",
		Side:         "server",
		Output:       filepath.Join(t.TempDir(), "physics.json"),
	})
	if err == nil {
		t.Fatal("Dump accepted a version with no dumper")
	}
	for _, want := range []string{"1.7.10", "1.8.9", "26.1.2"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}
}

func TestDumpingTwentySixWithoutAPreparedJarFailsClearly(t *testing.T) {
	// Not a panic and not an empty document: a missing workspace is the most
	// common way this is run wrong, and it has to say so.
	err := Dump(context.Background(), Options{
		ReferenceDir: t.TempDir(),
		Version:      "26.1.2",
		Side:         "server",
		Output:       filepath.Join(t.TempDir(), "physics.json"),
	})
	if err == nil {
		t.Fatal("Dump accepted a workspace with no prepared 26.1.2 jar")
	}
	if !strings.Contains(err.Error(), "compatibility report") {
		t.Errorf("the error does not name what is missing: %v", err)
	}
}

func TestDumpingTwentySixWritesADocument(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub toolchain uses a POSIX shell script")
	}
	root := t.TempDir()
	// identity naming: this version is compiled against the jar Mojang ships.
	prepareWorkspace(t, root, "26.1.2", "identity")

	payload := `{"defaultSlipperiness":0.6,"blockSlipperiness":{"ice":0.98,"slime":0.8},` +
		`"sinTableBase64":"AAAAAA=="}`
	javac := writeStubScript(t, root, "javac", "")
	java := writeStubScript(t, root, "java", payload)

	output := filepath.Join(root, "physics.json")
	if err := Dump(context.Background(), Options{
		ReferenceDir: root,
		Version:      "26.1.2",
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
	if document.Version != "26.1.2" || document.Side != "server" {
		t.Fatalf("identity not stamped: %+v", document)
	}
	if document.BlockSlipperiness["slime"] != 0.8 {
		t.Fatalf("slipperiness not carried through: %+v", document.BlockSlipperiness)
	}
}

func TestEveryDumperNamesItsOwnEntryPoint(t *testing.T) {
	// javac requires the file name to match the public class, and Dump writes
	// the source to ClassName+".java". A mismatch compiles nothing and the
	// error names a file rather than the mistake.
	for version, program := range dumpers {
		if !strings.Contains(program.Source, "public final class "+program.ClassName+" {") {
			t.Errorf("%s declares an entry point other than %q", version, program.ClassName)
		}
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

// TestTheTwentySixDumperNamesItsReviewedIdentities keeps the embedded program
// tied to the symbols someone actually checked against the jar.
//
// A renamed identity fails at javac here, because this dumper is typed — which
// is how the registry key type turning from ResourceLocation into Identifier was
// caught. This test covers the two it still reaches by reflection, where javac
// cannot help: a wrong field name there compiles and then throws after a minute
// of bootstrapping.
func TestTheTwentySixDumperNamesItsReviewedIdentities(t *testing.T) {
	for _, identity := range []string{
		"net.minecraft.core.registries.BuiltInRegistries",
		"net.minecraft.resources.Identifier",
		"net.minecraft.server.Bootstrap",
		"net.minecraft.util.Mth",
		"net.minecraft.world.entity.ai.attributes.DefaultAttributes",
		"net.minecraft.world.level.block.state.BlockBehaviour",
		// The two reflective reads.
		`getDeclaredField("SIN")`,
		`getDeclaredField("friction")`,
		// Registries are empty until the class that fills them is touched.
		`Class.forName("net.minecraft.world.level.block.Blocks")`,
		// The document is written through a stream captured before the game
		// installs a logger over System.out.
		"PrintStream out = System.out;",
	} {
		if !strings.Contains(dump261Source, identity) {
			t.Errorf("the 26.1.2 dumper no longer names the reviewed identity %q", identity)
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
