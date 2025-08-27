package main

import (
	"fmt"
	"github.com/bmatcuk/doublestar/v4"
)

func main() {
	pattern := "a/**/0"
	target := "a1/x/0"
	
	fmt.Printf("Pattern: %s\n", pattern)
	fmt.Printf("Target: %s\n", target)
	
	result := doublestar.MatchUnvalidated(pattern, target)
	fmt.Printf("Match result: %v\n", result)
	
	// Let's test what Git's a**/0 actually translates to
	patterns := []string{
		"a/**/0",      // Our current multi-level attempt
		"a**/0",       // Original pattern
		"a*/0",        // Single level  
		"a0",          // Zero width
		"a*/**/0",     // Alternative multi-level attempt
		"a*/*/0",      // Alternative single-level attempt
		"a/**/*0",     // Different multi-level pattern
	}
	
	for _, p := range patterns {
		result := doublestar.MatchUnvalidated(p, target)
		fmt.Printf("Pattern '%s' matches '%s': %v\n", p, target, result)
	}
}