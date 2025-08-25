package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	fmt.Println("=== DETAILED GIT BEHAVIOR TEST ===")
	
	tempDir := createTempRepo()
	defer os.RemoveAll(tempDir)
	
	// Test the dot negation more carefully
	testDotNegationDetailed(tempDir)
	
	// Test contents-only with proper Git tracking
	testContentsOnlyDetailed(tempDir)
	
	// Test dotfile behavior more thoroughly
	testDotfiles(tempDir)
}

func createTempRepo() string {
	tempDir, _ := os.MkdirTemp("", "git-detailed-*")
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	cmd.Run()
	
	// Configure git to avoid warnings
	exec.Command("git", "config", "user.email", "test@example.com").Dir = tempDir
	exec.Command("git", "config", "user.name", "Test User").Dir = tempDir
	
	return tempDir
}

func testDotNegationDetailed(tempDir string) {
	fmt.Println("\n=== DOT NEGATION DETAILED TEST ===")
	
	// Create some files first
	os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("test"), 0644)
	
	// Test 1: Just * pattern
	fmt.Println("\nTest 1: Pattern '*'")
	writeGitignore(tempDir, "*")
	
	dotResult := gitCheckIgnore(tempDir, ".")
	fmt.Printf("  check-ignore '.': %s", dotResult)
	
	fileResult := gitCheckIgnore(tempDir, "file1.txt")
	fmt.Printf("  check-ignore 'file1.txt': %s", fileResult)
	
	// Test 2: * with !. negation
	fmt.Println("\nTest 2: Pattern '*' with '!.'")
	writeGitignore(tempDir, `*
!.`)
	
	dotResult = gitCheckIgnore(tempDir, ".")
	fmt.Printf("  check-ignore '.': %s", dotResult)
	
	fileResult = gitCheckIgnore(tempDir, "file1.txt")
	fmt.Printf("  check-ignore 'file1.txt': %s", fileResult)
}

func testContentsOnlyDetailed(tempDir string) {
	fmt.Println("\n=== CONTENTS-ONLY DETAILED TEST ===")
	
	// Create build directory structure
	buildDir := filepath.Join(tempDir, "build")
	os.MkdirAll(filepath.Join(buildDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(buildDir, "file.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(buildDir, "subdir", "nested.txt"), []byte("content"), 0644)
	
	// Test build/** pattern
	writeGitignore(tempDir, "build/**")
	
	// Add files to git first so we can test properly
	exec.Command("git", "add", ".").Dir = tempDir
	
	fmt.Println("Testing pattern 'build/**':")
	
	buildResult := gitCheckIgnore(tempDir, "build")
	fmt.Printf("  check-ignore 'build': %s", buildResult)
	
	fileResult := gitCheckIgnore(tempDir, "build/file.txt")  
	fmt.Printf("  check-ignore 'build/file.txt': %s", fileResult)
	
	nestedResult := gitCheckIgnore(tempDir, "build/subdir/nested.txt")
	fmt.Printf("  check-ignore 'build/subdir/nested.txt': %s", nestedResult)
	
	subdirResult := gitCheckIgnore(tempDir, "build/subdir")
	fmt.Printf("  check-ignore 'build/subdir': %s", subdirResult)
}

func testDotfiles(tempDir string) {
	fmt.Println("\n=== DOTFILE BEHAVIOR TEST ===")
	
	// Create dotfiles
	os.WriteFile(filepath.Join(tempDir, ".hidden"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tempDir, "visible.txt"), []byte("content"), 0644)
	
	// Test different patterns
	patterns := []string{"*", "/*", "**"}
	
	for _, pattern := range patterns {
		fmt.Printf("\nTesting pattern '%s':\n", pattern)
		writeGitignore(tempDir, pattern)
		
		hiddenResult := gitCheckIgnore(tempDir, ".hidden")
		fmt.Printf("  check-ignore '.hidden': %s", hiddenResult)
		
		visibleResult := gitCheckIgnore(tempDir, "visible.txt")
		fmt.Printf("  check-ignore 'visible.txt': %s", visibleResult)
	}
}

func writeGitignore(tempDir, content string) {
	gitignorePath := filepath.Join(tempDir, ".gitignore")
	os.WriteFile(gitignorePath, []byte(content), 0644)
}

func gitCheckIgnore(repoDir, path string) string {
	cmd := exec.Command("git", "check-ignore", "-v", path)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	
	if err != nil && strings.Contains(err.Error(), "exit status 1") {
		return "NOT IGNORED\n"
	}
	if err != nil {
		return fmt.Sprintf("ERROR: %v\n", err)  
	}
	
	return fmt.Sprintf("IGNORED: %s", strings.TrimSpace(string(output)))
}