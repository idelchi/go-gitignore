package main

import (
	"fmt"

	gitignore "github.com/idelchi/go-gitignore"
)

func main() {
	fmt.Println("Testing pattern *?/0 with 0/x/0:")
	
	// Test the exact pattern
	g := gitignore.New("*?/0")
	
	testCases := []struct{
		path string
		isDir bool
		expected bool
	}{
		{"0/0", false, true},      // *? should match "0", so this should match
		{"ab/0", false, true},     // *? should match "ab", so this should match  
		{"abc/0", false, true},    // *? should match "abc", so this should match
		{"0/x/0", false, false},   // This should NOT match - why?
		{"/0/0", false, false},    // Rooted path shouldn't match non-rooted pattern
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