package main

import (
	"fmt"

	gitignore "github.com/idelchi/go-gitignore"
)

func main() {
	// Exact patterns from the failing test
	patterns := []string{
		"build/",
		"0***/", // This is the pattern with double slashes from fuzz test
		"!data/**/",
		"!data/**/", 
		"!data/**/",
		"!*.log",
		"!*.log",
	}
	
	fmt.Println("Testing exact failing fuzz case:")
	fmt.Printf("Patterns: %v\n", patterns)
	
	g := gitignore.New(patterns...)
	
	path := "0"
	isDir := true
	
	fmt.Printf("Testing path: %s (dir: %v)\n", path, isDir)
	result := g.Ignored(path, isDir)
	fmt.Printf("Result: %v (expected: true)\n", result)
	
	fmt.Println("\nLet's check individual pattern matching:")
	for i, pattern := range patterns {
		g := gitignore.New(pattern)
		result := g.Ignored(path, isDir)
		fmt.Printf("Pattern %d '%s' matches: %v\n", i, pattern, result)
	}
	
	fmt.Println("\nLet's test the exact failing pattern from test output:")
	g2 := gitignore.New("0***/")
	result2 := g2.Ignored(path, isDir)
	fmt.Printf("Pattern '0***//' matches: %v\n", result2)
}