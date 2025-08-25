package gitignore_test

import (
	"fmt"
	"strings"
	"testing"

	gitignore "github.com/idelchi/go-gitignore"
)

// result is a package-level variable to ensure the compiler doesn't optimize away benchmark calls.
var result bool //nolint:gochecknoglobals		// See above

func BenchmarkNew(b *testing.B) {
	b.Run("1000_Simple_Patterns", func(b *testing.B) {
		patterns := generateSimplePatterns(1000)

		b.ResetTimer()

		for range b.N {
			_ = gitignore.New(patterns)
		}
	})

	b.Run("1000_Complex_Patterns", func(b *testing.B) {
		patterns := generateComplexPatterns(1000)

		b.ResetTimer()

		for range b.N {
			_ = gitignore.New(patterns)
		}
	})
}

func BenchmarkIgnored(b *testing.B) {
	// Setup with a large, realistic .gitignore file
	realWorldPatterns := getRealWorldGitignore()
	giRealWorld := gitignore.New(realWorldPatterns)

	// Scenario 1: Test scaling with path depth
	b.Run("Path_Depth", func(b *testing.B) {
		deepPath := "a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p/q/r/s/t/u/v/w/x/y/z/file.go"

		b.Run("Shallow", func(b *testing.B) {
			for range b.N {
				result = giRealWorld.Ignored("src/components/button.tsx", false)
			}
		})
		b.Run("Deep", func(b *testing.B) {
			for range b.N {
				result = giRealWorld.Ignored(deepPath, false)
			}
		})
	})

	// Scenario 2: Test scaling with number of rules
	b.Run("Rule_Count", func(b *testing.B) {
		path := "src/app/core/services/api.service.ts"

		b.Run("100_Rules", func(b *testing.B) {
			gi := gitignore.New(generateSimplePatterns(100))

			b.ResetTimer()

			for range b.N {
				result = gi.Ignored(path, false)
			}
		})
		b.Run("5000_Rules", func(b *testing.B) {
			gi := gitignore.New(generateSimplePatterns(5000))

			b.ResetTimer()

			for range b.N {
				result = gi.Ignored(path, false)
			}
		})
	})

	// Scenario 3: Real-world simulation
	b.Run("RealWorld_Simulation", func(b *testing.B) {
		// A mix of paths to check against the real-world gitignore
		paths := []string{
			"node_modules/react/index.js",      // Should be ignored
			"src/main.go",                      // Should not be ignored
			"build/output/final.exe",           // Should be ignored
			"docs/images/screenshot.png",       // Should not be ignored
			".env.local",                       // Should be ignored
			"a/b/c/d/e/f/g/vendor/lib/file.go", // Should be ignored
		}

		b.ResetTimer()

		for i := range b.N {
			// Cycle through the paths to prevent CPU caching from skewing results
			result = giRealWorld.Ignored(paths[i%len(paths)], false)
		}
	})
}

func generateSimplePatterns(n int) []string {
	patterns := make([]string, n)
	for i := range n {
		patterns[i] = fmt.Sprintf("file-%d.log", i)
	}

	return patterns
}

func generateComplexPatterns(n int) []string {
	patterns := make([]string, n)
	for i := range n {
		patterns[i] = fmt.Sprintf("src/**/generated-%d-*/__tests__/**/*.spec.ts", i)
	}

	return patterns
}

func getRealWorldGitignore() []string {
	// A snippet from a large, typical Node.js project's .gitignore
	content := `
# See https://help.github.com/articles/ignoring-files/ for more about ignoring files.

# dependencies
/node_modules
/.pnp
.pnp.js

# testing
/coverage

# production
/build
/dist

# misc
.DS_Store
.env.local
.env.development.local
.env.test.local
.env.production.local

npm-debug.log*
yarn-debug.log*
yarn-error.log*

# Caching
.cache/
.eslintcache

# Editor directories and files
.idea
.vscode/*
!.vscode/settings.json
!.vscode/tasks.json
!.vscode/launch.json
!.vscode/extensions.json
*.sublime-workspace

# Go
vendor/
*.exe
*.out

# Python
__pycache__/
*.py[cod]
*$py.class
`

	return strings.Split(content, "\n")
}
