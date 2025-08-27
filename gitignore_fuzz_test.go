//go:build !windows

package gitignore_test

import (
	"strings"
	"testing"
	"unicode"

	gitignore "github.com/idelchi/go-gitignore"
)

// FuzzGitIgnoreParity fuzzes random .gitignore contents + paths,
// uses `git check-ignore` as the oracle, and asserts our matcher agrees.
//
// Git's exit code (0=ignored, 1=not ignored) becomes the expected value for the package under test.
func FuzzGitIgnoreParity(f *testing.F) {
	// A few useful seeds to hit tricky corners early.
	seed := func(gi, p string, dir bool) { f.Add(gi, p, dir) }
	// Parent exclusion / sandwich / contents-only:
	seed("**/node_modules/**\n!**/node_modules/**/README.md\n", "a/b/node_modules/README.md", false)
	seed("data/**\n!data/**/\n!data/**/*.txt\n", "data/data2/file2.txt", false)
	seed("build/\n!important.txt\n", "build/keep.txt", false) // cannot re-include under excluded dir
	// Rooted vs unrooted; bare *; escaped comment/negation; trailing space kept:
	seed("/*\n!/keep\n\\#literal\n\\!bang\nname\\ \\ \n", "keep", false)
	seed("a/**/b/\n!a/**/b/c.txt\n", "a/x/y/b/c.txt", false)
	seed("*.log\n", "app.log", false)
	seed("git/\n", "git/foo", true)

	f.Fuzz(func(t *testing.T, rawGitignore, rawPath string, isDir bool) {
		gi := sanitizeGitignore(rawGitignore)

		p := sanitizePath(rawPath)
		if gi == "" || p == "" {
			t.SkipNow()
		}

		// 1) Ask Git for ground truth via the existing helper.
		spec := GitIgnore{
			Name:      "fuzz",
			Gitignore: gi,
		}
		c := Case{
			Path:        p,
			Dir:         isDir,
			Description: "fuzz",
		}

		res := runGitCheckIgnoreTest(t, spec, c) // exit 0 => ignored
		if res.ExitCode != 0 && res.ExitCode != 1 {
			// Git refused to evaluate this path (unlikely with our sanitization);
			// don't learn from non-deterministic or errorful cases.
			t.Skipf("skip weird git exit=%d (stderr not captured here)", res.ExitCode)

			return
		}

		want := res.Actual

		// 2) Run our implementation under test on the same inputs.
		g := gitignore.New(strings.Split(gi, "\n")...)
		got := g.Ignored(p, isDir)

		if got != want {
			t.Fatalf(
				"Ignored() check failed:\n  path: %v\n  dir: %v\n  patterns: %v\n  expected: %v\n  got: %v\n",
				p,
				isDir,
				strings.Split(spec.Gitignore, "\n"),
				boolToIgnored(want),
				boolToIgnored(got),
			)
		}
	})
}

// sanitizeGitignore turns an arbitrary fuzzer string into a small, interesting .gitignore.
// It maps bytes to a vocabulary of edge-casey lines and also sprinkles in literal lines
// from the input. This keeps size bounded and avoids OS path hazards.
func sanitizeGitignore(s string) string {
	if s == "" {
		return "*.log\nbuild/\n!important.log"
	}

	const maxLines = 32

	vocab := []string{
		"",          // blank
		"# comment", // will be ignored by parser (expected)
		"*.log",
		"!*.log",
		"build/",
		"/build/",
		"/*",
		"*",
		"**/",
		"**/*.tmp",
		"*/cache/",
		"**/node_modules/**",
		"!**/node_modules/**/",
		"a/**/b/",
		"[abc]/*.go",
		"[!abc]/*.go",
		"\\#literal",     // escaped comment
		"\\!literalBang", // escaped negation
		"name\\ \\ ",     // trailing space kept
		"data/**",
		"!data/**/",
		"!data/**/*.txt",
	}

	var lines []string

	b := []byte(s)
	if len(b) == 0 { //nolint:nestif	// Function is complex by design.
		lines = append(lines, vocab[0])
	} else {
		for i := 0; i < len(b) && len(lines) < maxLines; i++ {
			lines = append(lines, vocab[int(b[i])%len(vocab)])
			// Occasionally emit a literal-ish line built from raw input (trimmed).
			if b[i]&0x7 == 0 && len(lines) < maxLines {
				lit := compactToPrintable(s)
				if lit != "" {
					// Keep it short to avoid runaway patterns.
					if len(lit) > 40 {
						lit = lit[:40]
					}

					lines = append(lines, lit)
				}
			}
		}
	}

	// Limit total size to keep git/doublestar happy.
	joined := strings.Join(lines, "\n")
	if len(joined) > 4096 {
		joined = joined[:4096]
	}
	// Normalize Windows newlines just in case.
	return strings.ReplaceAll(joined, "\r\n", "\n")
}

// isSafeRune reports whether r is allowed in sanitized paths and printable patterns.
func isSafeRune(r rune) bool {
	if r < 0x20 || r == 0x7f {
		return false
	}

	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}

	switch r {
	case '/', '-', '_', '.', ' ', '[', ']', '{', '}', '!', '#', '*', '?', '\\':
		return true
	}

	return false
}

// filterToSafeRunes returns a slice of runes from s that pass isSafeRune.
func filterToSafeRunes(s string) []rune {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if isSafeRune(r) {
			out = append(out, r)
		}
	}

	return out
}

// sanitizePath makes a safe relative path (no "..", no absolute, bounded length)
// using a restricted character set that still exercises interesting cases.
func sanitizePath(s string) string {
	if s == "" {
		return "a/b/file.txt"
	}
	// Keep only a safe subset of runes; drop control chars.
	out := filterToSafeRunes(s)

	ss := string(out)

	ss = strings.ReplaceAll(ss, "\r\n", "\n")
	ss = strings.ReplaceAll(ss, "\n", "/")
	ss = strings.TrimSpace(ss)
	ss = strings.Trim(ss, "/")

	// Split and scrub dangerous segments.
	if ss == "" {
		ss = "a"
	}

	parts := strings.Split(ss, "/")
	for i := range parts {
		if parts[i] == "" || parts[i] == "." || parts[i] == ".." {
			parts[i] = "x"
		}
		// Avoid .git special directory as a component.
		if parts[i] == ".git" {
			parts[i] = "git"
		}
		// Avoid ridiculously long components.
		if len(parts[i]) > 64 {
			parts[i] = parts[i][:64]
		}
	}

	ss = strings.Join(parts, "/")
	if len(ss) > 180 {
		ss = ss[:180]
	}
	// Avoid empty result.
	if ss == "" {
		ss = "x"
	}

	return ss
}

// compactToPrintable builds a small literal pattern from s, removing control chars.
func compactToPrintable(s string) string {
	out := filterToSafeRunes(s)

	return strings.TrimSpace(string(out))
}
