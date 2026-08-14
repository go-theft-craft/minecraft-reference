package archive

import (
	"archive/zip"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestListClassPathsPreservesZIPSeparators(t *testing.T) {
	path := filepath.Join(t.TempDir(), "classes.jar")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, name := range []string{"net/minecraft/Minecraft.class", "net.minecraft.FalsePositive.class"} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte{0}); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	paths, err := ListClassPaths(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(paths, []string{"net/minecraft/Minecraft", "net.minecraft.FalsePositive"}) {
		t.Fatalf("unexpected class paths: %#v", paths)
	}
	classes, err := ListClasses(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(classes, []string{"net.minecraft.Minecraft", "net.minecraft.FalsePositive"}) {
		t.Fatalf("unexpected binary class names: %#v", classes)
	}
}
