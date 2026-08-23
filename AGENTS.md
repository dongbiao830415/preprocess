# AGENTS.md

Single-package Go library (no external deps, Go 1.26) implementing the `%if/%ifdef/%ifndef/%elif/%else/%endif` preprocessor directives from SQLite's lemon generator. Public API: `Process`, `ProcessString`, `Define`, `Undefine`, `Op`.

## Commands

- Verify: `go test ./... && go vet ./...`
- No build steps, codegen, or lint config exist.

## Core semantics (easy to break, covered by tests)

- Directives must start at column 0 (no leading whitespace); indented `%ifdef` lines are plain text.
- Output preserves line numbers: directive lines and excluded text are replaced with spaces, newlines kept.
- `\r\n` is normalized to `\n`; a lone `\r` is kept.
- `Define("FOO=bar")` only defines `FOO` (everything from `=` on is dropped).
- Unmatched `%endif` is silently ignored (not an error); a `%else`/`%elif` with no open block implicitly opens one.
- `%elif` after `%else` is an error; unterminated `%if` is an error. All errors include a 1-based line number.
- Macro names may contain `_`; expressions support `!`, `&&`, `||`, parentheses (`!` > `&&` > `||`).

## Conventions

- Every file starts with the "Copyright (c) 2026 The preprocess Authors" BSD-style header.
- Keep zero dependencies; tests are table-driven in `preprocess_test.go` (same package, no external fixtures).
