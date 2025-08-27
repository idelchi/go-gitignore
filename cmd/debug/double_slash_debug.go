package main

import (
	"fmt"

	gitignore "github.com/idelchi/go-gitignore"
)

func main() {
	fmt.Println("Testing pattern 0***// with path 0:")
	
	// Test the exact pattern
	g := gitignore.New("0***/")
	
	path := "0"
	isDir := true
	
	fmt.Printf("Pattern: 0***/\n")
	fmt.Printf("Testing path: %s (dir: %v)\n", path, isDir)
	result := g.Ignored(path, isDir)
	fmt.Printf("Result: %v (expected: true)\n", result)
	
	// Let's also understand what the // pattern means
	fmt.Println("\nTesting various patterns with 0:")
	
	patterns := []string{
		"0**",
		"0**/",
		"0**//",
		"0***/",
		"0**/**",
	}
	
	for _, pattern := range patterns {
		g := gitignore.New(pattern)
		result := g.Ignored("0", true)
		fmt.Printf("Pattern '%s' matches '0' (dir): %v\n", pattern, result)
	}
}