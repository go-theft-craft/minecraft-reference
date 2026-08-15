package physics

import (
	"encoding/json"
	"math"
	"testing"
)

func TestMarshalCanonicalSortsBlockNames(t *testing.T) {
	document := Document{
		Version:             "1.8.9",
		Side:                "server",
		JarSHA256:           "abc123",
		DefaultSlipperiness: 0.6,
		BlockSlipperiness:   map[string]float64{"slime": 0.8, "ice": 0.98},
		SinTableBase64:      EncodeSinTable([]float32{0, 1}),
		EntityMotion: map[string]Motion{
			"player": {Gravity: 0.08, HorizontalDrag: 0.91, VerticalDrag: 0.98, StepHeight: 0.6},
		},
	}

	first, err := document.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical: %v", err)
	}
	second, err := document.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical second call: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("MarshalCanonical is not deterministic")
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatalf("unmarshal canonical output: %v", err)
	}
	for _, key := range []string{"version", "side", "jarSha256", "defaultSlipperiness", "blockSlipperiness", "sinTableBase64", "entityMotion"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("canonical output is missing key %q", key)
		}
	}
}

func TestParseDocumentRoundTrip(t *testing.T) {
	want := Document{
		Version:             "1.8.9",
		Side:                "server",
		JarSHA256:           "abc123",
		DefaultSlipperiness: 0.6,
		BlockSlipperiness:   map[string]float64{"ice": 0.98},
		SinTableBase64:      EncodeSinTable([]float32{0, 0.5, 1}),
		EntityMotion:        map[string]Motion{"arrow": {Gravity: 0.05, HorizontalDrag: 0.99, VerticalDrag: 0.99}},
	}
	raw, err := want.MarshalCanonical()
	if err != nil {
		t.Fatalf("MarshalCanonical: %v", err)
	}
	got, err := ParseDocument(raw)
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if got.Version != want.Version || got.BlockSlipperiness["ice"] != 0.98 {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if got.EntityMotion["arrow"].Gravity != 0.05 {
		t.Fatalf("entity motion round trip mismatch: %+v", got.EntityMotion)
	}
}

func TestParseDocumentRejectsUnknownFields(t *testing.T) {
	if _, err := ParseDocument([]byte(`{"version":"1.8.9","unexpected":1}`)); err == nil {
		t.Fatal("ParseDocument accepted an unknown field")
	}
}

func TestSinTableRoundTrip(t *testing.T) {
	values := make([]float32, 1024)
	for index := range values {
		values[index] = float32(math.Sin(float64(index) * math.Pi * 2 / 1024))
	}
	decoded, err := DecodeSinTable(EncodeSinTable(values))
	if err != nil {
		t.Fatalf("DecodeSinTable: %v", err)
	}
	if len(decoded) != len(values) {
		t.Fatalf("decoded length = %d, want %d", len(decoded), len(values))
	}
	for index := range values {
		if decoded[index] != values[index] {
			t.Fatalf("index %d = %v, want %v", index, decoded[index], values[index])
		}
	}
}

func TestDecodeSinTableRejectsPartialFloat(t *testing.T) {
	if _, err := DecodeSinTable("AAAA"); err == nil {
		t.Fatal("DecodeSinTable accepted a byte count that is not a multiple of four")
	}
}
