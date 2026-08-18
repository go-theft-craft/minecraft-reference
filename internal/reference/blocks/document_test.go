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

// registryDocument is a small stand-in for a 26.1.2 document: two blocks whose
// state ranges meet at the boundary, the second disagreeing with itself.
func registryDocument(t *testing.T, blocksJSON string) error {
	t.Helper()

	_, err := blocks.ParseDocument([]byte(`{
		"version":"26.1.2","side":"server","jarSha256":"",
		"stateEncoding":"block-state-registry",
		"blocks":` + blocksJSON + `}`))

	return err
}

func TestAStateRangeDocumentIsAccepted(t *testing.T) {
	t.Parallel()

	// The shape the 26.1.2 dumper produces: contiguous ranges from zero, and a
	// block carrying the states that answer differently from the rest of it.
	err := registryDocument(t, `[
		{"id":0,"name":"minecraft:air","blocksMovement":false,
		 "stateRange":{"from":0,"to":0}},
		{"id":1,"name":"minecraft:resin_brick_wall","blocksMovement":true,
		 "stateRange":{"from":1,"to":4},
		 "stateExceptions":[{"state":2,"blocksMovement":false}]}]`)
	if err != nil {
		t.Fatalf("rejected a well-formed document: %v", err)
	}
}

func TestAGapBetweenStateRangesIsRejected(t *testing.T) {
	t.Parallel()

	// A gap does not fail anything downstream. The missing states resolve to
	// unknown, the consumer reads unknown as impassable, and a bot stops in
	// front of a block nobody can name.
	err := registryDocument(t, `[
		{"id":0,"name":"minecraft:air","blocksMovement":false,"stateRange":{"from":0,"to":0}},
		{"id":1,"name":"minecraft:stone","blocksMovement":true,"stateRange":{"from":2,"to":2}}]`)
	if err == nil {
		t.Fatal("accepted a document with a state nothing describes")
	}
	if !strings.Contains(err.Error(), "state 1 is not described") {
		t.Errorf("error is %q, want it to name the undescribed state", err)
	}
}

func TestOverlappingStateRangesAreRejected(t *testing.T) {
	t.Parallel()

	// An overlap resolves to whichever range the consumer searches first,
	// which is not stated anywhere and answers about the wrong block.
	err := registryDocument(t, `[
		{"id":0,"name":"minecraft:air","blocksMovement":false,"stateRange":{"from":0,"to":4}},
		{"id":1,"name":"minecraft:stone","blocksMovement":true,"stateRange":{"from":3,"to":6}}]`)
	if err == nil {
		t.Fatal("accepted a document whose ranges overlap")
	}
}

func TestAnExceptionOutsideItsBlockIsRejected(t *testing.T) {
	t.Parallel()

	// An exception for a state the block does not own overrides a different
	// block's answer, which is the one mistake this format makes possible.
	err := registryDocument(t, `[
		{"id":0,"name":"minecraft:air","blocksMovement":false,"stateRange":{"from":0,"to":0},
		 "stateExceptions":[{"state":5,"blocksMovement":true}]}]`)
	if err == nil {
		t.Fatal("accepted an exception outside its block's range")
	}
	if !strings.Contains(err.Error(), "outside its range") {
		t.Errorf("error is %q, want it to say the exception is out of range", err)
	}
}

func TestAnExceptionThatAgreesIsRejected(t *testing.T) {
	t.Parallel()

	// An exception restating the block's own answer is noise that hides the
	// ones that matter, and a reader counting exceptions gets the wrong count.
	err := registryDocument(t, `[
		{"id":0,"name":"minecraft:stone","blocksMovement":true,"stateRange":{"from":0,"to":2},
		 "stateExceptions":[{"state":1,"blocksMovement":true}]}]`)
	if err == nil {
		t.Fatal("accepted an exception that agrees with its block")
	}
}

func TestStateDetailIsRejectedForAShiftedEncoding(t *testing.T) {
	t.Parallel()

	// Protocol 47 derives the block from the state by shifting, so a range is a
	// second answer to a question that already has one, and the two can differ.
	_, err := blocks.ParseDocument([]byte(`{
		"version":"1.8.9","side":"server","jarSha256":"",
		"stateEncoding":"id<<4|meta",
		"blocks":[{"id":1,"name":"minecraft:stone","blocksMovement":true,
		           "stateRange":{"from":16,"to":31}}]}`))
	if err == nil {
		t.Fatal("accepted a shifted-encoding document carrying state ranges")
	}
}

func TestAnUnknownStateEncodingIsRejected(t *testing.T) {
	t.Parallel()

	// An encoding nobody has taught this package to read cannot be validated,
	// and passing it through means the checks above silently do not run.
	_, err := blocks.ParseDocument([]byte(`{
		"version":"1.99","side":"server","jarSha256":"",
		"stateEncoding":"something-else","blocks":[]}`))
	if err == nil {
		t.Fatal("accepted a document with an unknown state encoding")
	}
}

func TestFallingAndClimbableSurviveARoundTrip(t *testing.T) {
	t.Parallel()

	// Both facts are per block in every version measured so far, so they ride
	// in the block record beside blocksMovement rather than in a state range.
	// The round trip is what proves the field names in the tag match the ones
	// ParseDocument accepts; DisallowUnknownFields turns a typo into a parse
	// failure, but only if something parses what was marshalled.
	document := blocks.Document{
		Version:       "1.8.9",
		Side:          "server",
		StateEncoding: blocks.StateEncodingChunk47,
		Blocks: []blocks.Block{
			{ID: 13, Name: "minecraft:gravel", BlocksMovement: true, Falls: true},
			{ID: 65, Name: "minecraft:ladder", Climbable: true},
			{ID: 88, Name: "minecraft:soul_sand", BlocksMovement: true},
		},
	}

	raw, err := document.MarshalCanonical()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	parsed, err := blocks.ParseDocument(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	byName := make(map[string]blocks.Block, len(parsed.Blocks))
	for _, block := range parsed.Blocks {
		byName[block.Name] = block
	}

	if !byName["minecraft:gravel"].Falls {
		t.Error("gravel does not fall")
	}
	// The one case that says why this is measured instead of derived from the
	// material: soul sand shares Material.sand with gravel and stays put.
	if byName["minecraft:soul_sand"].Falls {
		t.Error("soul sand falls")
	}
	if !byName["minecraft:ladder"].Climbable {
		t.Error("a ladder is not climbable")
	}
	if byName["minecraft:gravel"].Climbable {
		t.Error("gravel is climbable")
	}
}
