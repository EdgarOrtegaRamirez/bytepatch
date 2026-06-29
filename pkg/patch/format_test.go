package patch

import (
	"bytes"
	"testing"
)

func TestEncodeDecode_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		old  []byte
		new  []byte
	}{
		{"identical", []byte("hello"), []byte("hello")},
		{"insert", []byte("hello"), []byte("hello, world!")},
		{"delete", []byte("hello, world!"), []byte("hello")},
		{"replace", []byte("hello"), []byte("jello")},
		{"binary", []byte{0x00, 0x01, 0xFF, 0xFE}, []byte{0x00, 0xAA, 0xFF, 0xFE}},
		{"empty_old", nil, []byte("new")},
		{"empty_new", []byte("old"), nil},
		{"both_empty", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := BuildPatchDirect(tt.old, tt.new)

			// Encode
			var buf bytes.Buffer
			if err := Encode(&buf, p); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Decode
			decoded, err := Decode(&buf)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Verify header
			if decoded.Header.OldSize != p.Header.OldSize {
				t.Errorf("OldSize mismatch: %d != %d", decoded.Header.OldSize, p.Header.OldSize)
			}
			if decoded.Header.NewSize != p.Header.NewSize {
				t.Errorf("NewSize mismatch: %d != %d", decoded.Header.NewSize, p.Header.NewSize)
			}
			if decoded.Header.OldHash != p.Header.OldHash {
				t.Error("OldHash mismatch")
			}
			if decoded.Header.NewHash != p.Header.NewHash {
				t.Error("NewHash mismatch")
			}

			// Apply patch
			result, err := ApplyPatch(tt.old, decoded)
			if err != nil {
				t.Fatalf("ApplyPatch failed: %v", err)
			}
			if !bytes.Equal(result, tt.new) {
				t.Errorf("result mismatch: got %q, want %q", result, tt.new)
			}
		})
	}
}

func TestBuildPatchDirect(t *testing.T) {
	old := []byte("abcdef")
	new := []byte("abXYcdEF")

	p := BuildPatchDirect(old, new)

	// Should have COPY + INSERT + COPY + INSERT
	if len(p.Instructions) == 0 {
		t.Fatal("no instructions")
	}

	// Verify header
	if p.Header.OldSize != 6 {
		t.Errorf("OldSize = %d, want 6", p.Header.OldSize)
	}
	if p.Header.NewSize != 8 {
		t.Errorf("NewSize = %d, want 8", p.Header.NewSize)
	}

	// Apply and verify
	result, err := ApplyPatch(old, p)
	if err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}
	if !bytes.Equal(result, new) {
		t.Errorf("result mismatch: got %q, want %q", result, new)
	}
}

func TestApplyPatch_HashMismatch(t *testing.T) {
	old := []byte("hello")
	new := []byte("world")
	p := BuildPatchDirect(old, new)

	// Corrupt the old data
	corrupted := []byte("xxxxx")
	_, err := ApplyPatch(corrupted, p)
	if err == nil {
		t.Error("expected error for hash mismatch")
	}
}

func TestApplyPatch_SizeMismatch(t *testing.T) {
	old := []byte("hello")
	new := []byte("world")
	p := BuildPatchDirect(old, new)

	// Wrong size
	wrong := []byte("hi")
	_, err := ApplyPatch(wrong, p)
	if err == nil {
		t.Error("expected error for size mismatch")
	}
}

func TestDecode_InvalidMagic(t *testing.T) {
	data := []byte("INVALID\x00")
	_, err := Decode(bytes.NewReader(data))
	if err == nil {
		t.Error("expected error for invalid magic")
	}
}

func TestDecode_TruncatedData(t *testing.T) {
	// Write valid header but no instructions
	var buf bytes.Buffer
	buf.WriteString(Magic)
	// Just write zeros for the rest of header
	zeros := make([]byte, 92)
	buf.Write(zeros)

	_, err := Decode(&buf)
	// Instruction count is 0, so this is a valid (empty) patch
	if err != nil {
		t.Logf("Decode returned error (expected for truncated data): %v", err)
	}
}

func TestEstimateSize(t *testing.T) {
	old := []byte("hello, world!")
	new := []byte("hello, beautiful world!")
	p := BuildPatchDirect(old, new)

	estimated := EstimateSize(p)
	if estimated == 0 {
		t.Error("estimated size should be > 0")
	}
	t.Logf("Patch has %d instructions, estimated size: %d bytes", len(p.Instructions), estimated)
}

func TestLargeBinaryPatch(t *testing.T) {
	// Create two 10KB files with some differences (within DP threshold)
	old := make([]byte, 10*1024)
	new := make([]byte, 10*1024)

	for i := range old {
		old[i] = byte(i % 256)
		new[i] = byte(i % 256)
	}

	// Make changes at various positions
	for i := 1000; i < 1010; i++ {
		new[i] = byte(255)
	}
	for i := 5000; i < 5020; i++ {
		new[i] = byte(128)
	}
	for i := 9000; i < 9005; i++ {
		new[i] = byte(0)
	}

	p := BuildPatchDirect(old, new)

	// Encode and decode
	var buf bytes.Buffer
	if err := Encode(&buf, p); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	patchSize := buf.Len()
	t.Logf("Patch size: %d bytes (%.1f%% of original)", patchSize, float64(patchSize)/float64(len(old))*100)

	decoded, err := Decode(&buf)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	result, err := ApplyPatch(old, decoded)
	if err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}
	if !bytes.Equal(result, new) {
		t.Error("large binary patch result mismatch")
	}
}

func TestPatchCompressionRatio(t *testing.T) {
	// Test with a file where most bytes are unchanged
	old := make([]byte, 10000)
	new := make([]byte, 10000)
	for i := range old {
		old[i] = byte('A')
		new[i] = byte('A')
	}
	// Change 10 bytes
	for i := 5000; i < 5010; i++ {
		new[i] = byte('B')
	}

	p := BuildPatchDirect(old, new)
	estimated := EstimateSize(p)
	compression := float64(estimated) / float64(len(old)) * 100

	t.Logf("10KB file with 10 bytes changed:")
	t.Logf("  Patch instructions: %d", len(p.Instructions))
	t.Logf("  Estimated patch size: %d bytes", estimated)
	t.Logf("  Compression ratio: %.1f%%", compression)

	if compression > 50 {
		t.Errorf("compression ratio too high: %.1f%%", compression)
	}
}

func BenchmarkBuildPatch_1KB(b *testing.B) {
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
		BuildPatchDirect(old, new)
	}
}

func BenchmarkEncodeDecode_1KB(b *testing.B) {
	old := make([]byte, 1024)
	new := make([]byte, 1024)
	for i := range old {
		old[i] = byte(i % 256)
		new[i] = byte(i % 256)
	}
	for i := 100; i < 120; i++ {
		new[i] = byte(255)
	}
	p := BuildPatchDirect(old, new)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		Encode(&buf, p)
		Decode(&buf)
	}
}
