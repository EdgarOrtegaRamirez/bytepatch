// Package patch implements the binary patch format for BytePatch.
//
// The patch format stores edit operations as a compact binary representation:
//   - Header with metadata (file hashes, sizes, instruction count)
//   - Instruction stream: COPY (from original), INSERT (new data), SKIP (pass through)
//
// Format (version 1.0):
//
//	[MAGIC: 8 bytes] "BYPATCH\0"
//	[VERSION: 4 bytes] uint32 LE
//	[FLAGS: 4 bytes] uint32 LE
//	[OLD_SIZE: 8 bytes] uint64 LE
//	[NEW_SIZE: 8 bytes] uint64 LE
//	[OLD_HASH: 32 bytes] SHA-256
//	[NEW_HASH: 32 bytes] SHA-256
//	[NUM_INSTRUCTIONS: 4 bytes] uint32 LE
//	[INSTRUCTIONS...]
package patch

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	Magic     = "BYPATCH\x00"
	Version   = 1
	HeaderLen = 8 + 4 + 4 + 8 + 8 + 32 + 32 + 4 // 100 bytes

	// Flags
	FlagCompressed = 1 << 0 // Instructions are zlib-compressed
)

// OpCode represents the type of patch instruction.
type OpCode byte

const (
	OpCopy OpCode = iota // Copy bytes from old file at offset
	OpInsert             // Insert new bytes
	OpSkip               // Skip bytes in old file (advance offset without output)
)

// Instruction represents a single patch instruction.
type Instruction struct {
	Op     OpCode
	Offset uint32 // For OpCopy/Skip: offset in old file
	Length uint32 // Number of bytes
	Data   []byte // For OpInsert: bytes to insert
}

// Header contains the metadata for a patch file.
type Header struct {
	OldSize uint64
	NewSize uint64
	OldHash [32]byte
	NewHash [32]byte
	Flags   uint32
}

// Patch represents a complete binary patch.
type Patch struct {
	Header       Header
	Instructions []Instruction
}

// Hash computes the SHA-256 hash of data.
func Hash(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// Encode writes the patch to the given writer.
func Encode(w io.Writer, p *Patch) error {
	// Write magic
	if _, err := w.Write([]byte(Magic)); err != nil {
		return fmt.Errorf("write magic: %w", err)
	}

	// Write version
	if err := binary.Write(w, binary.LittleEndian, uint32(Version)); err != nil {
		return fmt.Errorf("write version: %w", err)
	}

	// Write flags
	if err := binary.Write(w, binary.LittleEndian, p.Header.Flags); err != nil {
		return fmt.Errorf("write flags: %w", err)
	}

	// Write sizes
	if err := binary.Write(w, binary.LittleEndian, p.Header.OldSize); err != nil {
		return fmt.Errorf("write old size: %w", err)
	}
	if err := binary.Write(w, binary.LittleEndian, p.Header.NewSize); err != nil {
		return fmt.Errorf("write new size: %w", err)
	}

	// Write hashes
	if _, err := w.Write(p.Header.OldHash[:]); err != nil {
		return fmt.Errorf("write old hash: %w", err)
	}
	if _, err := w.Write(p.Header.NewHash[:]); err != nil {
		return fmt.Errorf("write new hash: %w", err)
	}

	// Write instruction count
	if err := binary.Write(w, binary.LittleEndian, uint32(len(p.Instructions))); err != nil {
		return fmt.Errorf("write instruction count: %w", err)
	}

	// Write instructions
	for _, inst := range p.Instructions {
		if err := writeInstruction(w, &inst); err != nil {
			return fmt.Errorf("write instruction: %w", err)
		}
	}

	return nil
}

func writeInstruction(w io.Writer, inst *Instruction) error {
	// Write opcode + length in a packed format
	// Byte 0: opcode (2 bits) | length_high (6 bits)
	// If length_high == 0x3F: next 4 bytes are the full length (LE uint32)
	// For OpInsert: data follows

	length := inst.Length
 opcodeByte := byte(inst.Op) << 6

	if length < 0x3F {
		opcodeByte |= byte(length)
		if _, err := w.Write([]byte{opcodeByte}); err != nil {
			return err
		}
	} else {
		opcodeByte |= 0x3F
		if _, err := w.Write([]byte{opcodeByte}); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, length); err != nil {
			return err
		}
	}

	// For OpCopy and OpSkip, write the offset
	if inst.Op == OpCopy || inst.Op == OpSkip {
		if err := binary.Write(w, binary.LittleEndian, inst.Offset); err != nil {
			return err
		}
	}

	// For OpInsert, write the data
	if inst.Op == OpInsert {
		if _, err := w.Write(inst.Data); err != nil {
			return err
		}
	}

	return nil
}

// Decode reads a patch from the given reader.
func Decode(r io.Reader) (*Patch, error) {
	// Read magic
	magic := make([]byte, 8)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if string(magic) != Magic {
		return nil, fmt.Errorf("invalid magic: %q", magic)
	}

	// Read version
	var version uint32
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}
	if version != Version {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	// Read flags
	var flags uint32
	if err := binary.Read(r, binary.LittleEndian, &flags); err != nil {
		return nil, fmt.Errorf("read flags: %w", err)
	}

	// Read sizes
	var oldSize, newSize uint64
	if err := binary.Read(r, binary.LittleEndian, &oldSize); err != nil {
		return nil, fmt.Errorf("read old size: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &newSize); err != nil {
		return nil, fmt.Errorf("read new size: %w", err)
	}

	// Read hashes
	var oldHash, newHash [32]byte
	if _, err := io.ReadFull(r, oldHash[:]); err != nil {
		return nil, fmt.Errorf("read old hash: %w", err)
	}
	if _, err := io.ReadFull(r, newHash[:]); err != nil {
		return nil, fmt.Errorf("read new hash: %w", err)
	}

	// Read instruction count
	var numInst uint32
	if err := binary.Read(r, binary.LittleEndian, &numInst); err != nil {
		return nil, fmt.Errorf("read instruction count: %w", err)
	}

	// Read instructions
	instructions := make([]Instruction, numInst)
	for i := uint32(0); i < numInst; i++ {
		inst, err := readInstruction(r)
		if err != nil {
			return nil, fmt.Errorf("read instruction %d: %w", i, err)
		}
		instructions[i] = *inst
	}

	return &Patch{
		Header: Header{
			OldSize: oldSize,
			NewSize: newSize,
			OldHash: oldHash,
			NewHash: newHash,
			Flags:   flags,
		},
		Instructions: instructions,
	}, nil
}

func readInstruction(r io.Reader) (*Instruction, error) {
	// Read opcode byte
	var opByte byte
	if err := binary.Read(r, binary.LittleEndian, &opByte); err != nil {
		return nil, err
	}

	op := OpCode(opByte >> 6)
	lengthLow := uint32(opByte & 0x3F)

	var length uint32
	if lengthLow < 0x3F {
		length = lengthLow
	} else {
		// Extended length
		if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
			return nil, err
		}
	}

	inst := &Instruction{Op: op, Length: length}

	// Read offset for Copy/Skip
	if op == OpCopy || op == OpSkip {
		if err := binary.Read(r, binary.LittleEndian, &inst.Offset); err != nil {
			return nil, err
		}
	}

	// Read data for Insert
	if op == OpInsert {
		inst.Data = make([]byte, length)
		if _, err := io.ReadFull(r, inst.Data); err != nil {
			return nil, err
		}
	}

	return inst, nil
}

// ApplyPatch applies a patch to the original data to produce the new data.
func ApplyPatch(original []byte, p *Patch) ([]byte, error) {
	if uint64(len(original)) != p.Header.OldSize {
		return nil, fmt.Errorf("original size mismatch: got %d, expected %d", len(original), p.Header.OldSize)
	}

	// Verify original hash
	actualHash := Hash(original)
	if actualHash != p.Header.OldHash {
		return nil, fmt.Errorf("original hash mismatch")
	}

	result := make([]byte, 0, p.Header.NewSize)

	for _, inst := range p.Instructions {
		switch inst.Op {
		case OpCopy:
			if uint64(inst.Offset)+uint64(inst.Length) > p.Header.OldSize {
				return nil, fmt.Errorf("COPY out of bounds: offset=%d len=%d", inst.Offset, inst.Length)
			}
			result = append(result, original[inst.Offset:inst.Offset+inst.Length]...)
		case OpInsert:
			result = append(result, inst.Data...)
		case OpSkip:
			// Just advance the conceptual position in old data
			// (handled by subsequent OpCopy offsets)
		}
	}

	// Verify output size
	if uint64(len(result)) != p.Header.NewSize {
		return nil, fmt.Errorf("output size mismatch: got %d, expected %d", len(result), p.Header.NewSize)
	}

	// Verify output hash
	actualNewHash := Hash(result)
	if actualNewHash != p.Header.NewHash {
		return nil, fmt.Errorf("output hash mismatch")
	}

	return result, nil
}

// EstimateSize returns the estimated size of the patch in bytes.
func EstimateSize(p *Patch) uint64 {
	size := uint64(HeaderLen)
	for _, inst := range p.Instructions {
		size++ // opcode byte
		if inst.Length >= 0x3F {
			size += 4 // extended length
		}
		if inst.Op == OpCopy || inst.Op == OpSkip {
			size += 4 // offset
		}
		if inst.Op == OpInsert {
			size += uint64(inst.Length) // data
		}
	}
	return size
}
