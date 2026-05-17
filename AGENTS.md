# AGENTS.md

## Role

You are working on `letsreview`: Go CLI + local web UI + MCP server for human-in-loop code review.

## Hard Rules

- Use Go stdlib first. Add deps only with clear need.
- Never run `git init` in app/runtime code. App inspects existing repos only.
- Never overwrite user changes. Check `git status --short` before edits.
- Prefer small pure funcs, table tests, observable behavior tests.
- Use `gofmt` on Go changes.
- Use canvas UI carefully: no DOM row renderer for diff lines unless perf reason changes.
- Do not store secrets. Do not send code to network APIs by default.
- Default localhost bind: `127.0.0.1:55492`.
- Keep multi-project flow: new CLI process should join existing server via `/api/projects`.

## Doc Sync (Important)

- When MCP tools change: update `HANDBOOK.md` MCP Tools section, `README.md` MCP tools table, and tool description in `internal/mcp/server.go` all together.
- When CLI interface changes: update `HANDBOOK.md` CLI Flags, `README.md` CLI section.
- When HTTP API changes: update `HANDBOOK.md` HTTP API Summary and relevant behavior sections.
- When web UI behavior changes: update `HANDBOOK.md` relevant sections.
- Never leave docs out of sync with code. Stale docs are a bug.
- `README.md`: quick value + install/run. No implementation detail.
- `HANDBOOK.md`: complete usage, behavior, and API reference.
- Avoid stale claims about persistence, auth, cloud AI, or repo initialization.

## Verify

```sh
GOCACHE=/private/tmp/letsreview-gocache go test ./...
GOCACHE=/private/tmp/letsreview-gocache go vet ./...
GOCACHE=/private/tmp/letsreview-gocache go build .
```

## Architecture

- Root package: user CLI, starts/join local web server, and `mcp` subcommand for stdio server mode.
- `internal/gitdiff`: Git diff command + unified diff parser.
- `internal/server`: HTTP API, in-memory projects/sessions/comments/explanations.
- `internal/server/web`: embedded browser UI.
- `agent-work`: task tracking, not product runtime state.

## Runtime Model

- `letsreview <repo>` requires an explicit repo path. No args shows help.
- First CLI process owns HTTP server.
- Later CLI processes register extra repos with same server and heartbeat.
- UI shows live working tree diff, polling every 2s.
- Feedback/comments are in memory with resolved status. Browser-only state uses `sessionStorage`.
- Submit review sends only unresolved comments to MCP agent.
- Agent resolves each comment after applying the fix via `resolve_feedback`.
