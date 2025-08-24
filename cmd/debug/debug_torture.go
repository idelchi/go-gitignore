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
		fmt.Printf("  Processing i=%d, char=%q\n", i, string(pattern[i]))
		if pattern[i] == '\\' && i+1 < len(pattern) {
			next := pattern[i+1]
			fmt.Printf("    Found backslash, next=%q\n", string(next))
			switch next {
			case '*', '?', '[', ']':
				// These wildcards need special handling to be treated literally
				// We'll use a different approach - keep them escaped for doublestar
				fmt.Printf("    Escaped wildcard case\n")
				result.WriteByte('\\')
				result.WriteByte(next)
				i++ // Skip the next character
			case '#', '{', '}', '!':
				// These are escaped special chars - remove backslash for literal matching
				fmt.Printf("    Escaped special char case\n")
				result.WriteByte(next)
				i++ // Skip the next character
			case '\\':
				// Double backslash - check what follows
				fmt.Printf("    Double backslash case, i+2=%d, len=%d\n", i+2, len(pattern))
				if i+2 < len(pattern) {
					fmt.Printf("    char after second backslash: %q\n", string(pattern[i+2]))
				}
				if i+2 < len(pattern) && (pattern[i+2] == '*' || pattern[i+2] == '?' || pattern[i+2] == '[') {
					// This is \\* or \\? or \\[ - don't modify, let doublestar handle
					fmt.Printf("    Keeping double backslash before wildcard\n")
					result.WriteByte('\\')
					result.WriteByte('\\')
					i++ // Skip the second backslash, wildcard will be processed next
				} else {
					// Regular double backslash becomes single
					fmt.Printf("    Converting double backslash to single\n")
					result.WriteByte('\\')
					i++
				}
			default:
				// Keep backslash for other cases
				fmt.Printf("    Default case, keeping backslash\n")
				result.WriteByte(pattern[i])
			}
		} else {
			fmt.Printf("    Regular char, adding %q\n", string(pattern[i]))
			result.WriteByte(pattern[i])
		}
		fmt.Printf("    Result so far: %q\n", result.String())
	}
	
	return result.String()
}

func main() {
	// Test the specific failing case - use raw string to see actual bytes
	pattern := `path\\*`  // raw string: path + backslash + backslash + star
	fmt.Printf("Original pattern: %q (len=%d)\n", pattern, len(pattern))
	for i, b := range []byte(pattern) {
		fmt.Printf("  [%d] = %q (0x%02x)\n", i, string(b), b)
	}
	
	hasWildcards := hasUnescapedWildcards(pattern)
	fmt.Printf("Has unescaped wildcards: %v\n", hasWildcards)
	
	processed := processPatternEscapes(pattern)
	fmt.Printf("After processPatternEscapes: %q (len=%d)\n", processed, len(processed))
	for i, b := range []byte(processed) {
		fmt.Printf("  [%d] = %q (0x%02x)\n", i, string(b), b)
	}
	
	// Test direct doublestar matching
	fmt.Println("Direct doublestar test:")
	matched1, _ := doublestar.Match(processed, `path\anything`)
	matched2, _ := doublestar.Match(`path\\*`, `path\anything`)  
	fmt.Printf("  %q vs path\\anything -> %v\n", processed, matched1)
	fmt.Printf("  path\\\\* vs path\\anything -> %v\n", matched2)
	
	// Test with GitIgnore
	gi := gitignore.New([]string{pattern})
	fmt.Printf("GitIgnore patterns: %q\n", gi.Patterns())
	
	result := gi.Ignored(`path\anything`, false)
	fmt.Printf("GitIgnore result: path\\anything -> %v\n", result)
}