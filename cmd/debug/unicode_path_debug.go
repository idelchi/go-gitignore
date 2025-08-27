package main

import (
	"fmt"

	gitignore "github.com/idelchi/go-gitignore"
)

func main() {
	fmt.Println("Testing Unicode path patterns:")
	
	testCases := []struct{
		pattern string
		target string
		expected bool
		description string
	}{
		{"src/??/file.js", "src/ex/file.js", true, "ASCII 2-char dir should match ??"},
		{"src/??/file.js", "src/éx/file.js", false, "éx is 3 bytes, should NOT match ??"},
		{"src/???/file.js", "src/éx/file.js", true, "éx is 3 bytes, should match ???"},
	}
	
	for _, tc := range testCases {
		g := gitignore.New(tc.pattern)
		result := g.Ignored(tc.target, false)
		status := "✓"
		if result != tc.expected {
			status = "✗"
		}
		fmt.Printf("  %s Pattern '%s' vs '%s': %v (expected: %v) - %s\n", 
			status, tc.pattern, tc.target, result, tc.expected, tc.description)
	}
}