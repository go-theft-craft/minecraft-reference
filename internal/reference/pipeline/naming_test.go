package pipeline

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/artifact"
	"github.com/go-theft-craft/minecraft-reference/internal/reference/config"
)

func TestPrepareNamedJarMCPWithTinyV1(t *testing.T) {
	tiny := []byte("v1\tofficial\tnamed\nCLASS\tb\tnet/minecraft/B\nCLASS\ta\tnet/minecraft/A\nFIELD\ta\tLb;\tc\tfield\nMETHOD\ta\t(Lb;)V\td\trun\n")
	specialSource := []byte("special source")
	downloads := map[string][]byte{
		"/mappings.tiny": tiny,
		"/special.jar":   specialSource,
	}
	downloader, requests := namingFixtureDownloader(t, downloads)
	root := t.TempDir()
	analysisJar := writeNamingFixture(t, root, "analysis.jar", []byte("analysis"))

	var gotInput, gotOutput, gotMapping string
	restore := stubRemap(t, func(_ context.Context, java, tool, input, output, mappingFile string) error {
		if java != "/jdk/bin/java" {
			t.Fatalf("java = %q", java)
		}
		if data, err := os.ReadFile(tool); err != nil || !bytes.Equal(data, specialSource) {
			t.Fatalf("special source = %q, %v", data, err)
		}
		gotInput, gotOutput, gotMapping = input, output, mappingFile
		return nil
	})
	defer restore()

	got, results, err := prepareNamedJar(context.Background(), namingOptions{
		Version: config.Version{
			ID:     "1.7.10",
			Naming: "mcp",
			Mapping: &config.Mapping{
				Format: "tiny-v1",
				Tool:   "mcp-1.7.10-tiny",
			},
			Sides: configuredSides("client"),
		},
		Side:         "client",
		AnalysisJar:  analysisJar,
		VersionDir:   filepath.Join(root, "versions", "1.7.10"),
		ReferenceDir: root,
		Java:         "/jdk/bin/java",
		Tools: map[string]config.Tool{
			"mcp-1.7.10-tiny":      namingFixtureTool("mcp-1.7.10-tiny", "https://raw.githubusercontent.com/mappings.tiny", tiny),
			"specialsource-1.11.4": namingFixtureTool("specialsource-1.11.4", "https://repo.maven.apache.org/special.jar", specialSource),
		},
		Downloader: downloader,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantJar := filepath.Join(root, "versions", "1.7.10", "client", "named.jar")
	if got != wantJar || gotInput != analysisJar || gotOutput != wantJar {
		t.Fatalf("named jar = %q, remap input = %q, output = %q", got, gotInput, gotOutput)
	}
	mappingData, err := os.ReadFile(gotMapping)
	if err != nil {
		t.Fatal(err)
	}
	wantMapping := "CL: a net/minecraft/A\nCL: b net/minecraft/B\nFD: a/c net/minecraft/A/field\nMD: a/d (Lb;)V net/minecraft/A/run (Lnet/minecraft/B;)V\n"
	if string(mappingData) != wantMapping {
		t.Fatalf("mapping:\n%s\nwant:\n%s", mappingData, wantMapping)
	}
	assertArtifactRequests(t, requests, "/mappings.tiny", "/special.jar")
	if len(results) != 2 {
		t.Fatalf("got %d artifact results, want 2", len(results))
	}
}

func TestPrepareNamedJarMCPWithSRGCSV(t *testing.T) {
	joined := zipNamingFixture(t, map[string]string{
		"joined.srg": "CL: a net/minecraft/client/Minecraft\nFD: a/b net/minecraft/client/Minecraft/field_1_b\nMD: a/c ()V net/minecraft/client/Minecraft/func_2_c ()V\n",
	})
	names := zipNamingFixture(t, map[string]string{
		"fields.csv":  "searge,name\nfield_1_b,displayWidth\n",
		"methods.csv": "searge,name\nfunc_2_c,runTick\n",
	})
	specialSource := []byte("special source")
	downloader, requests := namingFixtureDownloader(t, map[string][]byte{
		"/joined.zip":  joined,
		"/names.zip":   names,
		"/special.jar": specialSource,
	})
	root := t.TempDir()
	analysisJar := writeNamingFixture(t, root, "analysis.jar", []byte("analysis"))

	var gotMapping string
	restore := stubRemap(t, func(_ context.Context, _, _, _, _, mappingFile string) error {
		gotMapping = mappingFile
		return nil
	})
	defer restore()

	_, results, err := prepareNamedJar(context.Background(), namingOptions{
		Version: config.Version{
			ID:     "1.8.9",
			Naming: "mcp",
			Mapping: &config.Mapping{
				Format:    "srg-csv",
				SRGTool:   "mcp-1.8.9-srg",
				NamesTool: "mcp-stable-22-1.8.9",
			},
			Sides: configuredSides("server"),
		},
		Side:         "server",
		AnalysisJar:  analysisJar,
		VersionDir:   filepath.Join(root, "versions", "1.8.9"),
		ReferenceDir: root,
		Java:         "java",
		Tools: map[string]config.Tool{
			"mcp-1.8.9-srg":        namingFixtureTool("mcp-1.8.9-srg", "https://mcp.zeith.org/joined.zip", joined),
			"mcp-stable-22-1.8.9":  namingFixtureTool("mcp-stable-22-1.8.9", "https://mcp.zeith.org/names.zip", names),
			"specialsource-1.11.4": namingFixtureTool("specialsource-1.11.4", "https://repo.maven.apache.org/special.jar", specialSource),
		},
		Downloader: downloader,
	})
	if err != nil {
		t.Fatal(err)
	}
	mappingData, err := os.ReadFile(gotMapping)
	if err != nil {
		t.Fatal(err)
	}
	want := "CL: a net/minecraft/client/Minecraft\nFD: a/b net/minecraft/client/Minecraft/displayWidth\nMD: a/c ()V net/minecraft/client/Minecraft/runTick ()V\n"
	if string(mappingData) != want {
		t.Fatalf("mapping:\n%s\nwant:\n%s", mappingData, want)
	}
	assertArtifactRequests(t, requests, "/joined.zip", "/names.zip", "/special.jar")
	if len(results) != 3 {
		t.Fatalf("got %d artifact results, want 3", len(results))
	}
}

func TestPrepareNamedJarUsesSideSpecificMojangMappings(t *testing.T) {
	for _, side := range []string{"client", "server"} {
		t.Run(side, func(t *testing.T) {
			mappingData := []byte("net.minecraft." + side + ".Main -> a:\n    void run() -> b\n")
			downloader, requests := namingFixtureDownloader(t, map[string][]byte{
				"/" + side + ".txt": mappingData,
				"/special.jar":      []byte("special source"),
			})
			root := t.TempDir()
			analysisJar := writeNamingFixture(t, root, "analysis.jar", []byte("analysis"))
			var gotMapping string
			restore := stubRemap(t, func(_ context.Context, _, _, _, _, mappingFile string) error {
				gotMapping = mappingFile
				return nil
			})
			defer restore()

			digest := sha1.Sum(mappingData)
			got, results, err := prepareNamedJar(context.Background(), namingOptions{
				Version:      config.Version{ID: "1.20.6", Naming: "mojang", Sides: configuredSides(side)},
				Side:         side,
				AnalysisJar:  analysisJar,
				VersionDir:   filepath.Join(root, "versions", "1.20.6"),
				ReferenceDir: root,
				Java:         "java",
				Tools: map[string]config.Tool{
					"specialsource-1.11.4": namingFixtureTool("specialsource-1.11.4", "https://repo.maven.apache.org/special.jar", []byte("special source")),
				},
				Metadata: artifact.VersionMetadata{Downloads: map[string]artifact.RemoteFile{
					side + "_mappings": {
						URL:  "https://piston-data.mojang.com/" + side + ".txt",
						Size: int64(len(mappingData)),
						SHA1: fmt.Sprintf("%x", digest),
					},
				}},
				Downloader: downloader,
			})
			if err != nil {
				t.Fatal(err)
			}
			wantJar := filepath.Join(root, "versions", "1.20.6", side, "named.jar")
			if got != wantJar {
				t.Fatalf("got %q, want %q", got, wantJar)
			}
			generated, err := os.ReadFile(gotMapping)
			if err != nil {
				t.Fatal(err)
			}
			wantMapping := "CL: a net/minecraft/" + side + "/Main\nMD: a/b ()V net/minecraft/" + side + "/Main/run ()V\n"
			if string(generated) != wantMapping {
				t.Fatalf("mapping:\n%s\nwant:\n%s", generated, wantMapping)
			}
			assertArtifactRequests(t, requests, "/"+side+".txt", "/special.jar")
			if len(results) != 2 {
				t.Fatalf("got %d artifact results, want 2", len(results))
			}
		})
	}
}

func TestPrepareNamedJarRequiresMojangMappingForSide(t *testing.T) {
	_, _, err := prepareNamedJar(context.Background(), namingOptions{
		Version:     config.Version{ID: "1.20.6", Naming: "mojang", Sides: configuredSides("client")},
		Side:        "client",
		AnalysisJar: filepath.Join(t.TempDir(), "analysis.jar"),
		Metadata: artifact.VersionMetadata{Downloads: map[string]artifact.RemoteFile{
			"server_mappings": {URL: "https://piston-data.mojang.com/server.txt", SHA1: "abc"},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "client_mappings") {
		t.Fatalf("got %v, want missing client_mappings error", err)
	}
}

func TestPrepareNamedJarIdentityReturnsAnalysisJar(t *testing.T) {
	analysisJar := filepath.Join(t.TempDir(), "analysis.jar")
	called := false
	restore := stubRemap(t, func(context.Context, string, string, string, string, string) error {
		called = true
		return nil
	})
	defer restore()

	got, results, err := prepareNamedJar(context.Background(), namingOptions{
		Version:     config.Version{ID: "26.1.2", Naming: "identity", Sides: configuredSides("server")},
		Side:        "server",
		AnalysisJar: analysisJar,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != analysisJar || len(results) != 0 || called {
		t.Fatalf("got jar %q, %d results, remap called %v", got, len(results), called)
	}
}

func TestPrepareNamedJarRejectsUnknownStrategy(t *testing.T) {
	_, _, err := prepareNamedJar(context.Background(), namingOptions{
		Version:     config.Version{ID: "test", Naming: "unknown", Sides: configuredSides("client")},
		Side:        "client",
		AnalysisJar: filepath.Join(t.TempDir(), "analysis.jar"),
	})
	if err == nil || !strings.Contains(err.Error(), `unsupported naming strategy "unknown"`) {
		t.Fatalf("got %v, want unsupported naming strategy error", err)
	}
}

func configuredSides(sides ...string) map[string]config.Validation {
	result := make(map[string]config.Validation, len(sides))
	for _, side := range sides {
		result[side] = config.Validation{MinSources: 1, MinSymbols: 1}
	}
	return result
}

func namingFixtureTool(id, source string, data []byte) config.Tool {
	digest := sha256.Sum256(data)
	return config.Tool{ID: id, URL: source, SHA256: fmt.Sprintf("%x", digest)}
}

func namingFixtureDownloader(t *testing.T, downloads map[string][]byte) (artifact.Downloader, map[string]int) {
	t.Helper()
	requests := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		data, ok := downloads[request.URL.Path]
		if !ok {
			http.NotFound(writer, request)
			return
		}
		requests[request.URL.Path]++
		_, _ = writer.Write(data)
	}))
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: namingRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		cloned := request.Clone(request.Context())
		cloned.URL = new(url.URL)
		*cloned.URL = *request.URL
		cloned.URL.Scheme = target.Scheme
		cloned.URL.Host = target.Host
		return server.Client().Transport.RoundTrip(cloned)
	})}
	return artifact.Downloader{Client: client}, requests
}

func writeNamingFixture(t *testing.T, root, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func zipNamingFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	for name, contents := range files {
		file, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(file, strings.NewReader(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func assertArtifactRequests(t *testing.T, got map[string]int, paths ...string) {
	t.Helper()
	if len(got) != len(paths) {
		t.Fatalf("got requests %v, want %v", got, paths)
	}
	for _, path := range paths {
		if got[path] != 1 {
			t.Errorf("request count for %s = %d, want 1", path, got[path])
		}
	}
}

func stubRemap(t *testing.T, stub func(context.Context, string, string, string, string, string) error) func() {
	t.Helper()
	original := remapJar
	remapJar = stub
	return func() { remapJar = original }
}

type namingRoundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip namingRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}
