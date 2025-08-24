package main

import (
	"fmt"
	gitignore "github.com/idelchi/go-gitignore"
	"github.com/bmatcuk/doublestar/v4"
	"strings"
)

func hasUnescapedWildcards(pattern string) bool {
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			// Skip escaped character
			i++
			continue
		}
		// Check for unescaped wildcards
		if pattern[i] == '*' || pattern[i] == '?' || pattern[i] == '[' {
			return true
		}
	}
	return false
}

func processPatternEscapes(pattern string) string {
	if pattern == "" {
		return pattern
	}
	
	var result strings.Builder
	result.Grow(len(pattern) + 10) // Extra space for potential escaping
	
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			next := pattern[i+1]
			switch next {
			case '*', '?', '[', ']':
				// These wildcards need special handling to be treated literally
				// We'll use a different approach - keep them escaped for doublestar
				result.WriteByte('\\')
				result.WriteByte(next)
				i++ // Skip the next character
			case '#', '{', '}', '!':
				// These are escaped special chars - remove backslash for literal matching
				result.WriteByte(next)
				i++ // Skip the next character
			case '\\':
				// Double backslash becomes single
				result.WriteByte('\\')
				i++
			default:
				// Keep backslash for other cases
				result.WriteByte(pattern[i])
			}
		} else {
			result.WriteByte(pattern[i])
		}
	}
	
	return result.String()
}

func main() {
	pattern := "\\*.txt"
	
	// Debug step by step
	fmt.Printf("Original pattern: %q\n", pattern)
	
	hasWildcards := hasUnescapedWildcards(pattern)
	fmt.Printf("Has unescaped wildcards: %v\n", hasWildcards)
	
	processed := processPatternEscapes(pattern)
	fmt.Printf("After processPatternEscapes: %q\n", processed)
	
	// Test simple escaped wildcard case
	gi := gitignore.New([]string{pattern})
	patterns := gi.Patterns()
	fmt.Printf("GitIgnore patterns: %q\n", patterns)
	
	// Test direct doublestar matching
	fmt.Println("\nDirect doublestar test:")
	matched1, _ := doublestar.Match("\\*.txt", "*.txt")
	matched2, _ := doublestar.Match("\\*.txt", "error.txt")
	fmt.Printf("  \\*.txt vs *.txt -> %v\n", matched1)
	fmt.Printf("  \\*.txt vs error.txt -> %v\n", matched2)
	
	// Test GitIgnore
	fmt.Println("\nGitIgnore test:")
	result1 := gi.Ignored("*.txt", false)
	result2 := gi.Ignored("error.txt", false)
	fmt.Printf("  *.txt -> %v\n", result1)
	fmt.Printf("  error.txt -> %v\n", result2)
}