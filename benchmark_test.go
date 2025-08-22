// Package gitignore_test provides benchmarks for the gitignore package.
package gitignore_test

import (
	"testing"

	gitignore "github.com/idelchi/go-gitignore"
)

// BenchmarkNew measures the performance of creating new gitignore instances
// with various pattern complexities.
func BenchmarkNew(b *testing.B) {
	patterns := []string{
		"*.log",
		"build/",
		"node_modules/",
		"!important.log",
		"docs/**/*.md",
		"/config.json",
		"*.tmp",
		"cache/",
		"src/**/*.go",
		"!src/main.go",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gitignore.New(patterns)
	}
}

// BenchmarkIgnored_SimplePatterns benchmarks pattern matching with simple patterns.
func BenchmarkIgnored_SimplePatterns(b *testing.B) {
	gi := gitignore.New([]string{
		"*.log",
		"*.tmp", 
		"build/",
		"cache/",
	})

	testPaths := []struct {
		path  string
		isDir bool
	}{
		{"app.log", false},
		{"data.tmp", false},
		{"build", true},
		{"cache", true},
		{"src/main.go", false},
		{"README.md", false},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, test := range testPaths {
			_ = gi.Ignored(test.path, test.isDir)
		}
	}
}

// BenchmarkIgnored_ComplexPatterns benchmarks pattern matching with complex patterns.
func BenchmarkIgnored_ComplexPatterns(b *testing.B) {
	gi := gitignore.New([]string{
		"docs/**/*.md",
		"!docs/README.md",
		"src/**/*.go",
		"!src/main.go", 
		"**/*.tmp",
		"**/cache/",
		"/config.json",
		"vendor/**",
		"!vendor/important/",
	})

	testPaths := []struct {
		path  string
		isDir bool
	}{
		{"docs/api/index.md", false},
		{"docs/README.md", false},
		{"src/utils/helper.go", false},
		{"src/main.go", false},
		{"temp/file.tmp", false},
		{"deep/nested/cache", true},
		{"config.json", false},
		{"src/config.json", false},
		{"vendor/lib/package.go", false},
		{"vendor/important/tool.go", false},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, test := range testPaths {
			_ = gi.Ignored(test.path, test.isDir)
		}
	}
}

// BenchmarkIgnored_DeepPaths benchmarks pattern matching with deeply nested paths.
func BenchmarkIgnored_DeepPaths(b *testing.B) {
	gi := gitignore.New([]string{
		"**/*.log",
		"**/node_modules/",
		"src/**/*.test.js",
		"!src/important/**",
	})

	testPaths := []struct {
		path  string
		isDir bool
	}{
		{"very/deep/nested/path/to/file.log", false},
		{"project/frontend/node_modules", true},
		{"src/components/Button/Button.test.js", false},
		{"src/important/module/critical.test.js", false},
		{"root/level/file.txt", false},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, test := range testPaths {
			_ = gi.Ignored(test.path, test.isDir)
		}
	}
}

// BenchmarkIgnored_ManyPatterns benchmarks performance with a large number of patterns.
func BenchmarkIgnored_ManyPatterns(b *testing.B) {
	// Generate many patterns to simulate a large gitignore file
	patterns := make([]string, 0, 100)
	
	// Add various pattern types
	for i := 0; i < 20; i++ {
		patterns = append(patterns, 
			"*.log", "*.tmp", "*.cache", "*.bak", "*.swp",
		)
	}
	for i := 0; i < 10; i++ {
		patterns = append(patterns,
			"build/", "dist/", "out/", "target/", "bin/",
			"node_modules/", ".git/", ".svn/", "coverage/", 
		)
	}
	for i := 0; i < 5; i++ {
		patterns = append(patterns,
			"**/*.test.js", "**/*.spec.js", "**/*.d.ts",
			"!important/**", "!src/main/**",
		)
	}

	gi := gitignore.New(patterns)

	testPaths := []struct {
		path  string
		isDir bool
	}{
		{"app.log", false},
		{"build", true},
		{"src/components/test.spec.js", false},
		{"important/file.log", false},
		{"regular/file.txt", false},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, test := range testPaths {
			_ = gi.Ignored(test.path, test.isDir)
		}
	}
}

// BenchmarkParallelIgnored benchmarks concurrent access to gitignore instances.
func BenchmarkParallelIgnored(b *testing.B) {
	gi := gitignore.New([]string{
		"*.log",
		"build/",
		"!important.log",
		"docs/**/*.md",
		"src/**/*.go",
	})

	testPaths := []struct {
		path  string
		isDir bool
	}{
		{"app.log", false},
		{"build", true},
		{"important.log", false},
		{"docs/api.md", false},
		{"src/main.go", false},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, test := range testPaths {
				_ = gi.Ignored(test.path, test.isDir)
			}
		}
	})
}