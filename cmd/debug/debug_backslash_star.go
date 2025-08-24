package main

import (
	"fmt"
	"github.com/bmatcuk/doublestar/v4"
)

func main() {
	// Test different backslash-star combinations
	patterns := []string{
		"path\\*",     // escaped star (literal)
		"path\\\\*",   // escaped backslash + wildcard star  
		"path\\\\\\*", // escaped backslash + escaped star
	}
	
	paths := []string{
		"path*",        // literal star
		"path\\*",      // backslash + literal star
		"path\\anything", // backslash + any text
		"pathanything", // no backslash, any text
	}
	
	for _, pattern := range patterns {
		fmt.Printf("\nPattern: %q\n", pattern)
		for _, path := range paths {
			matched, _ := doublestar.Match(pattern, path)
			fmt.Printf("  %q -> %v\n", path, matched)
		}
	}
}