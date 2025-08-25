----------

Consider gitignore.go and the full test suite.

What is the risk that gitignore.go is biased against "just" solving the test suite, and not actually fulfilling gitignore parity? e.g., that the implementation is overfitted to the tests.

If you believe it is biased, return a concentrated set of YAML defined tests (10-15), which aim to prove that above is the case or not.


-----------

Assess the current implementation. Read `OVERFITTING_GUIDE.md` for a current situation analysis.

Read `GENERAL_OVERFITTING_GUIDE.md` and strictly adhere to it.

Keep a holistic view in order to reach 1-1 parity. No "overfitting" to the tests.

Keeping running `go test -run TestGitIgnore .` after each change.

You are only allowed to edit `gitignore.go`.

If you require to create debug files to run, create them in cmd/debug and not in the root. Your debug files must always end with the suffix _debug.go.

Strive to always run all tests so that you don't fixate/isolate on a single test case.

You are done only when 100% of the tests are successful and the concerns in `GENERAL_OVERFITTING_GUIDE.md` are addressed.

Be mindful of the interaction with bmatcuk/doublestar, the quirks of `gitignore` that might need tailored solutions, and keep an eye on its behavior.

If you truly believe the the suggestions in `OVERFITTING_GUIDE.md` will NOT work - stop - make a situation analysis and return the analysis to me.

----------

Assess whether there's potential for simpifications now that we've made multiple passes on the package.

Strive for modern Go practices as much as possible.

If you do find potential, iteratively try to simplify `gitignore.go` while keeping all tests passing. Run `go test -run TestGitIgnore .` after each change.

You are only allowed to edit `gitignore.go`.

If you require to create debug files to run, create them in cmd/debug and not in the root. Your debug files must always end with the suffix _debug.go.

Strive to always run all tests so that you don't fixate/isolate on a single test case. Keep always a holistic view on the task.

You are done only when 100% of the tests are successful and the concerns in `GENERAL_OVERFITTING_GUIDE.md` are still fulfilled.

Be mindful of the interaction with bmatcuk/doublestar and keep an eye on its behavior.





----------

Modernize this package.

Iteratively modernize `gitignore.go` while keeping all tests passing. Run `go test -run TestGitIgnore .` after each change.

- Naming of methods/functions
- Comments should be descriptive and document behavior and quirks of git

You are only allowed to edit `gitignore.go`.

If you require to create debug files to run, create them in cmd/debug and not in the root. Your debug files must always end with the suffix _debug.go.

Strive to always run all tests so that you don't fixate/isolate on a single test case. Keep always a holistic view on the task.

You are done only when 100% of the tests are successful. Keep your comments informative and concise (no making grandiose claims that cannot be backed.)

----------


----------

Go through all tests/*.yml files and make sure the format is consistent

1. Simple strings are unquoted, for fields name, description, cases .path, cases.description
2. Strings that have special characters or spaces are quoted such that the YAML parsing is successful. This is commonly happening in description and cases.description and sometimes cases.path
3. Make sure "name:" is wellformed - that is, it should be a string text, not with underscores, but with free text style. So "escaped_special_char" -> "escaped special characters", and so on. So a free text descriptive style. Search for underscores in "name:" fields and keep iterating until there are none left.

You MUST not change the test cases in anyway - so be careful when doing this so that you don't accidentally strip away intended whitespaces etc.

At the end, run `go test -run TestGitCheckIgnore .` to make sure you didn't break any of the tests.

----------


----------

Run task go:lint and iteratively address all linting warnings EXCEPT gocognic, cyclop, nestif

Run `go test -run TestGitIgnore .` after each change to make sure you didn't break any of the tests.

----------
