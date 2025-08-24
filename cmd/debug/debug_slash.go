package main

import (
	"fmt"
	gitignore "github.com/idelchi/go-gitignore"
)

func main() {
	// Test ambiguous slash case
	patterns := []string{"/a/b/", "a/b/", "/a/b", "a/b", "a//b", "a/b//", "//a/b"}
	gi := gitignore.New(patterns)
	
	fmt.Printf("Patterns: %q\n", patterns)
	
	testCases := []string{
		"a/b/",
		"x/a/b/", 
		"a//b",
		"//a/b",
	}
	
	for _, path := range testCases {
		result := gi.Ignored(path, false)
		fmt.Printf("Path: %q -> Ignored: %v\n", path, result)
	}
}