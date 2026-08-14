package index

import "testing"

func TestParseJavapPreservesDescriptorsAndOverloads(t *testing.T) {
	data := `Compiled from "Example.java"
public class net.minecraft.Example {
  private int health;
    descriptor: I
  public net.minecraft.Example();
    descriptor: ()V
  public void tick(int);
    descriptor: (I)V
  public void tick(double);
    descriptor: (D)V
  static {};
    descriptor: ()V
}`
	symbols, err := ParseJavap(data, "1.8.9", "client")
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 5 {
		t.Fatalf("got %d symbols: %#v", len(symbols), symbols)
	}
	if symbols[1].Kind != "constructor" || symbols[1].Name != "<init>" {
		t.Fatalf("unexpected constructor: %#v", symbols[1])
	}
	if symbols[2].Descriptor == symbols[3].Descriptor {
		t.Fatalf("overload descriptors collapsed: %#v", symbols)
	}
	if symbols[4].Name != "<clinit>" {
		t.Fatalf("unexpected initializer: %#v", symbols[4])
	}
}

func TestParseJavapAcceptsPackagePrivateInterface(t *testing.T) {
	data := `interface net.minecraft.CoordinateSource {
  public abstract double getCoordinate(double, double, float, net.minecraft.util.RandomSource);
    descriptor: (DDFLnet/minecraft/util/RandomSource;)D
}`
	symbols, err := ParseJavap(data, "26.1.2", "server")
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 1 || symbols[0].Owner != "net.minecraft.CoordinateSource" {
		t.Fatalf("unexpected symbols: %#v", symbols)
	}
}
