package diff

import (
	"bytes"
	"testing"
)

func TestComputeDiff_EmptyInputs(t *testing.T) {
	tests := []struct {
		name string
		old  []byte
		new  []byte
	}{
		{"both empty", nil, nil},
		{"old empty", nil, []byte("hello")},
		{"new empty", []byte("hello"), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeDiff(tt.old, tt.new)
			if result == nil {
				t.Fatal("ComputeDiff returned nil")
			}
			reconstructed, err := ApplyOps(tt.old, result.Ops)
			if err != nil {
				t.Fatalf("ApplyOps failed: %v", err)
			}
			if !bytes.Equal(reconstructed, tt.new) {
				t.Errorf("reconstruction mismatch: got %q, want %q", reconstructed, tt.new)
			}
		})
	}
}

func TestComputeDiff_Identical(t *testing.T) {
	data := []byte("hello, world!")
	result := ComputeDiff(data, data)

	if result.EditDistance() != 0 {
		t.Errorf("expected edit distance 0, got %d", result.EditDistance())
	}
	if result.NumChanges() != 0 {
		t.Errorf("expected 0 changes, got %d", result.NumChanges())
	}

	reconstructed, err := ApplyOps(data, result.Ops)
	if err != nil {
		t.Fatalf("ApplyOps failed: %v", err)
	}
	if !bytes.Equal(reconstructed, data) {
		t.Errorf("reconstruction mismatch")
	}
}

func TestComputeDiff_SimpleInsert(t *testing.T) {
	old := []byte("hello")
	new := []byte("hello, world!")

	result := ComputeDiff(old, new)
	reconstructed, err := ApplyOps(old, result.Ops)
	if err != nil {
		t.Fatalf("ApplyOps failed: %v", err)
	}
	if !bytes.Equal(reconstructed, new) {
		t.Errorf("reconstruction mismatch: got %q, want %q", reconstructed, new)
	}
	if result.EditDistance() != len(", world!") {
		t.Errorf("expected edit distance %d, got %d", len(", world!"), result.EditDistance())
	}
}

func TestComputeDiff_SimpleDelete(t *testing.T) {
	old := []byte("hello, world!")
	new := []byte("hello")

	result := ComputeDiff(old, new)
	reconstructed, err := ApplyOps(old, result.Ops)
	if err != nil {
		t.Fatalf("ApplyOps failed: %v", err)
	}
	if !bytes.Equal(reconstructed, new) {
		t.Errorf("reconstruction mismatch: got %q, want %q", reconstructed, new)
	}
}

func TestComputeDiff_SimpleReplace(t *testing.T) {
	old := []byte("hello")
	new := []byte("jello")

	result := ComputeDiff(old, new)
	reconstructed, err := ApplyOps(old, result.Ops)
	if err != nil {
		t.Fatalf("ApplyOps failed: %v", err)
	}
	if !bytes.Equal(reconstructed, new) {
		t.Errorf("reconstruction mismatch: got %q, want %q", reconstructed, new)
	}
}

func TestComputeDiff_CompleteReplacement(t *testing.T) {
	old := []byte("aaaa")
	new := []byte("bbbb")

	result := ComputeDiff(old, new)
	reconstructed, err := ApplyOps(old, result.Ops)
	if err != nil {
		t.Fatalf("ApplyOps failed: %v", err)
	}
	if !bytes.Equal(reconstructed, new) {
		t.Errorf("reconstruction mismatch: got %q, want %q", reconstructed, new)
	}
	// 4 deletes + 4 inserts = 8 single-byte edits
	if result.EditDistance() != 8 {
		t.Errorf("expected edit distance 8, got %d", result.EditDistance())
	}
}

func TestComputeDiff_Interleaved(t *testing.T) {
	old := []byte("abcdef")
	new := []byte("abXYcdEF")

	result := ComputeDiff(old, new)
	reconstructed, err := ApplyOps(old, result.Ops)
	if err != nil {
		t.Fatalf("ApplyOps failed: %v", err)
	}
	if !bytes.Equal(reconstructed, new) {
		t.Errorf("reconstruction mismatch: got %q, want %q", reconstructed, new)
	}
}

func TestComputeDiff_BinaryData(t *testing.T) {
	old := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
	new := []byte{0x00, 0x01, 0xAA, 0x02, 0xFF, 0xFE, 0xFD}

	result := ComputeDiff(old, new)
	reconstructed, err := ApplyOps(old, result.Ops)
	if err != nil {
		t.Fatalf("ApplyOps failed: %v", err)
	}
	if !bytes.Equal(reconstructed, new) {
		t.Errorf("reconstruction mismatch")
	}
}

func TestComputeDiff_LargeInput(t *testing.T) {
	// Generate a 10KB file with some changes
	old := make([]byte, 10000)
	new := make([]byte, 10000)

	for i := range old {
		old[i] = byte(i % 256)
		new[i] = byte(i % 256)
	}

	// Make some changes in new
	for i := 100; i < 110; i++ {
		new[i] = byte(255 - i%256)
	}
	for i := 5000; i < 5010; i++ {
		new[i] = byte(128)
	}

	result := ComputeDiff(old, new)
	reconstructed, err := ApplyOps(old, result.Ops)
	if err != nil {
		t.Fatalf("ApplyOps failed: %v", err)
	}
	if !bytes.Equal(reconstructed, new) {
		t.Error("reconstruction mismatch for large input")
	}
	t.Logf("Edit distance: %d, Changes: %d", result.EditDistance(), result.NumChanges())
}

func TestComputeDiff_SingleByteDiff(t *testing.T) {
	old := []byte{0x00}
	new := []byte{0x01}

	result := ComputeDiff(old, new)
	reconstructed, err := ApplyOps(old, result.Ops)
	if err != nil {
		t.Fatalf("ApplyOps failed: %v", err)
	}
	if !bytes.Equal(reconstructed, new) {
		t.Errorf("reconstruction mismatch")
	}
	// Changing a byte requires 1 delete + 1 insert = 2 edits
	if result.EditDistance() != 2 {
		t.Errorf("expected edit distance 2, got %d", result.EditDistance())
	}
}

func TestApplyOps_OutOfBounds(t *testing.T) {
	old := []byte{0x01, 0x02, 0x03}
	ops := []EditOp{{Type: OpEqual, Offset: 0, Length: 10}}

	_, err := ApplyOps(old, ops)
	if err == nil {
		t.Error("expected error for out-of-bounds EQUAL op")
	}
}

func BenchmarkComputeDiff_1KB(b *testing.B) {
	old := make([]byte, 1024)
	new := make([]byte, 1024)
	for i := range old {
		old[i] = byte(i % 256)
		new[i] = byte(i % 256)
	}
	for i := 100; i < 120; i++ {
		new[i] = byte(255)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeDiff(old, new)
	}
}

func BenchmarkComputeDiff_10KB(b *testing.B) {
	old := make([]byte, 10240)
	new := make([]byte, 10240)
	for i := range old {
		old[i] = byte(i % 256)
		new[i] = byte(i % 256)
	}
	for i := 1000; i < 1020; i++ {
		new[i] = byte(255)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeDiff(old, new)
	}
}

func BenchmarkApplyOps_1KB(b *testing.B) {
	old := make([]byte, 1024)
	new := make([]byte, 1024)
	for i := range old {
		old[i] = byte(i % 256)
		new[i] = byte(i % 256)
	}
	for i := 100; i < 120; i++ {
		new[i] = byte(255)
	}
	result := ComputeDiff(old, new)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ApplyOps(old, result.Ops)
	}
}
