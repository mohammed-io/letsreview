# fix_installed_cli_execution

## Status: in_progress

## Context
Installed `/Users/mohammed/.local/bin/letsreview` appears not to run for user. `--help` currently returns fatal error because `flag.ErrHelp` bubbles to `log.Fatal`. Installed binary also carries `com.apple.provenance` xattr.

## Value Proposition
Make installed CLI usable from PATH, with clean help behavior and verified execution.

## Alternatives considered (with trade-offs)
1. Only reinstall binary: may leave `--help` broken.
2. Only fix `--help`: may leave install execution blocked by xattr/install mode.
3. Fix help, rebuild, reinstall, clear xattr, verify installed binary: best complete fix.

## Todos
- [x] Fix `--help` to print usage and exit cleanly.
- [x] Add CLI tests for help and MCP flag behavior.
- [ ] Rebuild and reinstall binary to `~/.local/bin`.
- [ ] Clear restrictive install xattrs if present.
- [ ] Verify installed binary executes.

## Acceptance Criteria
- [ ] `letsreview --help` exits 0 and prints usage.
- [ ] `/Users/mohammed/.local/bin/letsreview --help` executes.
- [ ] `/Users/mohammed/.local/bin/letsreview --mcp` remains wired.
- [ ] Tests pass.

## Notes
Do not overwrite unrelated user changes.
