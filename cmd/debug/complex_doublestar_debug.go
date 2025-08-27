package main

import (
	"fmt"

	gitignore "github.com/idelchi/go-gitignore"
)

func main() {
	fmt.Println("Testing pattern 0**/**/* with 0:")
	
	g := gitignore.New("0**/**/*")
	
	testCases := []struct{
		path string
		isDir bool
		expected bool
		description string
	}{
		{"0", false, true, "Should match 0 (file)"},
		{"0", true, true, "Should match 0 (directory)"},
		{"0/file", false, true, "Should match 0/file"},
		{"0/dir", true, true, "Should match 0/dir"},
		{"00", false, true, "Should match 00"},
		{"01", false, true, "Should match 01"},
		{"0abc", false, true, "Should match 0abc"},
		{"0/x/file", false, true, "Should match 0/x/file"},
		{"notmatch", false, false, "Should NOT match notmatch"},
	}
	
	for _, tc := range testCases {
		result := g.Ignored(tc.path, tc.isDir)
		status := "✓"
		if result != tc.expected {
			status = "✗"
		}
		fmt.Printf("  %s %s (dir: %v) -> %v (expected: %v) - %s\n", 
			status, tc.path, tc.isDir, result, tc.expected, tc.description)
	}
}