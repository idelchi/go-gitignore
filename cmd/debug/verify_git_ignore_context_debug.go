package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	fmt.Println("=== VERIFYING GIT CONTEXT FOR DOTFILE BEHAVIOR ===")

	tempDir := createTempRepo()
	defer os.RemoveAll(tempDir)

	// Test different scenarios
	testDotfileScenarios(tempDir)
}

func createTempRepo() string {
	tempDir, _ := os.MkdirTemp("", "git-context-*")
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	cmd.Run()

	exec.Command("git", "config", "user.email", "test@example.com").Dir = tempDir
	exec.Command("git", "config", "user.name", "Test User").Dir = tempDir

	return tempDir
}

func testDotfileScenarios(tempDir string) {
	// Create various dotfiles
	dotfiles := []string{".gitignore", ".env", ".secret", ".DS_Store"}
	for _, file := range dotfiles {
		os.WriteFile(filepath.Join(tempDir, file), []byte("content"), 0o644)
	}

	// Also create regular files
	regularFiles := []string{"file.txt", "README.md"}
	for _, file := range regularFiles {
		os.WriteFile(filepath.Join(tempDir, file), []byte("content"), 0o644)
	}

	scenarios := []struct {
		pattern string
		desc    string
	}{
		{"/*", "rooted wildcard"},
		{"*", "unrooted wildcard"},
		{".gitignore", "literal .gitignore"},
		{".*", "dotfile pattern"},
	}

	for _, scenario := range scenarios {
		fmt.Printf("\n--- Testing pattern '%s' (%s) ---\n", scenario.pattern, scenario.desc)

		// Write pattern to .gitignore
		gitignorePath := filepath.Join(tempDir, ".gitignore")
		os.WriteFile(gitignorePath, []byte(scenario.pattern), 0o644)

		// Test all files
		allFiles := append(dotfiles, regularFiles...)
		for _, testFile := range allFiles {
			result := gitCheckIgnore(tempDir, testFile)
			status := "NOT IGNORED"
			if strings.Contains(result, "IGNORED") {
				status = "IGNORED"
			}
			fmt.Printf("  %s: %s\n", testFile, status)
		}
	}

	// Test the specific failing pattern from test suite
	fmt.Println("\n=== REPRODUCING EXACT TEST SCENARIO ===")
	fmt.Println("This matches the failing test: rooted_star_vs_dotfiles")

	gitignorePath := filepath.Join(tempDir, ".gitignore")
	os.WriteFile(gitignorePath, []byte("/*"), 0o644)

	result := gitCheckIgnore(tempDir, ".gitignore")
	fmt.Printf("git check-ignore '.gitignore' with pattern '/*': %s\n", result)

	if strings.Contains(result, "IGNORED") {
		fmt.Println("Git says: .gitignore IS ignored by /*")
		fmt.Println("Test expects: .gitignore should NOT be ignored by /*")
		fmt.Println("CONCLUSION: Test expectation seems wrong OR there's special context")
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
