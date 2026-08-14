package mapping

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseTinyV1AndWriteSRG(t *testing.T) {
	input := strings.NewReader("v1\tofficial\tnamed\n" +
		"CLASS\ta\tnet/minecraft/client/Minecraft\n" +
		"CLASS\tb\tnet/minecraft/world/World\n" +
		"FIELD\ta\tLb;\tc\tworld\n" +
		"METHOD\ta\t(Lb;[I)V\td\tsetWorld\n")

	parsed, err := ParseTinyV1(input)
	if err != nil {
		t.Fatal(err)
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

func TestParseTinyV1SkipsConstructorsAndSortsOutput(t *testing.T) {
	input := strings.NewReader("v1\tofficial\tnamed\n" +
		"CLASS\tb\tnet/minecraft/world/World\n" +
		"CLASS\ta\tnet/minecraft/client/Minecraft\n" +
		"METHOD\ta\t()V\t<clinit>\t<clinit>\n" +
		"METHOD\ta\t()V\t<init>\t<init>\n" +
		"METHOD\ta\t(I)V\tz\tlast\n" +
		"METHOD\ta\t([Lb;Z)I\td\tfirst\n" +
		"FIELD\ta\tI\tz\tlastField\n" +
		"FIELD\ta\t[Lb;\tc\tfirstField\n")

	parsed, err := ParseTinyV1(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(parsed.Methods); got != 2 {
		t.Fatalf("parsed %d methods, want 2 non-constructors", got)
	}

	var first bytes.Buffer
	if err := WriteSRG(&first, parsed); err != nil {
		t.Fatal(err)
	}
	var second bytes.Buffer
	if err := WriteSRG(&second, parsed); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Fatalf("output changed between writes:\nfirst:\n%s\nsecond:\n%s", first.String(), second.String())
	}

	want := "CL: a net/minecraft/client/Minecraft\n" +
		"CL: b net/minecraft/world/World\n" +
		"FD: a/c net/minecraft/client/Minecraft/firstField\n" +
		"FD: a/z net/minecraft/client/Minecraft/lastField\n" +
		"MD: a/d ([Lb;Z)I net/minecraft/client/Minecraft/first ([Lnet/minecraft/world/World;Z)I\n" +
		"MD: a/z (I)V net/minecraft/client/Minecraft/last (I)V\n"
	if got := first.String(); got != want {
		t.Fatalf("unexpected SRG output:\n%s\nwant:\n%s", got, want)
	}
}

func TestParseTinyV1RejectsInvalidInput(t *testing.T) {
	tests := map[string]string{
		"duplicate class":             "v1\tofficial\tnamed\nCLASS\ta\tone/A\nCLASS\ta\ttwo/A\n",
		"duplicate target":            "v1\tofficial\tnamed\nCLASS\ta\tone/A\nCLASS\tb\tone/A\n",
		"malformed field descriptor":  "v1\tofficial\tnamed\nCLASS\ta\tone/A\nFIELD\ta\tV\tb\tfield\n",
		"malformed method descriptor": "v1\tofficial\tnamed\nCLASS\ta\tone/A\nMETHOD\ta\t(I\tb\tmethod\n",
		"missing field owner":         "v1\tofficial\tnamed\nCLASS\ta\tone/A\nFIELD\tb\tI\tc\tfield\n",
		"missing method owner":        "v1\tofficial\tnamed\nCLASS\ta\tone/A\nMETHOD\tb\t()V\tc\tmethod\n",
		"short class record":          "v1\tofficial\tnamed\nCLASS\ta\n",
		"short field record":          "v1\tofficial\tnamed\nCLASS\ta\tone/A\nFIELD\ta\tI\tb\n",
		"short method record":         "v1\tofficial\tnamed\nCLASS\ta\tone/A\nMETHOD\ta\t()V\tb\n",
		"unknown namespace":           "v1\tintermediary\tnamed\nCLASS\ta\tone/A\n",
	}

	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseTinyV1(strings.NewReader(input)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
