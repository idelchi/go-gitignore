package main

import (
	"fmt"
	"github.com/bmatcuk/doublestar/v4"
)

func main() {
	pattern := "a**/00"
	prefix := "a"
	suffix := "/00"
	
	fmt.Printf("Original pattern: %s\n", pattern)
	fmt.Printf("Prefix: '%s', Suffix: '%s'\n", prefix, suffix)
	
	// Test the three variants
	zeroWidth := prefix + suffix[1:] // Remove the leading slash from suffix
	singleLevel := prefix + "*" + suffix
	multiLevel := prefix + "/**" + suffix // Standard doublestar pattern
	
	fmt.Printf("Zero-width: %s\n", zeroWidth)
	fmt.Printf("Single-level: %s\n", singleLevel)
	fmt.Printf("Multi-level: %s\n", multiLevel)
	
	targets := []string{"a00", "a/00", "a/b/00", "a/b", "a/b/file.txt"}
	
	for _, target := range targets {
		fmt.Printf("\nTesting target: %s\n", target)
		fmt.Printf("  Zero-width (%s): %v\n", zeroWidth, doublestar.MatchUnvalidated(zeroWidth, target))
		fmt.Printf("  Single-level (%s): %v\n", singleLevel, doublestar.MatchUnvalidated(singleLevel, target))
		fmt.Printf("  Multi-level (%s): %v\n", multiLevel, doublestar.MatchUnvalidated(multiLevel, target))
	}
}