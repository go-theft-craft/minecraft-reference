package mapping

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseProGuardAndWriteSRG(t *testing.T) {
	input := strings.NewReader("net.minecraft.client.Minecraft -> a:\n" +
		"    net.minecraft.world.World world -> c\n" +
		"    void setWorld(net.minecraft.world.World,int[]) -> d\n" +
		"net.minecraft.world.World -> b:\n")

	parsed, err := ParseProGuard(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(parsed.Methods); got != 1 {
		t.Fatalf("parsed %d methods, want 1 non-constructor", got)
	}

	var output bytes.Buffer
	if err := WriteSRG(&output, parsed); err != nil {
		t.Fatal(err)
	}

	want := "CL: a net/minecraft/client/Minecraft\n" +
		"CL: b net/minecraft/world/World\n" +
		"FD: a/c net/minecraft/client/Minecraft/world\n" +
		"MD: a/d (Lb;[I)V net/minecraft/client/Minecraft/setWorld (Lnet/minecraft/world/World;[I)V\n"
	if got := output.String(); got != want {
		t.Fatalf("unexpected SRG output:\n%s\nwant:\n%s", got, want)
	}
}

func TestParseProGuardHandlesLineNumbersConstructorsTypesAndComments(t *testing.T) {
	input := strings.NewReader("# ProGuard mapping comment\n" +
		"net.minecraft.Outer$Inner -> a$b:\n" +
		"    1:2:void <init>(int):3:4 -> <init>\n" +
		"    5:6:void <clinit>():7:8 -> <clinit>\n" +
		"    int[] values -> c\n" +
		"    9:10:boolean check(byte,char,double,float,long,short,boolean,java.lang.String[],net.minecraft.Outer$Inner[][]):11 -> d\n")

	parsed, err := ParseProGuard(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(parsed.Methods); got != 1 {
		t.Fatalf("parsed %d methods, want 1 non-constructor", got)
	}

	var output bytes.Buffer
	if err := WriteSRG(&output, parsed); err != nil {
		t.Fatal(err)
	}

	want := "CL: a$b net/minecraft/Outer$Inner\n" +
		"FD: a$b/c net/minecraft/Outer$Inner/values\n" +
		"MD: a$b/d (BCDFJSZ[Ljava/lang/String;[[La$b;)Z net/minecraft/Outer$Inner/check (BCDFJSZ[Ljava/lang/String;[[Lnet/minecraft/Outer$Inner;)Z\n"
	if got := output.String(); got != want {
		t.Fatalf("unexpected SRG output:\n%s\nwant:\n%s", got, want)
	}
}

func TestParseProGuardRejectsInvalidInput(t *testing.T) {
	tests := map[string]string{
		"duplicate named class":      "one.A -> a:\none.A -> b:\n",
		"duplicate obfuscated class": "one.A -> a:\ntwo.A -> a:\n",
		"malformed class":            "one.A -> a\n",
		"malformed field type":       "one.A -> a:\n    void field -> b\n",
		"malformed method":           "one.A -> a:\n    void method(int -> b\n",
		"missing owner":              "    int field -> a\n",
		"short member":               "one.A -> a:\n    int field\n",
		"malformed array type":       "one.A -> a:\n    int[ field -> b\n",
	}

	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseProGuard(strings.NewReader(input)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
