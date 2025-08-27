package main

import (
	"fmt"

	gitignore "github.com/idelchi/go-gitignore"
)

func main() {
	fmt.Println("Testing Unicode patterns:")
	
	testCases := []struct{
		pattern string
		target string
		expected bool
		description string
	}{
		{"?", "a", true, "Single ASCII char should match ?"},
		{"?", "é", false, "é is 2 bytes, should NOT match single ?"},
		{"??", "é", true, "é is 2 bytes, should match ??"},
		{"????", "🚀", true, "🚀 is 4 bytes, should match ????"},
		{"???", "🚀", false, "🚀 is 4 bytes, should NOT match ???"},
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