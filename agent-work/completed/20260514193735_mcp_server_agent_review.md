# mcp_server_agent_review

## Status: completed (20260514204030)

## Context
Build MCP server for let'sreview so AI agents can request human code review, open browser, wait for feedback, and receive structured review payload back via MCP protocol.

## Value Proposition
Agent-driven code review loop: agent requests review → human reviews in browser → agent receives comments → agent applies changes.

## Alternatives Considered
- In-process MCP + HTTP (chosen): single binary, shared memory, simplest.
- Separate MCP proxy + standalone letsreview: needs IPC, harder.
- WebSocket push: overkill for human-speed reviews.

## Todos
- [x] Create `internal/mcp/server.go` — MCP JSON-RPC protocol layer (stdio).
- [x] Create `cmd/mcp/main.go` — entry point, starts HTTP + MCP.
- [x] Add `SubmittedReview` to Store + `POST /api/sessions/{id}/submit-review` endpoint.
- [x] UI: rename "Export agent payload" → "Submit review", wire to new endpoint.
- [x] UI: show submitted state (green badge, disable editing).
- [x] MCP tools: `request_code_review`, `check_review_status`, `get_review_result`, `cancel_review`.
- [x] Tests for MCP protocol + submit endpoint.
- [ ] Manual E2E test: agent → MCP → browser → submit → agent gets result.

## Acceptance Criteria
- `go run ./cmd/mcp` starts MCP server over stdio.
- `request_code_review` returns working URL with project/session params.
- Browser "Submit review" stores review; UI locks editing.
- `check_review_status` returns pending/submitted.
- `get_review_result` returns comments matching agent-payload shape.

## Notes
MCP spec v2024-11-05. JSON-RPC 2.0 over stdin/stdout. No external deps.
