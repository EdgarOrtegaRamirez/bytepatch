// Package diff implements a diff algorithm for byte sequences.
//
// For small inputs (< 64KB), uses DP-based LCS with backtracking: O(N*M) time.
// For larger inputs, uses content-defined chunking (Rabin fingerprint) to split
// into chunks and diffs each chunk independently for better performance.
package diff

import (
	"hash"
	"hash/fnv"
	"fmt"
)

// OpType represents the type of edit operation.
type OpType int

const (
	OpEqual  OpType = iota
	OpInsert
	OpDelete
)

func (o OpType) String() string {
	switch o {
	case OpEqual:
		return "EQUAL"
	case OpInsert:
		return "INSERT"
	case OpDelete:
		return "DELETE"
	default:
		return "UNKNOWN"
	}
}

// EditOp represents a single edit operation.
type EditOp struct {
	Type   OpType
	Offset int    // Offset in old data for EQUAL/DELETE
	Length int    // Number of bytes
	Data   []byte // For INSERT: bytes to insert
}

// DiffResult contains the diff between two byte slices.
type DiffResult struct {
	OldLen int
	NewLen int
	Ops    []EditOp
}

// EditDistance returns the minimum number of single-byte edits.
func (d *DiffResult) EditDistance() int {
	dist := 0
	for _, op := range d.Ops {
		if op.Type == OpInsert || op.Type == OpDelete {
			dist += op.Length
		}
	}
	return dist
}

// NumChanges returns the number of changed regions.
func (d *DiffResult) NumChanges() int {
	count := 0
	for _, op := range d.Ops {
		if op.Type != OpEqual {
			count++
		}
	}
	return count
}

// ComputeDiff computes the optimal diff between two byte slices.
func ComputeDiff(old, new []byte) *DiffResult {
	result := &DiffResult{OldLen: len(old), NewLen: len(new)}
	if len(old) == 0 && len(new) == 0 {
		return result
	}
	if len(old) == 0 {
		result.Ops = []EditOp{{Type: OpInsert, Data: cloneBytes(new), Length: len(new)}}
		return result
	}
	if len(new) == 0 {
		result.Ops = []EditOp{{Type: OpDelete, Offset: 0, Length: len(old)}}
		return result
	}

	// For small inputs, use direct DP
	if len(old) <= 65536 && len(new) <= 65536 {
		result.Ops = computeDiffDP(old, new)
		return result
	}

	// For large inputs, use content-defined chunking
	result.Ops = computeDiffChunked(old, new)
	return result
}

// computeDiffDP computes diff using DP LCS + backtracking.
// Time: O(N*M), Space: O(N*M). Best for inputs up to ~64KB.
func computeDiffDP(old, new []byte) []EditOp {
	n, m := len(old), len(new)

	dp := make([][]uint16, n+1)
	for i := range dp {
		dp[i] = make([]uint16, m+1)
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if old[i-1] == new[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	var ops []EditOp
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && old[i-1] == new[j-1] {
			count := 0
			for i > 0 && j > 0 && old[i-1] == new[j-1] {
				count++
				i--
				j--
			}
			ops = append(ops, EditOp{Type: OpEqual, Offset: i, Length: count})
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			j--
			ops = append(ops, EditOp{Type: OpInsert, Data: []byte{new[j]}, Length: 1})
		} else {
			i--
			ops = append(ops, EditOp{Type: OpDelete, Offset: i, Length: 1})
		}
	}

	for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
		ops[i], ops[j] = ops[j], ops[i]
	}

	return mergeOps(ops)
}

// computeDiffChunked uses content-defined chunking to diff large files.
// Splits both files into content-defined chunks using Rabin fingerprint,
// finds matching chunks, and diffs each pair of matching chunks.
func computeDiffChunked(old, new []byte) []EditOp {
	oldChunks := chunkFile(old)
	newChunks := chunkFile(new)

	// Build a map from chunk hash to positions in old
	oldIndex := make(map[uint64][]int)
	for i, c := range oldChunks {
		oldIndex[c.hash] = append(oldIndex[c.hash], i)
	}

	// Find matching chunks using longest common subsequence on chunk hashes
	var matches []match

	// Greedy matching: for each new chunk, find the first unmatched old chunk with same hash
	usedOld := make([]bool, len(oldChunks))
	for j, c := range newChunks {
		if positions, ok := oldIndex[c.hash]; ok {
			for _, i := range positions {
				if !usedOld[i] {
					matches = append(matches, match{i, j})
					usedOld[i] = true
					break
				}
			}
		}
	}

	// Sort matches by oldIdx
	sortMatches(matches)

	// Convert matches to edit operations
	var ops []EditOp
	prevOldEnd := 0
	prevNewEnd := 0

	for _, m := range matches {
		oldStart := oldChunks[m.oldIdx].offset
		newStart := newChunks[m.newIdx].offset
		chunkLen := oldChunks[m.oldIdx].length

		// Insert/delete bytes between matched chunks
		if oldStart > prevOldEnd || newStart > prevNewEnd {
			// There are unmatched bytes - diff them directly
			oldMid := old[prevOldEnd:oldStart]
			newMid := new[prevNewEnd:newStart]
			midOps := computeDiffDP(oldMid, newMid)
			// Adjust offsets
			for k := range midOps {
				if midOps[k].Type == OpEqual || midOps[k].Type == OpDelete {
					midOps[k].Offset += prevOldEnd
				}
			}
			ops = append(ops, midOps...)
		}

		// Add the matching chunk
		ops = append(ops, EditOp{Type: OpEqual, Offset: oldStart, Length: chunkLen})

		prevOldEnd = oldStart + chunkLen
		prevNewEnd = newStart + chunkLen
	}

	// Handle remaining bytes
	if prevOldEnd < len(old) || prevNewEnd < len(new) {
		oldTail := old[prevOldEnd:]
		newTail := new[prevNewEnd:]
		tailOps := computeDiffDP(oldTail, newTail)
		for k := range tailOps {
			if tailOps[k].Type == OpEqual || tailOps[k].Type == OpDelete {
				tailOps[k].Offset += prevOldEnd
			}
		}
		ops = append(ops, tailOps...)
	}

	return mergeOps(ops)
}

// chunkInfo represents a content-defined chunk.
type chunkInfo struct {
	offset int
	length int
	hash   uint64
}

// match represents a matched chunk pair.
type match struct {
	oldIdx, newIdx int
}

// chunkFile splits a file into content-defined chunks using Rabin fingerprint.
func chunkFile(data []byte) []chunkInfo {
	const (
		minChunk = 256         // Minimum chunk size
		maxChunk = 8192        // Maximum chunk size
		mask     = 0x1FFF      // 13-bit mask: average chunk ~8KB
		windowSize = 48        // Rabin fingerprint window
	)

	if len(data) == 0 {
		return nil
	}

	rabin := newRabinFingerprint(windowSize)
	var chunks []chunkInfo
	start := 0

	for i := 0; i < len(data); i++ {
		rabin.slide(data[i])

		chunkLen := i - start + 1
		if chunkLen >= minChunk && (chunkLen >= maxChunk || (rabin.hash&mask == 0 && chunkLen >= minChunk)) {
			chunks = append(chunks, chunkInfo{
				offset: start,
				length: chunkLen,
				hash:   rabin.hash,
			})
			start = i + 1
			rabin.reset()
		}
	}

	// Handle remaining bytes
	if start < len(data) {
		h := fnv.New64a()
		h.Write(data[start:])
		chunks = append(chunks, chunkInfo{
			offset: start,
			length: len(data) - start,
			hash:   h.Sum64(),
		})
	}

	return chunks
}

// rabinFingerprint implements a rolling Rabin fingerprint.
type rabinFingerprint struct {
	window []byte
	pos    int
	size   int
	hash   uint64
	filled bool
	// Constants for the polynomial
	base  uint64
	mod   uint64
	power uint64 // base^(size-1) mod mod
}

func newRabinFingerprint(windowSize int) *rabinFingerprint {
	r := &rabinFingerprint{
		window: make([]byte, windowSize),
		size:   windowSize,
		base:   31,
		mod:    1<<61 - 1, // Mersenne prime
	}
	// Compute base^(size-1) mod mod
	r.power = uint64(1)
	for i := 0; i < windowSize-1; i++ {
		r.power = (r.power * r.base) % r.mod
	}
	return r
}

func (r *rabinFingerprint) slide(b byte) {
	if r.filled {
		// Remove oldest byte
		old := uint64(r.window[r.pos])
		r.hash = (r.hash + r.mod - (old*r.power)%r.mod) % r.mod
	} else if r.pos == r.size {
		r.filled = true
	}

	r.window[r.pos] = b
	r.hash = (r.hash*r.base + uint64(b)) % r.mod
	r.pos++
	if r.pos >= r.size {
		r.pos = 0
	}
}

func (r *rabinFingerprint) reset() {
	r.hash = 0
	r.pos = 0
	r.filled = false
	for i := range r.window {
		r.window[i] = 0
	}
}

// sortMatches sorts matches by oldIdx.
func sortMatches(matches []match) {
	for i := 1; i < len(matches); i++ {
		for j := i; j > 0 && matches[j].oldIdx < matches[j-1].oldIdx; j-- {
			matches[j], matches[j-1] = matches[j-1], matches[j]
		}
	}
}

// mergeOps merges consecutive same-type operations.
func mergeOps(ops []EditOp) []EditOp {
	if len(ops) == 0 {
		return ops
	}
	merged := []EditOp{ops[0]}
	for i := 1; i < len(ops); i++ {
		last := &merged[len(merged)-1]
		switch {
		case last.Type == OpEqual && ops[i].Type == OpEqual && last.Offset+last.Length == ops[i].Offset:
			last.Length += ops[i].Length
		case last.Type == OpDelete && ops[i].Type == OpDelete && last.Offset+last.Length == ops[i].Offset:
			last.Length += ops[i].Length
		case last.Type == OpInsert && ops[i].Type == OpInsert:
			last.Data = append(last.Data, ops[i].Data...)
			last.Length += ops[i].Length
		default:
			merged = append(merged, ops[i])
		}
	}
	return merged
}

// ApplyOps applies the edit operations to old data to produce new data.
func ApplyOps(old []byte, ops []EditOp) ([]byte, error) {
	var result []byte
	for _, op := range ops {
		switch op.Type {
		case OpEqual:
			if op.Offset < 0 || op.Offset+op.Length > len(old) {
				return nil, fmt.Errorf("EQUAL out of bounds: offset=%d len=%d", op.Offset, op.Length)
			}
			result = append(result, old[op.Offset:op.Offset+op.Length]...)
		case OpDelete:
			if op.Offset < 0 || op.Offset+op.Length > len(old) {
				return nil, fmt.Errorf("DELETE out of bounds: offset=%d len=%d", op.Offset, op.Length)
			}
		case OpInsert:
			result = append(result, op.Data...)
		}
	}
	return result, nil
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

// Ensure hash import is used
var _ hash.Hash
