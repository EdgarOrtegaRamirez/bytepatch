# AGENTS.md

## Project Overview

BytePatch is a binary diff & patch tool in Go. It generates compact patches between files using the LCS algorithm and applies them to reconstruct originals.

## Key Algorithms

1. **LCS (Longest Common Subsequence)**: DP-based diff for files ≤64KB
2. **Rabin Fingerprint**: Content-defined chunking for larger files
3. **Binary Patch Format**: Custom compact format with SHA-256 integrity

## Code Structure

- `pkg/diff/`: Core diff algorithm (LCS + chunking)
- `pkg/patch/`: Patch format (encode/decode/apply)
- `internal/cli/`: CLI commands (diff, apply, info, verify)
- `cmd/bytepatch/`: Entry point

## Development

```bash
# Run all tests
go test ./... -v

# Build
go build -o bytepatch ./cmd/bytepatch/

# Run benchmarks
go test ./pkg/diff/ -bench=.
```

## Testing

- All tests must pass before pushing
- Tests cover: empty inputs, identical files, inserts, deletes, replacements, binary data, large files
- Round-trip tests: diff → encode → decode → apply → verify

## Commit Convention

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation
- `test:` tests
- `chore:` maintenance
