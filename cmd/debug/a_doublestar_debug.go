package main

import (
	"fmt"

	gitignore "github.com/idelchi/go-gitignore"
)

func main() {
	fmt.Println("Testing pattern a** with a0:")
	g := gitignore.New("a**")
	
	testCases := []struct{
		path string
		isDir bool
		expected bool
	}{
		{"a", false, false},
		{"a", true, false},
		{"a0", false, true},
		{"a0", true, true},
		{"ab", false, true},
		{"ab", true, true},
		{"a/file", false, true},
		{"a/dir", true, true},
	}
	
	for _, tc := range testCases {
		result := g.Ignored(tc.path, tc.isDir)
		status := "✓"
		if result != tc.expected {
			status = "✗"
		}
		fmt.Printf("  %s %s (dir: %v) -> %v (expected: %v)\n", status, tc.path, tc.isDir, result, tc.expected)
	}
}