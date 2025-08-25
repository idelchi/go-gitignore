package main

import (
	"fmt"
	"strings"
	"path"
	gitignore "github.com/idelchi/go-gitignore"
)

func debugIgnored(patterns []string, testPath string, isDir bool) {
	gi := gitignore.New(patterns)
	
	fmt.Printf("\n=== Debugging: %s (isDir=%v) ===\n", testPath, isDir)
	fmt.Printf("Patterns: %q\n", patterns)
	
	// Simulate findExcludedParentDirectories logic
	parts := strings.Split(testPath, "/")
	fmt.Println("\nParent paths to check:")
	for i := 1; i <= len(parts); i++ {
		parentPath := strings.Join(parts[:i], "/")
		if parentPath != testPath {
			fmt.Printf("  - %s\n", parentPath)
		}
	}
	
	// Check what matches
	fmt.Println("\nPattern matching analysis:")
	
	// For bare pattern "b", it should match:
	// - "b" as a basename
	// - Any directory whose basename is "b"
	// But after "!a/b/" re-includes a/b/, the children should NOT be matched
	
	result := gi.Ignored(testPath, isDir)
	fmt.Printf("\nFinal result: %v\n", result)
	
	// Check if parent is excluded
	parentPath := path.Dir(testPath)
	if parentPath != "." && parentPath != testPath {
		parentResult := gi.Ignored(parentPath, true)
		fmt.Printf("Parent %s ignored: %v\n", parentPath, parentResult)
	}
}

func main() {
	patterns := []string{"b", "!a/b/"}
	
	// This should NOT be ignored because a/b/ is re-included
	debugIgnored(patterns, "a/b/c", true)
	
	// This SHOULD be ignored because x/b is not re-included
	debugIgnored(patterns, "x/b/c", true)
	
	// Edge case: what about deeper nesting?
	debugIgnored(patterns, "a/b/c/d", true)
}