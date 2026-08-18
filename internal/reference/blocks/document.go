// Package blocks extracts and encodes the block facts a consumer needs to
// decide whether a position can be occupied.
//
// It exists because nothing else supplies them. A protocol carries block state
// identifiers and says nothing about what a state means, and a world model that
// stores what the server sent is right to refuse to interpret them. Somewhere
// between the two, a caller that wants to walk has to learn that stone stops it
// and a flower does not, and that answer comes from the game's own code or it
// is a guess.
package blocks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// StateEncodingChunk47 names how protocol 47 packs a block state where a
// consumer meets one.
//
// It is recorded in the document rather than assumed, because 1.8.9 packs the
// same fact two different ways and choosing wrong fails silently. Chunk section
// data carries a state as the block identifier shifted left four with the
// metadata in the low bits, which is what a client reads out of terrain and
// what this document is keyed against. The game's own Block.getStateId packs it
// the other way round — the identifier in the low twelve bits with the metadata
// above it — and a table keyed that way looks correct, resolves every lookup,
// and answers about the wrong block every time.
const StateEncodingChunk47 = "id<<4|meta"

// StateEncodingRegistry775 names how protocol 775 identifies a block state.
//
// There is no arithmetic here at all, which is the difference worth recording.
// A state identifier is an index into the game's own registry of every possible
// state, assigned in registration order, and nothing about it can be shifted or
// masked back into a block identifier. A consumer holding one of these has to
// look it up in the ranges this document carries; a consumer that applies the
// protocol 47 shift to it gets a plausible number for an unrelated block.
const StateEncodingRegistry775 = "block-state-registry"

// StateRange is the span of state identifiers one block owns.
//
// The states of a block are contiguous, so a range says which states belong to
// it in two numbers rather than in the several thousand a per-state table would
// need. That the states really are contiguous is checked rather than assumed:
// ParseDocument rejects ranges that overlap or leave a gap, because either one
// silently answers about the wrong block.
type StateRange struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// StateException is one state that disagrees with the rest of its block.
//
// It exists for a single block in 26.1.2 and is carried anyway, because that
// block is the reason this document cannot be keyed by block. Every wall in the
// game is registered forceSolidOn, which settles the answer for all of its
// states at once — except resin_brick_wall, which is not, so its states fall
// through to the collision shape and the unconnected ones come back empty. A
// document that recorded one answer per block would state the wrong one for
// those states and nothing downstream could tell.
type StateException struct {
	State          int  `json:"state"`
	BlocksMovement bool `json:"blocksMovement"`
}

// Block is what one block does to something trying to occupy it.
//
// Whether the facts are per block or per state depends on the version, which is
// what StateEncoding distinguishes. Under StateEncodingChunk47 they are per
// block: Material hangs off Block, not off IBlockState, so every state of one
// block answers together and neither StateRange nor StateExceptions is
// carried. Under StateEncodingRegistry775 the answer hangs off the state, so
// every block carries the range of states it owns, and the rare block whose
// states disagree carries the ones that differ.
type Block struct {
	// ID is the numeric block identifier, which is a chunk state identifier
	// shifted right four.
	ID int `json:"id"`
	// Name is the registry name, kept for the reader rather than the program.
	// A table of numbers nobody can check is a table nobody will correct.
	Name string `json:"name"`
	// BlocksMovement reports that this block stops an entity walking into it.
	// It is the material's own answer, which is the one the game uses: air,
	// water, plants, and torches do not block, and stone does.
	//
	// It is the only fact here a program reads, and that is deliberate. An
	// earlier draft also carried whether the block fills its cell, on the
	// argument that a slab is something to step onto rather than walk around.
	// Nothing asked for it: the one caller decides that from the clearance
	// above a block, and a movement kernel will need real collision shapes
	// rather than a boolean, so it was neither used now nor sufficient later.
	// A field in a published document is far harder to remove than to add.
	BlocksMovement bool `json:"blocksMovement"`
	// Falls reports that this block is pulled down when the block beneath it
	// is removed, which is the fact a route that digs has to know before it
	// digs.
	//
	// It is a class test — BlockFalling in 1.8.9, FallingBlock in 26.1.2 —
	// because that is where the game hangs the behaviour, and it is the only
	// answer either version states in one place. Material will not substitute:
	// soul sand shares Material.sand with gravel and does not fall, which is
	// the whole reason this is measured rather than derived.
	//
	// It is a class test and therefore describes exactly what the class
	// describes. 26.1.2 has one block that falls without extending it —
	// pointed dripstone, which breaks and drops through its own tick — and it
	// is reported here as not falling. Recording that is better than patching
	// it: a hand-maintained exception list beside a measurement is a guess
	// wearing the jar's hash, and the caller that needs dripstone can ask for
	// a second measurement rather than trust an edited one.
	//
	// It is per block in both versions measured so far. Unlike BlocksMovement
	// it hangs off the block rather than off one of its states, so it carries
	// no range and no exception even under StateEncodingRegistry775.
	Falls bool `json:"falls"`
	// Climbable reports that a body can climb this block's column — a ladder,
	// a vine, and in 26.1.2 seven more.
	//
	// Nothing in a collision shape says it. A ladder's box is empty, so a
	// consumer reading shapes alone cannot tell one from air, which is why the
	// fact is measured here rather than left to be inferred.
	//
	// The two versions state it in different places and both are read where
	// the game reads them. 1.8.9 has no tag system and names the two blocks
	// directly in EntityLivingBase.isOnLadder, so the dumper tests those two
	// classes. 26.1.2 asks whether the state is in the climbable block tag,
	// and that tag is a document in the jar rather than something a bootstrap
	// binds, so the dumper reads it out of the same jar the digest names.
	//
	// Per block in both versions, for the same reason Falls is.
	Climbable bool `json:"climbable"`
	// StateRange is the span of state identifiers this block owns, for a
	// version whose states are registry indices. It is absent for a version
	// whose state identifier is arithmetic on the block identifier.
	//
	// It is a pointer rather than a pair of plain integers because air owns
	// exactly state 0, and a range of zero to zero is indistinguishable from an
	// absent one in any encoding that omits empty values.
	StateRange *StateRange `json:"stateRange,omitempty"`
	// StateExceptions lists the states of this block that answer differently
	// from BlocksMovement. It is empty for all but one block in the versions
	// measured so far, and dropping it would make that block wrong.
	StateExceptions []StateException `json:"stateExceptions,omitempty"`
}

// Document is the extracted block record for one version.
type Document struct {
	Version   string `json:"version"`
	Side      string `json:"side"`
	JarSHA256 string `json:"jarSha256"`
	// StateEncoding names how to turn a state identifier into a Block.ID.
	StateEncoding string  `json:"stateEncoding"`
	Blocks        []Block `json:"blocks"`
}

// MarshalCanonical renders the document sorted by block identifier so that two
// runs of the dumper produce the same bytes.
func (d Document) MarshalCanonical() ([]byte, error) {
	if d.Blocks == nil {
		d.Blocks = []Block{}
	}
	sorted := make([]Block, len(d.Blocks))
	copy(sorted, d.Blocks)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	d.Blocks = sorted

	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal blocks document: %w", err)
	}

	return append(raw, '\n'), nil
}

// ParseDocument decodes and validates a blocks document.
func ParseDocument(raw []byte) (Document, error) {
	var document Document

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return Document{}, fmt.Errorf("parse blocks document: %w", err)
	}

	seen := make(map[int]struct{}, len(document.Blocks))
	for _, block := range document.Blocks {
		if block.ID < 0 {
			return Document{}, fmt.Errorf("block %q has negative id %d", block.Name, block.ID)
		}
		if _, duplicate := seen[block.ID]; duplicate {
			return Document{}, fmt.Errorf("block id %d appears more than once", block.ID)
		}
		seen[block.ID] = struct{}{}
	}

	if err := validateStates(document); err != nil {
		return Document{}, err
	}

	return document, nil
}

// validateStates checks the parts of a document that depend on how the version
// identifies a state.
//
// Both halves refuse a document that is merely inconsistent rather than
// malformed, because that is the failure this data has. A state table with a
// gap in it does not fail to load; it answers "unknown" for a handful of real
// blocks, and the consumer treats unknown as impassable, so a bot stops in
// front of nothing and no error is ever printed.
func validateStates(document Document) error {
	switch document.StateEncoding {
	case StateEncodingChunk47:
		// This encoding derives the block from the state by shifting, so a
		// range would be a second, redundant answer to the same question, and
		// two answers that can disagree are worse than one.
		for _, block := range document.Blocks {
			if block.StateRange != nil || len(block.StateExceptions) != 0 {
				return fmt.Errorf(
					"block %q carries state detail, which encoding %q derives by shifting instead",
					block.Name, document.StateEncoding,
				)
			}
		}

		return nil
	case StateEncodingRegistry775:
		return validateStateRanges(document)
	default:
		return fmt.Errorf("unknown state encoding %q", document.StateEncoding)
	}
}

func validateStateRanges(document Document) error {
	ranges := make([]Block, 0, len(document.Blocks))
	for _, block := range document.Blocks {
		if block.StateRange == nil {
			return fmt.Errorf("block %q carries no state range", block.Name)
		}
		if block.StateRange.From < 0 || block.StateRange.To < block.StateRange.From {
			return fmt.Errorf(
				"block %q has state range %d..%d",
				block.Name, block.StateRange.From, block.StateRange.To,
			)
		}
		for _, exception := range block.StateExceptions {
			if exception.State < block.StateRange.From || exception.State > block.StateRange.To {
				return fmt.Errorf(
					"block %q has an exception for state %d outside its range %d..%d",
					block.Name, exception.State, block.StateRange.From, block.StateRange.To,
				)
			}
			// An exception that agrees with its block says nothing and hides
			// the ones that do, so it is a mistake rather than a redundancy.
			if exception.BlocksMovement == block.BlocksMovement {
				return fmt.Errorf(
					"block %q has an exception for state %d that agrees with the block",
					block.Name, exception.State,
				)
			}
		}
		ranges = append(ranges, block)
	}

	sort.Slice(ranges, func(i, j int) bool { return ranges[i].StateRange.From < ranges[j].StateRange.From })

	// Every state the wire can carry has to land in exactly one range. A gap
	// reads as an unknown block and an overlap picks whichever range is
	// searched first, and neither announces itself.
	next := 0
	for _, block := range ranges {
		if block.StateRange.From != next {
			return fmt.Errorf(
				"state %d is not described: block %q starts its range at %d",
				next, block.Name, block.StateRange.From,
			)
		}
		next = block.StateRange.To + 1
	}

	return nil
}

// BlockID returns the block identifier a packed state refers to.
//
// It is here rather than left to each consumer because the shift is the whole
// of the encoding, and a consumer that gets it wrong gets plausible answers
// about the wrong blocks.
func BlockID(state uint32) int { return int(state >> 4) }
