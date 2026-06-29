# Security Policy

## Overview

BytePatch is a binary diff & patch tool. It processes files locally and does not make network requests or access external resources.

## Security Considerations

### File Operations
- BytePatch reads and writes files specified by the user
- No path traversal protection is needed as the tool operates on user-specified paths
- Patch files contain SHA-256 hashes for integrity verification

### Patch Verification
- Always use `bytepatch verify` before applying patches from untrusted sources
- Patches include SHA-256 hashes of both original and patched files
- The tool will reject patches that don't match the expected hashes

### Binary Format
- The patch format uses a fixed header with magic bytes
- Instruction lengths are bounds-checked against file sizes
- No arbitrary code execution is possible through patch files

## Best Practices

1. **Verify patches**: Always run `bytepatch verify` before applying
2. **Use checksums**: Compare SHA-256 hashes of original files before patching
3. **Backup originals**: Keep backups of original files before applying patches
4. **Validate input**: Ensure patch files come from trusted sources

## Reporting Issues

If you discover a security vulnerability, please report it responsibly by opening an issue on GitHub.
