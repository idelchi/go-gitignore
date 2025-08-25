package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	fmt.Println("=== VERIFYING ROOTED /* DOTFILE BEHAVIOR ===")
	
	tempDir := createTempRepo()
	defer os.RemoveAll(tempDir)
	
	// Test the specific failing case
	testRootedStarDotfiles(tempDir)
}

func createTempRepo() string {
	tempDir, _ := os.MkdirTemp("", "git-rooted-star-*")
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	cmd.Run()
	
	exec.Command("git", "config", "user.email", "test@example.com").Dir = tempDir
	exec.Command("git", "config", "user.name", "Test User").Dir = tempDir
	
	return tempDir
}

func testRootedStarDotfiles(tempDir string) {
	// Create test files (both dotfiles and regular files)
	files := []string{".gitignore", ".hidden", "visible.txt", "README.md"}
	for _, file := range files {
		os.WriteFile(filepath.Join(tempDir, file), []byte("content"), 0644)
	}
	
	// Test with rooted /* pattern
	gitignorePath := filepath.Join(tempDir, ".gitignore") 
	os.WriteFile(gitignorePath, []byte("/*"), 0644)
	
	fmt.Println("Testing rooted /* pattern against different files:")
	
	testFiles := []string{".gitignore", ".hidden", "visible.txt", "README.md"}
	
	for _, testFile := range testFiles {
		result := gitCheckIgnore(tempDir, testFile)
		if strings.Contains(result, "IGNORED") {
			fmt.Printf("  IGNORED: %s\n", testFile)
		} else {
			fmt.Printf("  NOT IGNORED: %s\n", testFile)
		}
	}
	
	// Also test the exact case from the failing test
	fmt.Println("\nSpecific failing test case:")
	fmt.Println("Pattern: [/*]") 
	fmt.Println("Expected: .gitignore should NOT be ignored")
	result := gitCheckIgnore(tempDir, ".gitignore")
	if strings.Contains(result, "IGNORED") {
		fmt.Println("❌ ACTUAL: .gitignore IS ignored - test expectation might be wrong!")
	} else {
		fmt.Println("✅ ACTUAL: .gitignore is NOT ignored - matches test expectation")
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