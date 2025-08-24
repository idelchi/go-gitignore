package main

import (
	"fmt"
	gitignore "github.com/idelchi/go-gitignore"
	"github.com/bmatcuk/doublestar/v4"
)

func hasUnescapedWildcards(pattern string) bool {
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			// Skip escaped character
			i++
			continue
		}
		// Check for unescaped wildcards
		if pattern[i] == '*' || pattern[i] == '?' || pattern[i] == '[' {
			return true
		}
	}
	return false
}

func main() {
	// Test the escaped wildcard case
	gi := gitignore.New([]string{"a/**/\\*.log"})
	patterns := gi.Patterns()
	fmt.Printf("Original patterns: %q\n", patterns)
	
	// Debug: Check if pattern is detected as having unescaped wildcards
	testPattern := "a/**/\\*.log"
	hasWildcards := hasUnescapedWildcards(testPattern)
	fmt.Printf("Pattern has unescaped wildcards: %v\n", hasWildcards)
	
	// Test direct doublestar matching
	fmt.Println("Direct doublestar test:")
	matched1, _ := doublestar.Match("a/**/\\*.log", "a/b/c/*.log")
	matched2, _ := doublestar.Match("a/**/\\*.log", "a/b/c/error.log")
	fmt.Printf("  a/b/c/*.log -> %v\n", matched1)
	fmt.Printf("  a/b/c/error.log -> %v\n", matched2)
	
	// Test cases
	testCases := []string{
		"a/b/c/*.log",     // Should match (literal * in filename)
		"a/b/c/error.log", // Should NOT match (no literal * in filename)
	}
	
	fmt.Println("GitIgnore test:")
	for _, path := range testCases {
		result := gi.Ignored(path, false)
		fmt.Printf("  Path: %q, Ignored: %v\n", path, result)
	}
}