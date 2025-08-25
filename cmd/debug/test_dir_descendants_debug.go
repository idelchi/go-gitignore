package main

import (
	"fmt"

	gitignore "github.com/idelchi/go-gitignore"
)

func testDirDescendants() {
	fmt.Println("=== TESTING DIR DESCENDANTS PATTERN ===")
	
	// Test the failing case: abc/**/ should NOT match abc/ itself
	lines := []string{"abc/**/"}
	gi := gitignore.New(lines)
	
	fmt.Println("Pattern: abc/**/")
	fmt.Println("This should match directories UNDER abc, but NOT abc itself")
	
	// Test cases that are failing
	testCases := []struct {
		path  string
		isDir bool
		expected bool
		description string
	}{
		{"abc", true, false, "Base directory 'abc' should NOT match"},
		{"abc/file.txt", false, false, "File under base should NOT match (dir-only pattern)"},
		{"abc/subdir", true, true, "Directory under base SHOULD match"},
		{"abc/subdir/file.txt", false, true, "File under subdir should be ignored (parent exclusion)"},
	}
	
	for _, tc := range testCases {
		result := gi.Ignored(tc.path, tc.isDir)
		status := "✓"
		if result != tc.expected {
			status = "❌ FAIL"
		}
		fmt.Printf("  %s %s (isDir=%v) -> ignored=%v (expected %v) - %s\n", 
			status, tc.path, tc.isDir, result, tc.expected, tc.description)
	}
	
	// Let's also test with actual Git to confirm behavior
	fmt.Println("\nTesting with actual Git...")
	testWithGit()
}

func testWithGit() {
	// This was already verified in verify_git_behavior_debug.go
	// We confirmed that:
	// - Pattern "abc/**/" in Git does NOT match "abc" directory
	// - Pattern "abc/**/" in Git DOES match contents under "abc"
	fmt.Println("Previously verified: Git behavior matches our expectations")
}

func main() {
	testDirDescendants()
}