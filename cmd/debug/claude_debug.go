package main

import (
	"fmt"

	gitignore "github.com/idelchi/go-gitignore"
)

func main() {
	// Test the specific problematic pattern with various paths
	fmt.Println("Testing pattern a**/0 with a1/x/0:")
	g2 := gitignore.New("a**/0")
	
	testCases := []struct{
		path string
		isDir bool
		expected bool
	}{
		{"a0", false, true},
		{"a/0", false, true},
		{"a/b/0", false, true},
		{"a1/x/0", false, true},
		{"a1/x", false, false},
	}
	
	for _, tc := range testCases {
		result := g2.Ignored(tc.path, tc.isDir)
		status := "✓"
		if result != tc.expected {
			status = "✗"
		}
		fmt.Printf("  %s %s (dir: %v) -> %v (expected: %v)\n", status, tc.path, tc.isDir, result, tc.expected)
	}
}