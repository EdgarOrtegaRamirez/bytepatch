package patch

import (
	"crypto/sha256"

	"github.com/EdgarOrtegaRamirez/bytepatch/pkg/diff"
)

// BuildPatch converts diff operations into a compact patch.
//
// The diff operations (EQUAL, INSERT, DELETE) are transformed into
// patch instructions (COPY, INSERT, SKIP) that are more efficient
// for binary patching.
//
// Strategy:
//   - EQUAL → COPY (copy bytes from old file)
//   - INSERT → INSERT (insert new bytes)
//   - DELETE → SKIP (advance position in old file without output)
func BuildPatch(old, new []byte, ops []diff.EditOp) *Patch {
	p := &Patch{
		Header: Header{
			OldSize: uint64(len(old)),
			NewSize: uint64(len(new)),
			OldHash: sha256.Sum256(old),
			NewHash: sha256.Sum256(new),
		},
	}

	for _, op := range ops {
		switch op.Type {
		case diff.OpEqual:
			p.Instructions = append(p.Instructions, Instruction{
				Op:     OpCopy,
				Offset: uint32(op.Offset),
				Length: uint32(op.Length),
			})
		case diff.OpInsert:
			p.Instructions = append(p.Instructions, Instruction{
				Op:     OpInsert,
				Length: uint32(op.Length),
				Data:   op.Data,
			})
		case diff.OpDelete:
			p.Instructions = append(p.Instructions, Instruction{
				Op:     OpSkip,
				Offset: uint32(op.Offset),
				Length: uint32(op.Length),
			})
		}
	}

	return p
}

// BuildPatchDirect computes the diff and builds the patch in one step.
func BuildPatchDirect(old, new []byte) *Patch {
	result := diff.ComputeDiff(old, new)
	return BuildPatch(old, new, result.Ops)
}
