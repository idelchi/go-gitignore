// Package gitignore_test provides examples demonstrating the gitignore package usage.
package gitignore_test

import (
	"fmt"

	gitignore "github.com/idelchi/go-gitignore"
)

// ExampleNew demonstrates basic gitignore pattern creation and usage.
func ExampleNew() {
	// Create a gitignore instance with common patterns
	gi := gitignore.New([]string{
		"*.log",        // Ignore all log files
		"build/",       // Ignore build directory
		"!important.log", // Don't ignore important.log
	})

	// Test various paths
	fmt.Println(gi.Ignored("app.log", false))        // true - matches *.log
	fmt.Println(gi.Ignored("important.log", false))  // false - negated by !important.log
	fmt.Println(gi.Ignored("build", true))           // true - matches build/ (directory)
	fmt.Println(gi.Ignored("src/main.go", false))    // false - no matching pattern

	// Output:
	// true
	// false
	// true
	// false
}

// ExampleNew_complexPatterns demonstrates advanced gitignore patterns.
func ExampleNew_complexPatterns() {
	gi := gitignore.New([]string{
		"docs/**/*.md",   // Ignore markdown files in docs and subdirectories
		"!docs/README.md", // Except the main README
		"/config.json",   // Only ignore config.json in root
		"*.tmp",          // Ignore all temporary files
	})

	fmt.Println(gi.Ignored("docs/api/index.md", false))    // true
	fmt.Println(gi.Ignored("docs/README.md", false))       // false
	fmt.Println(gi.Ignored("config.json", false))          // true
	fmt.Println(gi.Ignored("src/config.json", false))      // false
	fmt.Println(gi.Ignored("cache.tmp", false))            // true

	// Output:
	// true
	// false
	// true
	// false
	// true
}

// ExampleNew_directories demonstrates directory-specific patterns.
func ExampleNew_directories() {
	gi := gitignore.New([]string{
		"node_modules/", // Ignore node_modules directory
		"*.cache/",      // Ignore cache directories
		"temp",          // Ignore both temp files and directories
	})

	fmt.Println(gi.Ignored("node_modules", true))      // true - directory
	fmt.Println(gi.Ignored("node_modules", false))     // false - file
	fmt.Println(gi.Ignored("build.cache", true))       // true - directory ending with .cache
	fmt.Println(gi.Ignored("temp", true))              // true - directory named temp
	fmt.Println(gi.Ignored("temp", false))             // true - file named temp

	// Output:
	// true
	// false
	// true
	// true
	// true
}