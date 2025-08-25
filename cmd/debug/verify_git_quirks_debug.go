package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Before removing "special cases", let's verify which ones are actual Git quirks
func main() {
	fmt.Println("=== VERIFYING GIT QUIRKS vs OVERFITTING ===")

	tempDir := createTempRepo()
	defer os.RemoveAll(tempDir)

	// Test 1: The "." special case - is this really a Git quirk?
	testDotNegation(tempDir)

	// Test 2: Dotfile handling inconsistency - rooted /* vs unrooted *
	testDotfileHandling(tempDir)

	// Test 3: Sandwich patterns with wildcards - does Git really reject them?
	testSandwichWildcards(tempDir)

	// Test 4: Contents-only patterns - verify this is real Git behavior
	testContentsOnly(tempDir)
}

func createTempRepo() string {
	tempDir, _ := os.MkdirTemp("", "git-verify-*")
	exec.Command("git", "init").Dir = tempDir
	return tempDir
}

func testDotNegation(tempDir string) {
	fmt.Println("\n1. DOT NEGATION TEST:")
	fmt.Println("   Testing if '!.' can re-include current directory")

	// Test patterns from the special case
	gitignoreContent := `*
!.`
	writeGitignore(tempDir, gitignoreContent)

	result := gitCheckIgnore(tempDir, ".")
	fmt.Printf("   Git result for '.': %s", result)

	if strings.Contains(result, "IGNORED") {
		fmt.Println("   ✅ LEGITIMATE: Git does ignore '.' despite '!.'")
	} else {
		fmt.Println("   ❌ OVERFITTED: Git allows '!.' to work")
	}
}

func testDotfileHandling(tempDir string) {
	fmt.Println("\n2. DOTFILE HANDLING INCONSISTENCY TEST:")

	// Create test dotfile
	os.WriteFile(filepath.Join(tempDir, ".hidden"), []byte("test"), 0o644)

	// Test rooted /*
	fmt.Println("   Testing rooted /* pattern:")
	writeGitignore(tempDir, "/*")
	result := gitCheckIgnore(tempDir, ".hidden")
	fmt.Printf("   Git result for '.hidden' with '/*': %s", result)

	// Test unrooted *
	fmt.Println("   Testing unrooted * pattern:")
	writeGitignore(tempDir, "*")
	result = gitCheckIgnore(tempDir, ".hidden")
	fmt.Printf("   Git result for '.hidden' with '*': %s", result)
}

func testSandwichWildcards(tempDir string) {
	fmt.Println("\n3. SANDWICH PATTERNS WITH WILDCARDS:")
	fmt.Println("   Testing if Git rejects wildcards in sandwich middle")

	// Create test structure
	os.MkdirAll(filepath.Join(tempDir, "src/node_modules_test/pkg"), 0o755)
	os.WriteFile(filepath.Join(tempDir, "src/node_modules_test/file.js"), []byte("test"), 0o644)

	// Test sandwich with wildcard in middle
	writeGitignore(tempDir, "**/node_*/**")
	result := gitCheckIgnore(tempDir, "src/node_modules_test/file.js")
	fmt.Printf("   Git result for sandwich with wildcard: %s", result)

	if strings.Contains(result, "IGNORED") {
		fmt.Println("   ❌ OVERFITTED: Git accepts wildcards in sandwich middle")
	} else {
		fmt.Println("   ✅ LEGITIMATE: Git rejects wildcards in sandwich middle")
	}
}

func testContentsOnly(tempDir string) {
	fmt.Println("\n4. CONTENTS-ONLY PATTERNS:")
	fmt.Println("   Verifying build/** behavior")

	os.MkdirAll(filepath.Join(tempDir, "build/sub"), 0o755)
	os.WriteFile(filepath.Join(tempDir, "build/file.txt"), []byte("test"), 0o644)

	writeGitignore(tempDir, "build/**")

	baseResult := gitCheckIgnore(tempDir, "build")
	fmt.Printf("   Git result for 'build': %s", baseResult)

	contentResult := gitCheckIgnore(tempDir, "build/file.txt")
	fmt.Printf("   Git result for 'build/file.txt': %s", contentResult)

	if !strings.Contains(baseResult, "IGNORED") && strings.Contains(contentResult, "IGNORED") {
		fmt.Println("   ✅ LEGITIMATE: Git has contents-only behavior")
	} else {
		fmt.Println("   ❌ UNEXPECTED: Git behavior differs from expectation")
	}
}

func writeGitignore(tempDir, content string) {
	gitignorePath := filepath.Join(tempDir, ".gitignore")
	os.WriteFile(gitignorePath, []byte(content), 0o644)
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

	return fmt.Sprintf("IGNORED: %s\n", strings.TrimSpace(string(output)))
}
