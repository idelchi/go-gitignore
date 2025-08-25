package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// verifyGitBehavior tests special cases against actual Git
func verifyGitBehavior() {
	fmt.Println("=== VERIFYING GIT BEHAVIOR ===")

	// Create temporary test repository
	tempDir := createTempRepo()
	defer os.RemoveAll(tempDir)

	// Test 1: Dot negation behavior (lines 272-276 in gitignore.go)
	fmt.Println("\n1. Testing dot negation behavior:")
	testDotNegation(tempDir)

	// Test 2: Leading double slash behavior (lines 231-233)
	fmt.Println("\n2. Testing double slash behavior:")
	testDoubleSlash(tempDir)

	// Test 3: Contents-only patterns (**/ behavior)
	fmt.Println("\n3. Testing contents-only patterns:")
	testContentsOnly(tempDir)

	// Test 4: Sandwich patterns (**/middle/**)
	fmt.Println("\n4. Testing sandwich patterns:")
	testSandwichPatterns(tempDir)
}

func createTempRepo() string {
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		return ""
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to init git repo: %v\n", err)
		return ""
	}

	return tempDir
}

func testDotNegation(tempDir string) {
	// Create .gitignore with dot patterns
	gitignoreContent := `*
!.
!..
!...`

	gitignorePath := filepath.Join(tempDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0o644); err != nil {
		fmt.Printf("Failed to write .gitignore: %v\n", err)
		return
	}

	// Test Git's behavior with "."
	result := gitCheckIgnore(tempDir, ".")
	fmt.Printf("   Git check-ignore '.': %s", result)

	// Test with ".."
	result = gitCheckIgnore(tempDir, "..")
	fmt.Printf("   Git check-ignore '..': %s", result)
}

func testDoubleSlash(tempDir string) {
	gitignoreContent := `*`
	gitignorePath := filepath.Join(tempDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0o644); err != nil {
		return
	}

	// Test paths with double slash
	result := gitCheckIgnore(tempDir, "//test")
	fmt.Printf("   Git check-ignore '//test': %s", result)

	result = gitCheckIgnore(tempDir, "///test")
	fmt.Printf("   Git check-ignore '///test': %s", result)
}

func testContentsOnly(tempDir string) {
	gitignoreContent := `build/**`
	gitignorePath := filepath.Join(tempDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0o644); err != nil {
		return
	}

	// Create test files
	os.MkdirAll(filepath.Join(tempDir, "build/sub"), 0o755)
	os.WriteFile(filepath.Join(tempDir, "build/file.txt"), []byte("test"), 0o644)

	// Test if base directory is ignored
	result := gitCheckIgnore(tempDir, "build")
	fmt.Printf("   Git check-ignore 'build': %s", result)

	// Test if contents are ignored
	result = gitCheckIgnore(tempDir, "build/file.txt")
	fmt.Printf("   Git check-ignore 'build/file.txt': %s", result)
}

func testSandwichPatterns(tempDir string) {
	gitignoreContent := `**/node_modules/**`
	gitignorePath := filepath.Join(tempDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0o644); err != nil {
		return
	}

	// Create test structure
	os.MkdirAll(filepath.Join(tempDir, "src/node_modules/pkg"), 0o755)
	os.WriteFile(filepath.Join(tempDir, "src/node_modules/file.js"), []byte("test"), 0o644)

	// Test base directory
	result := gitCheckIgnore(tempDir, "src/node_modules")
	fmt.Printf("   Git check-ignore 'src/node_modules': %s", result)

	// Test contents
	result = gitCheckIgnore(tempDir, "src/node_modules/file.js")
	fmt.Printf("   Git check-ignore 'src/node_modules/file.js': %s", result)
}

func gitCheckIgnore(repoDir, path string) string {
	cmd := exec.Command("git", "check-ignore", "-v", path)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()

	result := strings.TrimSpace(string(output))
	if err != nil {
		if strings.Contains(err.Error(), "exit status 1") {
			return "NOT IGNORED\n"
		}
		return fmt.Sprintf("ERROR: %v\n", err)
	}

	return fmt.Sprintf("IGNORED: %s\n", result)
}

func main() {
	verifyGitBehavior()

	fmt.Println("\n=== CONCLUSIONS ===")
	fmt.Println("Use the results above to determine which special cases are:")
	fmt.Println("✓ LEGITIMATE - Match actual Git behavior (keep these)")
	fmt.Println("❌ OVERFITTED - Only exist to pass tests (remove these)")
}
