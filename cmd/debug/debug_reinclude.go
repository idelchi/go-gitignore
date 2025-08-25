package main

import (
	"fmt"
	gitignore "github.com/idelchi/go-gitignore"
)

func main() {
	fmt.Println("=== Testing re-inclusion logic ===")
	
	// Test case 1: basename_no_spillover_after_reinclude
	patterns1 := []string{"b", "!a/b/"}
	gi1 := gitignore.New(patterns1)
	
	fmt.Printf("\nPatterns: %q\n", patterns1)
	fmt.Println("Test: a/b/c/ (isDir=true)")
	
	// This should be false but returns true
	result1 := gi1.Ignored("a/b/c/", true)
	fmt.Printf("Result: %v (expected: false)\n", result1)
	
	// Let's test parent paths
	fmt.Println("\nTesting related paths:")
	testPaths := []struct{
		path string
		isDir bool
	}{
		{"b", true},
		{"b", false},
		{"a/b", true},
		{"a/b", false},
		{"a/b/", true},
		{"a/b/c", true},
		{"a/b/c", false},
		{"a/b/c/", true},
		{"x/b", true},
		{"x/b/c", true},
	}
	
	for _, tp := range testPaths {
		result := gi1.Ignored(tp.path, tp.isDir)
		fmt.Printf("  %s (isDir=%v) -> %v\n", tp.path, tp.isDir, result)
	}
	
	// Test case 2: with wildcard
	fmt.Println("\n--- Test with wildcard ---")
	patterns2 := []string{"b*", "!a/b/"}
	gi2 := gitignore.New(patterns2)
	fmt.Printf("Patterns: %q\n", patterns2)
	
	for _, tp := range testPaths {
		result := gi2.Ignored(tp.path, tp.isDir)
		fmt.Printf("  %s (isDir=%v) -> %v\n", tp.path, tp.isDir, result)
	}
}