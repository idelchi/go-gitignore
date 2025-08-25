package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	fmt.Println("=== FINAL DOT NEGATION VERIFICATION ===")
	
	tempDir := createTempRepo()
	defer os.RemoveAll(tempDir)
	
	testDotNegationBehavior(tempDir)
}

func createTempRepo() string {
	tempDir, _ := os.MkdirTemp("", "git-dot-final-*")
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	cmd.Run()
	
	exec.Command("git", "config", "user.email", "test@example.com").Dir = tempDir
	exec.Command("git", "config", "user.name", "Test User").Dir = tempDir
	
	return tempDir
}

func testDotNegationBehavior(tempDir string) {
	// Exact pattern from the failing test: [* !. !.. !...]
	fmt.Println("Testing exact pattern from failing test: [* !. !.. !...]")
	
	gitignoreContent := `*
!.
!..
!...`
	
	gitignorePath := filepath.Join(tempDir, ".gitignore")
	os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)
	
	// Test the specific paths mentioned in test
	testCases := []struct {
		path     string
		desc     string
		expected string
	}{
		{".", "current directory", "should be ignored per test"},
		{"./.", "normalized to .", "should be ignored per test"},
		{"..", "parent directory", "should NOT be ignored per test"},
		{"...", "three dots", "should NOT be ignored per test"},
	}
	
	fmt.Println("\nResults:")
	for _, tc := range testCases {
		result := gitCheckIgnore(tempDir, tc.path)
		status := "NOT IGNORED"
		if strings.Contains(result, "IGNORED") {
			status = "IGNORED"
		}
		fmt.Printf("  '%s' (%s): %s (%s)\n", tc.path, tc.desc, status, tc.expected)
	}
	
	// Also test what happens with just "*" pattern
	fmt.Println("\nTesting with just '*' pattern (no negations):")
	os.WriteFile(gitignorePath, []byte("*"), 0644)
	
	for _, tc := range testCases {
		result := gitCheckIgnore(tempDir, tc.path)
		status := "NOT IGNORED"
		if strings.Contains(result, "IGNORED") {
			status = "IGNORED"
		}
		fmt.Printf("  '%s': %s\n", tc.path, status)
	}
}

func gitCheckIgnore(repoDir, path string) string {
	cmd := exec.Command("git", "check-ignore", "-v", path)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	
	if err != nil && strings.Contains(err.Error(), "exit status 1") {
		return "NOT IGNORED"
	}
	if err != nil {
		return fmt.Sprintf("ERROR: %v", err)
	}
	
	return fmt.Sprintf("IGNORED: %s", strings.TrimSpace(string(output)))
}