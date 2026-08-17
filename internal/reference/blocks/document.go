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

// Block is what one block does to something trying to occupy it.
//
// The facts are per block rather than per state because in this version they
// are. Material hangs off Block, not off IBlockState, so every state of one
// block blocks movement or does not together. A later version splits them, and
// a document for that version will have to be keyed by state; recording the
// encoding above is what lets a consumer tell which it is holding.
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

	return document, nil
}

// BlockID returns the block identifier a packed state refers to.
//
// It is here rather than left to each consumer because the shift is the whole
// of the encoding, and a consumer that gets it wrong gets plausible answers
// about the wrong blocks.
func BlockID(state uint32) int { return int(state >> 4) }
