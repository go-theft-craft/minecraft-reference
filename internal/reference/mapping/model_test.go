package mapping

import (
	"bytes"
	"testing"
)

func TestWriteSRGRejectsEmptyObjectNameSegments(t *testing.T) {
	descriptors := []string{
		"L/foo;",
		"Lfoo/;",
		"Lfoo//Bar;",
	}
	for _, descriptor := range descriptors {
		t.Run(descriptor, func(t *testing.T) {
			value := Mapping{
				Classes: []Class{{Source: "a", Target: "example/Owner"}},
				Fields:  []Field{{Owner: "a", Descriptor: descriptor, Source: "b", Target: "field"}},
			}
			if err := WriteSRG(&bytes.Buffer{}, value); err == nil {
				t.Fatalf("WriteSRG accepted invalid descriptor %q", descriptor)
			}
		})
	}
}

func TestRemapDescriptorAcceptsPackagesInnerClassesArraysAndPrimitives(t *testing.T) {
	classes := map[string]string{
		"net/minecraft/Outer$Inner": "a$b",
	}
	descriptor := "([Lnet/minecraft/Outer$Inner;Ljava/lang/String;[I)Z"

	got, err := remapDescriptor(descriptor, classes, true)
	if err != nil {
		t.Fatal(err)
	}
	want := "([La$b;Ljava/lang/String;[I)Z"
	if got != want {
		t.Fatalf("remapDescriptor() = %q, want %q", got, want)
	}
}
