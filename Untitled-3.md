----------

Consider gitignore.go and the full test suite.

What is the risk that gitignore.go is biased against "just" solving the test suite, and not actually fulfilling gitignore parity? e.g., that the implementation is overfitted to the tests.

If you believe it is biased, return a concentrated set of YAML defined tests (10-15), which aim to prove that above is the case or not.


----------

Go through all tests/*.yml files and make sure the format is consistent

1. Simple strings are unquoted, for fields name, description, cases.path, cases.description
2. Strings that have special characters or spaces are quoted such that the YAML parsing is successful. This is commonly happening in description and cases.description and sometimes cases.path
3. Make sure "name:" is wellformed - that is, it should be a string text, not with underscores, but with free text style. So "escaped_special_char" -> "escaped special characters", and so on. So a free text descriptive style. Search for underscores in "name:" fields and keep iterating until there are none left.

----------


Run `go test -run TestGitIgnore .` and amend gitignore.go until all tests pass.

Keep a holistic view in order to reach 1-1 parity. No "overfitting" to the tests.

Keeping running `go test -run TestGitIgnore .` after each change.

You are only allowed to edit `gitignore.go`.

If you require to create debug files to run, create them in cmd/debug and not in the root.

Strive to always run all tests so that you don't fixate/isolate on a single test case.

You are done only when 100% of the tests are successful.

Be mindful of the interaction with bmatcuk/doublestar and keep an eye on its behavior.

----------

I am questioning whether this package needs to be as complex as it is.

Come up with a simplification plan. Strive for modern Go practices as much as possible.

Iteratively try to simplify `gitignore.go` while keeping all tests passing. Run `go test -run TestGitIgnore .` after each change.

You are only allowed to edit `gitignore.go`.

If you require to create debug files to run, create them in cmd/debug and not in the root.

Strive to always run all tests so that you don't fixate/isolate on a single test case. Keep always a holistic view on the task.

You are done only when 100% of the tests are successful.

Be mindful of the interaction with bmatcuk/doublestar and keep an eye on its behavior.


----------
