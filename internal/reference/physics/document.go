// Package physics extracts and encodes Minecraft physics constants.
package physics

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
)

// Motion records the per-tick motion constants for one entity family.
type Motion struct {
	Gravity        float64 `json:"gravity"`
	HorizontalDrag float64 `json:"horizontalDrag"`
	VerticalDrag   float64 `json:"verticalDrag"`
	StepHeight     float64 `json:"stepHeight"`
}

// Document is the complete extracted and transcribed physics record for one version.
type Document struct {
	Version             string             `json:"version"`
	Side                string             `json:"side"`
	JarSHA256           string             `json:"jarSha256"`
	DefaultSlipperiness float64            `json:"defaultSlipperiness"`
	BlockSlipperiness   map[string]float64 `json:"blockSlipperiness"`
	SinTableBase64      string             `json:"sinTableBase64"`
	EntityMotion        map[string]Motion  `json:"entityMotion"`
}

// MarshalCanonical renders the document with sorted keys and a stable layout.
func (d Document) MarshalCanonical() ([]byte, error) {
	if d.BlockSlipperiness == nil {
		d.BlockSlipperiness = map[string]float64{}
	}
	if d.EntityMotion == nil {
		d.EntityMotion = map[string]Motion{}
	}

	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal physics document: %w", err)
	}

	return append(raw, '\n'), nil
}

// ParseDocument decodes and validates a physics document.
func ParseDocument(raw []byte) (Document, error) {
	var document Document

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return Document{}, fmt.Errorf("parse physics document: %w", err)
	}

	return document, nil
}

// EncodeSinTable encodes float32 values as little-endian base64.
func EncodeSinTable(values []float32) string {
	buffer := make([]byte, len(values)*4)
	for index, value := range values {
		binary.LittleEndian.PutUint32(buffer[index*4:], math.Float32bits(value))
	}

	return base64.StdEncoding.EncodeToString(buffer)
}

// DecodeSinTable decodes little-endian base64 into float32 values.
func DecodeSinTable(encoded string) ([]float32, error) {
	buffer, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode sin table: %w", err)
	}
	if len(buffer)%4 != 0 {
		return nil, fmt.Errorf("sin table has %d bytes, want a multiple of four", len(buffer))
	}

	values := make([]float32, len(buffer)/4)
	for index := range values {
		values[index] = math.Float32frombits(binary.LittleEndian.Uint32(buffer[index*4:]))
	}

	return values, nil
}
