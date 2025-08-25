package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// analyzeOverfittingPatterns examines the current implementation for overfitting indicators
func analyzeOverfittingPatterns() {
	fmt.Println("=== OVERFITTING ANALYSIS ===")

	// 1. Check for pattern classification explosion
	fmt.Println("\n1. Pattern Classification Analysis:")
	fmt.Println("   Current implementation has:")
	fmt.Println("   - formBareAnyDir: bool")
	fmt.Println("   - formDirDescendants: bool")
	fmt.Println("   - formContentsOnly: bool")
	fmt.Println("   - formSandwich: bool")
	fmt.Println("   - sandwichMiddle: string")
	fmt.Println("   ⚠️  This suggests classification explosion (RED FLAG)")

	// 2. Check for special cases
	fmt.Println("\n2. Special Case Analysis:")
	fmt.Println("   Found special cases:")
	fmt.Println("   - Dot negation special case (lines 272-276)")
	fmt.Println("   - Leading double slash (lines 231-233)")
	fmt.Println("   - Wildcard handling for rooted vs unrooted (lines 905-928)")
	fmt.Println("   ⚠️  Multiple special cases suggest overfitting")

	// 3. Function proliferation
	fmt.Println("\n3. Function Proliferation Analysis:")
	funcs := []string{
		"matches", "matchesFilePattern", "patternExcludesDirectory",
		"isSandwichBase", "matchGlob", "matchRawGlob",
		"escapeBraces", "processEscapes", "hasUnescapedWildcards",
	}
	fmt.Printf("   Found %d specialized matching functions:\n", len(funcs))
	for _, f := range funcs {
		fmt.Printf("   - %s\n", f)
	}
	fmt.Println("   ⚠️  Many specialized functions suggest complexity explosion")

	// 4. Check complexity indicators
	fmt.Println("\n4. Complexity Indicators:")
	fmt.Println("   - gitignore.go: 1052 lines (VERY LARGE)")
	fmt.Println("   - Multiple pattern forms requiring different logic")
	fmt.Println("   - Nested conditions and switch statements")
	fmt.Println("   - Different matching algorithms for similar patterns")

	fmt.Println("\n5. Recommendations from avoid-overfitting.md:")
	fmt.Println("   ✓ Should have ONE general matching algorithm")
	fmt.Println("   ✓ Should trust the glob library more (doublestar)")
	fmt.Println("   ✓ Should remove pattern classifications")
	fmt.Println("   ✓ Should simplify escape processing")
	fmt.Println("   ✓ Should verify special cases against actual Git")
}

// testCurrentImplementation runs tests to establish baseline
func testCurrentImplementation() {
	fmt.Println("\n=== CURRENT IMPLEMENTATION TEST ===")

	cmd := exec.Command("go", "test", "-run", "TestGitIgnore", ".")
	cmd.Dir = filepath.Dir(filepath.Dir(getCurrentDir()))
	output, err := cmd.CombinedOutput()

	fmt.Printf("Test result: %s\n", string(output))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

// getCurrentDir gets current directory
func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func main() {
	analyzeOverfittingPatterns()
	testCurrentImplementation()

	fmt.Println("\n=== NEXT STEPS ===")
	fmt.Println("1. Simplify pattern struct - remove boolean flags")
	fmt.Println("2. Create single unified matching algorithm")
	fmt.Println("3. Trust doublestar library more, reduce custom logic")
	fmt.Println("4. Verify special cases against actual Git behavior")
	fmt.Println("5. Run tests after each simplification")
}
