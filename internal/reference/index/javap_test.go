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

func TestParseJavapRecognizesGenericClassConstructor(t *testing.T) {
	data := `Compiled from "PropertyEnum.java"
public class net.minecraft.block.properties.PropertyEnum<T extends java.lang.Enum<T>> {
  protected net.minecraft.block.properties.PropertyEnum(java.lang.String, java.lang.Class<T>);
    descriptor: (Ljava/lang/String;Ljava/lang/Class;)V
}`
	symbols, err := ParseJavap(data, "1.8.9", "client")
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 1 {
		t.Fatalf("got %d symbols: %#v", len(symbols), symbols)
	}
	if symbols[0].Owner != "net.minecraft.block.properties.PropertyEnum" || symbols[0].Kind != "constructor" || symbols[0].Name != "<init>" {
		t.Fatalf("unexpected generic constructor: %#v", symbols[0])
	}
	if err := ValidateSymbol(symbols[0], "1.8.9", "client"); err != nil {
		t.Fatalf("ValidateSymbol: %v", err)
	}
}

func TestValidateSymbolAcceptsGeneratedRecordKinds(t *testing.T) {
	for _, symbol := range []Symbol{
		{Version: "1.8.9", Side: "client", Owner: "net.minecraft.Game", Kind: "field", Name: "health", Descriptor: "I"},
		{Version: "1.8.9", Side: "client", Owner: "net.minecraft.Game", Kind: "method", Name: "tick", Descriptor: "()V"},
		{Version: "1.8.9", Side: "client", Owner: "net.minecraft.Game", Kind: "constructor", Name: "<init>", Descriptor: "(I)V"},
		{Version: "1.8.9", Side: "client", Owner: "net.minecraft.Game", Kind: "initializer", Name: "<clinit>", Descriptor: "()V"},
	} {
		if err := ValidateSymbol(symbol, "1.8.9", "client"); err != nil {
			t.Fatalf("ValidateSymbol(%#v): %v", symbol, err)
		}
	}
}

func TestValidateSymbolRejectsNonVoidConstructor(t *testing.T) {
	symbol := Symbol{Version: "1.8.9", Side: "client", Owner: "net.minecraft.Game", Kind: "constructor", Name: "<init>", Descriptor: "()I"}
	if err := ValidateSymbol(symbol, "1.8.9", "client"); err == nil {
		t.Fatal("ValidateSymbol accepted a non-void constructor")
	}
}
