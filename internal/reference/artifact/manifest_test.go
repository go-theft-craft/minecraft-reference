package artifact

import "testing"

func TestDecodeVersionFillsLibraryURL(t *testing.T) {
	resolver := Resolver{}
	metadata, err := resolver.DecodeVersion([]byte(`{
  "id":"1.8.9",
  "downloads":{"client":{"sha1":"abc","size":1,"url":"https://example/client.jar"}},
  "libraries":[{"name":"example:test:1","downloads":{"artifact":{"path":"example/test/1/test-1.jar","sha1":"def","size":2,"url":""}}}]
}`), "1.8.9")
	if err != nil {
		t.Fatal(err)
	}
	got := metadata.Libraries[0].Downloads.Artifact.URL
	want := "https://libraries.minecraft.net/example/test/1/test-1.jar"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDecodeVersionRejectsWrongID(t *testing.T) {
	_, err := (Resolver{}).DecodeVersion([]byte(`{"id":"other"}`), "1.8.9")
	if err == nil {
		t.Fatal("expected id mismatch")
	}
}

func TestLibraryDetectsNativeClassifierCoordinate(t *testing.T) {
	if !(Library{Name: "org.lwjgl:lwjgl:3.4.1:natives-linux"}).HasClassifier() {
		t.Fatal("expected native classifier")
	}
	if !(Library{Name: "io.netty:netty-transport-native-epoll:4.2.7.Final:linux-x86_64"}).HasClassifier() {
		t.Fatal("expected platform classifier")
	}
	if (Library{Name: "org.lwjgl:lwjgl:3.4.1"}).HasClassifier() {
		t.Fatal("plain Java artifact detected as native")
	}
}
