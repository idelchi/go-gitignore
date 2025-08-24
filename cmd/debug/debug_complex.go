package main

import (
	"fmt"
	"github.com/bmatcuk/doublestar/v4"
)

func main() {
	// Test complex pattern with escaped asterisk
	pattern := "a/**/\\*.log"
	
	paths := []string{
		"a/b/c/*.log",     // Should match
		"a/b/c/error.log", // Should NOT match
		"a/*.log",         // Should match
		"a/sub/*.log",     // Should match  
	}
	
	fmt.Printf("Pattern: %q\n", pattern)
	for _, path := range paths {
		matched, _ := doublestar.Match(pattern, path)
		fmt.Printf("  %q -> %v\n", path, matched)
	}
}