package blocks_test

import (
	"strings"
	"testing"

	"github.com/go-theft-craft/minecraft-reference/internal/reference/blocks"
)

func TestAPackedStateResolvesToItsBlock(t *testing.T) {
	t.Parallel()

	// The whole encoding is this shift, and getting it wrong does not fail —
	// it resolves every lookup and answers about the wrong block. Stone is
	// block 1, so its states run from 16 through 31.
	for _, state := range []uint32{16, 17, 31} {
		if got := blocks.BlockID(state); got != 1 {
			t.Errorf("state %d resolved to block %d, want 1", state, got)
		}
	}
	if got := blocks.BlockID(0); got != 0 {
		t.Errorf("state 0 resolved to block %d, want air at 0", got)
	}
}

func TestADocumentIsSortedAndStable(t *testing.T) {
	t.Parallel()

	// Two runs of the dumper must produce the same bytes, or every consumer
	// sees a diff whenever the registry happens to iterate differently.
	document := blocks.Document{
		Version:       "1.8.9",
		StateEncoding: blocks.StateEncodingChunk47,
		Blocks: []blocks.Block{
			{ID: 9, Name: "minecraft:water"},
			{ID: 1, Name: "minecraft:stone", BlocksMovement: true},
		},
	}

	raw, err := document.MarshalCanonical()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if stone, water := strings.Index(string(raw), "stone"), strings.Index(string(raw), "water"); stone > water {
		t.Error("blocks are not sorted by id")
	}

	again, err := document.MarshalCanonical()
	if err != nil {
		t.Fatalf("marshal again: %v", err)
	}
	if string(raw) != string(again) {
		t.Error("marshalling twice produced different bytes")
	}
}

func TestADuplicateBlockIsRejected(t *testing.T) {
	t.Parallel()

	// Two rows for one id means one of them is silently ignored by whichever
	// consumer builds a map last-wins, and which one wins is not stated
	// anywhere.
	_, err := blocks.ParseDocument([]byte(`{
		"version":"1.8.9","side":"server","jarSha256":"",
		"stateEncoding":"id<<4|meta",
		"blocks":[{"id":1,"name":"a","blocksMovement":true},
		          {"id":1,"name":"b","blocksMovement":false}]}`))
	if err == nil {
		t.Fatal("accepted a document with a duplicate block id")
	}
	if !strings.Contains(err.Error(), "more than once") {
		t.Errorf("error is %q, want it to name the duplicate", err)
	}
}

func TestAnUnknownFieldIsRejected(t *testing.T) {
	t.Parallel()

	// A field this version does not know is either a newer document or a typo,
	// and both are worth stopping for rather than silently dropping.
	_, err := blocks.ParseDocument([]byte(`{
		"version":"1.8.9","side":"server","jarSha256":"",
		"stateEncoding":"id<<4|meta","blocks":[],"fullCube":true}`))
	if err == nil {
		t.Fatal("accepted a document with an unknown field")
	}
}
