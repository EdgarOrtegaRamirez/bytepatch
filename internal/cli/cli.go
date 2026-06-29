// Package main implements the CLI for bytepatch.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/EdgarOrtegaRamirez/bytepatch/pkg/patch"
)

// Version is the current version of bytepatch.
const Version = "1.0.0"

// Run is the main entry point for the CLI.
func Run(args []string) error {
	if len(args) < 2 {
		printUsage()
		return nil
	}

	cmd := args[1]
	switch cmd {
	case "diff":
		return cmdDiff(args[2:])
	case "apply":
		return cmdApply(args[2:])
	case "info":
		return cmdInfo(args[2:])
	case "verify":
		return cmdVerify(args[2:])
	case "version":
		fmt.Printf("bytepatch %s\n", Version)
		return nil
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s\nRun 'bytepatch help' for usage", cmd)
	}
}

func printUsage() {
	fmt.Print(`bytepatch - Modern binary diff & patch tool

USAGE:
    bytepatch <command> [options]

COMMANDS:
    diff <old> <new> [-o <patch>]    Generate a binary patch
    apply <file> <patch> [-o <out>]  Apply a patch to a file
    info <patch>                     Show patch metadata
    verify <file> <patch> <expected> Verify a patch produces expected output
    version                          Show version
    help                             Show this help

OPTIONS:
    -o, --output <file>    Output file (default: stdout or <file>.bp)

EXAMPLES:
    # Generate a patch
    bytepatch diff old.bin new.bin -o changes.bp

    # Apply a patch
    bytepatch apply old.bin changes.bp -o new.bin

    # Show patch info
    bytepatch info changes.bp

    # Verify a patch
    bytepatch verify old.bin changes.bp new.bin
`)
}

func cmdDiff(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: bytepatch diff <old> <new> [-o <patch>]")
	}

	oldFile := args[0]
	newFile := args[1]
	outputFile := getOutputFlag(args, "")

	// Default output name
	if outputFile == "" {
		base := filepath.Base(newFile)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		outputFile = name + ".bp"
	}

	// Read files
	old, err := os.ReadFile(oldFile)
	if err != nil {
		return fmt.Errorf("read old file: %w", err)
	}
	new, err := os.ReadFile(newFile)
	if err != nil {
		return fmt.Errorf("read new file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Diffing %s (%d bytes) -> %s (%d bytes)...\n", oldFile, len(old), newFile, len(new))

	// Build patch
	p := patch.BuildPatchDirect(old, new)

	// Write patch
	f, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("create patch file: %w", err)
	}
	defer f.Close()

	if err := patch.Encode(f, p); err != nil {
		return fmt.Errorf("encode patch: %w", err)
	}

	info, _ := f.Stat()
	patchSize := info.Size()

	fmt.Fprintf(os.Stderr, "Patch: %s (%d instructions, %d bytes, %.1f%% of original)\n",
		outputFile, len(p.Instructions), patchSize, float64(patchSize)/float64(max(len(old), 1))*100)

	return nil
}

func cmdApply(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: bytepatch apply <file> <patch> [-o <output>]")
	}

	fileFile := args[0]
	patchFile := args[1]
	outputFile := getOutputFlag(args, "")

	// Read original file
	original, err := os.ReadFile(fileFile)
	if err != nil {
		return fmt.Errorf("read original file: %w", err)
	}

	// Read patch
	patchData, err := os.ReadFile(patchFile)
	if err != nil {
		return fmt.Errorf("read patch file: %w", err)
	}

	p, err := patch.Decode(strings.NewReader(string(patchData)))
	if err != nil {
		return fmt.Errorf("decode patch: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Applying patch to %s (%d instructions)...\n", fileFile, len(p.Instructions))

	// Apply patch
	result, err := patch.ApplyPatch(original, p)
	if err != nil {
		return fmt.Errorf("apply patch: %w", err)
	}

	// Write output
	if outputFile == "" {
		outputFile = fileFile + ".patched"
	}

	if err := os.WriteFile(outputFile, result, 0644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Output: %s (%d bytes)\n", outputFile, len(result))
	return nil
}

func cmdInfo(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: bytepatch info <patch>")
	}

	patchFile := args[0]
	data, err := os.ReadFile(patchFile)
	if err != nil {
		return fmt.Errorf("read patch file: %w", err)
	}

	p, err := patch.Decode(strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("decode patch: %w", err)
	}

	fmt.Printf("Patch: %s\n", patchFile)
	fmt.Printf("Format: bytepatch v%d\n", 1)
	fmt.Printf("Old file: %d bytes (SHA-256: %x)\n", p.Header.OldSize, p.Header.OldHash[:8])
	fmt.Printf("New file: %d bytes (SHA-256: %x)\n", p.Header.NewSize, p.Header.NewHash[:8])
	fmt.Printf("Instructions: %d\n", len(p.Instructions))
	fmt.Printf("Patch size: %d bytes\n", len(data))
	fmt.Printf("Compression: %.1f%%\n", float64(len(data))/float64(maxInt(1, int(p.Header.OldSize)))*100)

	// Instruction breakdown
	var copyCount, insertCount, skipCount int
	var insertBytes uint64
	for _, inst := range p.Instructions {
		switch inst.Op {
		case patch.OpCopy:
			copyCount++
		case patch.OpInsert:
			insertCount++
			insertBytes += uint64(inst.Length)
		case patch.OpSkip:
			skipCount++
		}
	}
	fmt.Printf("  COPY: %d instructions\n", copyCount)
	fmt.Printf("  INSERT: %d instructions (%d bytes)\n", insertCount, insertBytes)
	fmt.Printf("  SKIP: %d instructions\n", skipCount)

	return nil
}

func cmdVerify(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: bytepatch verify <file> <patch> <expected>")
	}

	fileFile := args[0]
	patchFile := args[1]
	expectedFile := args[2]

	// Read files
	original, err := os.ReadFile(fileFile)
	if err != nil {
		return fmt.Errorf("read original file: %w", err)
	}

	data, err := os.ReadFile(patchFile)
	if err != nil {
		return fmt.Errorf("read patch file: %w", err)
	}

	expected, err := os.ReadFile(expectedFile)
	if err != nil {
		return fmt.Errorf("read expected file: %w", err)
	}

	p, err := patch.Decode(strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("decode patch: %w", err)
	}

	result, err := patch.ApplyPatch(original, p)
	if err != nil {
		return fmt.Errorf("apply patch: %w", err)
	}

	if string(result) == string(expected) {
		fmt.Println("✓ VERIFIED: Patch produces expected output")
		return nil
	}

	// Find first difference
	for i := range result {
		if i >= len(expected) || result[i] != expected[i] {
			fmt.Printf("✗ FAILED: First difference at byte %d (got 0x%02x, expected 0x%02x)\n",
				i, result[i], expected[i])
			return fmt.Errorf("verification failed")
		}
	}
	fmt.Printf("✗ FAILED: Output is shorter (got %d bytes, expected %d)\n", len(result), len(expected))
	return fmt.Errorf("verification failed")
}

func getOutputFlag(args []string, defaultVal string) string {
	for i, arg := range args {
		if (arg == "-o" || arg == "--output") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return defaultVal
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
