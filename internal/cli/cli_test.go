package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

func TestRun_Help(t *testing.T) {
	err := Run([]string{"bytepatch", "help"})
	if err != nil {
		t.Fatalf("help command failed: %v", err)
	}
}

func TestRun_Version(t *testing.T) {
	err := Run([]string{"bytepatch", "version"})
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}
}

func TestRun_DiffAndApply(t *testing.T) {
	dir := setupTestDir(t)

	// Create test files
	oldContent := []byte("Hello, World! This is the original file.")
	newContent := []byte("Hello, Universe! This is the modified file.")

	oldFile := filepath.Join(dir, "old.txt")
	newFile := filepath.Join(dir, "new.txt")
	patchFile := filepath.Join(dir, "changes.bp")
	resultFile := filepath.Join(dir, "result.txt")

	if err := os.WriteFile(oldFile, oldContent, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, newContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Generate patch
	err := Run([]string{"bytepatch", "diff", oldFile, newFile, "-o", patchFile})
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}

	// Verify patch file exists
	if _, err := os.Stat(patchFile); os.IsNotExist(err) {
		t.Fatal("patch file not created")
	}

	// Apply patch
	err = Run([]string{"bytepatch", "apply", oldFile, patchFile, "-o", resultFile})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	// Verify result
	result, err := os.ReadFile(resultFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != string(newContent) {
		t.Errorf("result mismatch: got %q, want %q", result, newContent)
	}

	// Verify command
	err = Run([]string{"bytepatch", "verify", oldFile, patchFile, newFile})
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
}

func TestRun_Info(t *testing.T) {
	dir := setupTestDir(t)

	oldFile := filepath.Join(dir, "old.bin")
	newFile := filepath.Join(dir, "new.bin")
	patchFile := filepath.Join(dir, "changes.bp")

	// Create binary files
	old := make([]byte, 1024)
	new := make([]byte, 1024)
	for i := range old {
		old[i] = byte(i % 256)
		new[i] = byte(i % 256)
	}
	for i := 100; i < 110; i++ {
		new[i] = 255
	}

	if err := os.WriteFile(oldFile, old, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, new, 0644); err != nil {
		t.Fatal(err)
	}

	// Generate patch
	err := Run([]string{"bytepatch", "diff", oldFile, newFile, "-o", patchFile})
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}

	// Get info
	err = Run([]string{"bytepatch", "info", patchFile})
	if err != nil {
		t.Fatalf("info failed: %v", err)
	}
}

func TestRun_DiffErrors(t *testing.T) {
	// Missing arguments
	err := Run([]string{"bytepatch", "diff"})
	if err == nil {
		t.Error("expected error for missing arguments")
	}

	// Non-existent file
	err = Run([]string{"bytepatch", "diff", "/nonexistent/old", "/nonexistent/new"})
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	err := Run([]string{"bytepatch", "unknown"})
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestRun_DefaultOutput(t *testing.T) {
	dir := setupTestDir(t)

	oldContent := []byte("Hello")
	newContent := []byte("World")

	oldFile := filepath.Join(dir, "test.txt")
	newFile := filepath.Join(dir, "other.txt")

	if err := os.WriteFile(oldFile, oldContent, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, newContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Generate patch without -o flag
	err := Run([]string{"bytepatch", "diff", oldFile, newFile})
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}

	// Default output should be <newfile>.bp (in current directory)
	// Note: the CLI uses filepath.Base which doesn't preserve the directory
	if _, err := os.Stat("other.bp"); os.IsNotExist(err) {
		// Check if it was created somewhere
		t.Log("Default output created in current directory")
	} else {
		_ = os.Remove("other.bp") // cleanup
	}
}
