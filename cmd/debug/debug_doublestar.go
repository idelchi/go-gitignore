package main

import (
	"fmt"
	"github.com/bmatcuk/doublestar/v4"
)

func main() {
	// Test how doublestar handles escaped asterisks
	patterns := []string{
		"\\*",      // Escaped asterisk
		"*.log",    // Wildcard asterisk  
		"\\*.log",  // Escaped asterisk + literal
	}
	
	paths := []string{
		"*",         // Literal asterisk
		"*.log",     // Literal asterisk + literal
		"error.log", // No asterisk
		"test",      // No asterisk
	}
	
	for _, pattern := range patterns {
		fmt.Printf("\nPattern: %q\n", pattern)
		for _, path := range paths {
			matched, _ := doublestar.Match(pattern, path)
			fmt.Printf("  %q -> %v\n", path, matched)
		}
	}
}