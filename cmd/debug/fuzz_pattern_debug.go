package main

import (
	"fmt"

	gitignore "github.com/idelchi/go-gitignore"
)

func main() {
	// Test the exact failing case from the fuzz test
	// patterns: [build/ 0***// !data/**/ !data/**/ !data/**/ !*.log !*.log]
	patterns := []string{
		"build/",
		"0***/", // This gets normalized to "0**" during processing
		"!data/**/",
		"!data/**/", 
		"!data/**/",
		"!*.log",
		"!*.log",
	}
	
	// Let's also test what the fuzz output actually showed
	patterns2 := []string{
		"build/",
		"0***/", // But maybe it was meant to be a different pattern
		"!data/**/",
		"!data/**/", 
		"!data/**/",
		"!*.log",
		"!*.log",
	}
	
	fmt.Println("Testing fuzz failure case:")
	fmt.Printf("Patterns: %v\n", patterns)
	
	g := gitignore.New(patterns...)
	
	path := "0"
	isDir := true
	
	fmt.Printf("Testing path: %s (dir: %v)\n", path, isDir)
	result := g.Ignored(path, isDir)
	fmt.Printf("Result: %v (expected: true)\n", result)
	
	// Also test the pattern in isolation
	fmt.Println("\nTesting just the 0***/ pattern:")
	g2 := gitignore.New("0***/")
	result2 := g2.Ignored(path, isDir)
	fmt.Printf("Result: %v (expected: true)\n", result2)
}