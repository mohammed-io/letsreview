# mock_openbrowser_during_tests

## Status: completed (20260516112317)

## Context
`openBrowser` is a concrete function in both `cmd/letsreview/main.go` and `internal/mcp/server.go`. MCP tests exercise `request_code_review` which calls `openBrowser`, spawning real browser windows during `go test`.

## Value Proposition
Tests run without side effects (no browser windows). Standard Go var-func pattern for mockability.

## Alternatives considered (with trade-offs)
1. **var openBrowser = func(...)**: idiomatic Go, tests override with no-op. Same pattern as `pidFilePath`. ✅
2. **Interface + struct**: overkill for single function, more boilerplate.
3. **Build tag**: splits test/prod code, harder to maintain.

## Todos
- [x] Convert `openBrowser` to `var` in `cmd/letsreview/main.go`
- [x] Convert `openBrowser` to `var` in `internal/mcp/server.go`
- [x] Add no-op override in MCP test file
- [x] Add no-op override in cmd test file
- [x] Run full verify suite

## Acceptance Criteria
- [ ] `go test ./...` opens zero browser windows
- [ ] All tests pass
- [ ] Vet/build clean

## Notes
Same pattern as existing `pidFilePath` var override.
