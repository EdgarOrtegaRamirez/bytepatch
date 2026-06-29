# BytePatch

A modern binary diff & patch tool written in Go.

BytePatch generates compact binary patches between files and applies them to reconstruct the original. It uses the LCS (Longest Common Subsequence) algorithm for small files and content-defined chunking (Rabin fingerprint) for larger files.

## Features

- **Optimal Diffs**: Uses dynamic programming to find the minimum edit distance
- **Binary-Safe**: Works with any file type — text, images, executables, archives
- **Compact Patches**: Efficient binary format with SHA-256 integrity verification
- **Content-Defined Chunking**: Rabin fingerprint-based chunking for large files
- **Verification**: Built-in patch integrity verification with hash checking
- **Cross-Platform**: Single binary, no dependencies

## Installation

```bash
# From source
go install github.com/EdgarOrtegaRamirez/bytepatch/cmd/bytepatch@latest

# Or build locally
git clone https://github.com/EdgarOrtegaRamirez/bytepatch
cd bytepatch
go build -o bytepatch ./cmd/bytepatch/
```

## Quick Start

```bash
# Generate a patch
bytepatch diff old.bin new.bin -o changes.bp

# Apply the patch
bytepatch apply old.bin changes.bp -o new.bin

# Verify the patch produces expected output
bytepatch verify old.bin changes.bp expected.bin

# Show patch metadata
bytepatch info changes.bp
```

## How It Works

### Diff Algorithm

1. **Small files (≤64KB)**: Uses DP-based LCS with backtracking — O(N×M) time
2. **Large files (>64KB)**: Uses Rabin fingerprint content-defined chunking to split files into chunks, matches chunks by hash, and diffs unmatched regions

### Patch Format

```
[MAGIC: 8 bytes]       "BYPATCH\0"
[VERSION: 4 bytes]     uint32 LE (1)
[FLAGS: 4 bytes]       uint32 LE
[OLD_SIZE: 8 bytes]    uint64 LE
[NEW_SIZE: 8 bytes]    uint64 LE
[OLD_HASH: 32 bytes]   SHA-256
[NEW_HASH: 32 bytes]   SHA-256
[NUM_INST: 4 bytes]    uint32 LE
[INSTRUCTIONS...]
```

### Instructions

| Opcode | Description |
|--------|-------------|
| COPY   | Copy bytes from original file |
| INSERT | Insert new bytes |
| SKIP   | Skip bytes in original file |

## CLI Reference

```
bytepatch <command> [options]

COMMANDS:
    diff <old> <new> [-o <patch>]    Generate a binary patch
    apply <file> <patch> [-o <out>]  Apply a patch to a file
    info <patch>                     Show patch metadata
    verify <file> <patch> <expected> Verify a patch produces expected output
    version                          Show version
    help                             Show this help
```

## Library Usage

```go
package main

import (
    "fmt"
    "github.com/EdgarOrtegaRamirez/bytepatch/pkg/diff"
    "github.com/EdgarOrtegaRamirez/bytepatch/pkg/patch"
)

func main() {
    old := []byte("Hello, World!")
    new := []byte("Hello, Universe!")

    // Compute diff
    result := diff.ComputeDiff(old, new)
    fmt.Printf("Edit distance: %d\n", result.EditDistance())

    // Build and apply patch
    p := patch.BuildPatchDirect(old, new)
    patched, err := patch.ApplyPatch(old, p)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Result: %s\n", patched)
}
```

## Performance

| File Size | Changes | Patch Size | Time |
|-----------|---------|------------|------|
| 1 KB      | 20 bytes | 159 bytes | <1ms |
| 10 KB     | 35 bytes | 189 bytes | <1ms |
| 100 KB    | 35 bytes | ~200 bytes | ~1s |
| 1 MB      | 100 bytes | ~250 bytes | ~10s |

## Architecture

```
bytepatch/
├── cmd/bytepatch/       # CLI entry point
├── internal/cli/        # CLI implementation
├── pkg/
│   ├── diff/            # LCS diff algorithm
│   │   ├── myers.go     # Core diff engine
│   │   └── myers_test.go
│   └── patch/           # Patch format
│       ├── format.go    # Encode/decode
│       ├── builder.go   # Build patches from diffs
│       └── format_test.go
└── .github/workflows/   # CI/CD
```

## License

MIT
