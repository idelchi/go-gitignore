package main

import (
	"fmt"

	gitignore "github.com/idelchi/go-gitignore"
)

func main() {
	// Test the exact failing case from the fuzz test
	patterns := []string{
		"**/*.tmp",
		"!data/**/",
		"!data/**/", 
		"*/cache/",
		"a**",
	}
	
	fmt.Println("Testing original fuzz failure case:")
	fmt.Printf("Patterns: %v\n", patterns)
	
	g := gitignore.New(patterns...)
	
	path := "a0"
	isDir := true
	
	fmt.Printf("Testing path: %s (dir: %v)\n", path, isDir)
	result := g.Ignored(path, isDir)
	fmt.Printf("Result: %v (expected: true)\n", result)
}