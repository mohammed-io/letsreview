# AGENTS.md

## Role

You are working on `letsreview`: Go CLI + local web UI + MCP server for human-in-loop code review.

## Hard Rules

- Use Go stdlib first. Add deps only with clear need.
- Never run `git init` in app/runtime code. App inspects existing repos only.
- Never overwrite user changes. Check `git status --short` before edits.
- Keep docs/code accurate to current behavior.
- Prefer small pure funcs, table tests, observable behavior tests.
- Use `gofmt` on Go changes.
- Use canvas UI carefully: no DOM row renderer for diff lines unless perf reason changes.
- Do not store secrets. Do not send code to network APIs by default.
- Default localhost bind: `127.0.0.1:55492`.
- Keep multi-project flow: new CLI process should join existing server via `/api/projects`.

## Verify

```sh
GOCACHE=/private/tmp/letsreview-gocache go test ./...
GOCACHE=/private/tmp/letsreview-gocache go vet ./...
GOCACHE=/private/tmp/letsreview-gocache go build ./cmd/letsreview
GOCACHE=/private/tmp/letsreview-gocache go build ./cmd/mcp
```

## Architecture

- `cmd/letsreview`: user CLI, starts/join local web server, and `--mcp` stdio server mode.
- `cmd/mcp`: legacy/compat stdio MCP entrypoint.
- `internal/gitdiff`: Git diff command + unified diff parser.
- `internal/server`: HTTP API, in-memory projects/sessions/comments/explanations.
- `internal/server/web`: embedded browser UI.
- `agent-work`: task tracking, not product runtime state.

## Runtime Model

- `letsreview <repo>` accepts a path, but diff APIs require an existing Git repo.
- First CLI process owns HTTP server.
- Later CLI processes register extra repos with same server and heartbeat.
- UI has Live mode and Sessions mode.
- Live mode polls working tree diff.
- Sessions mode creates snapshot reviews: `working`, `staged`, or `refs`.
- Feedback/comments are in memory. Browser-only state uses `sessionStorage`.
- Submit review stores comments for MCP `get_review_result`.

## Docs Rules

- `README.md`: quick value + install/run.
- `HANDBOOK.md`: complete usage and behavior.
- Avoid stale claims about persistence, auth, cloud AI, or repo initialization.
