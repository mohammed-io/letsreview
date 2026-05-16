# wait_for_explanation_mcp_tool

## Status: completed (20260516112032)

## Context
User wants explain flow to behave like review flow: agent should wait until human asks for explanation from UI. Generic `wait_for_review_event` exists, but explain deserves first-class MCP wait tool.

## Value Proposition
Agents can call a clear blocking tool for explanation requests instead of polling or parsing generic events.

## Alternatives considered (with trade-offs)
1. Keep generic `wait_for_review_event`: already supports explanation, but discoverability is weak.
2. Make `get_explanation_requests` block: breaks existing non-blocking semantics.
3. Add `wait_for_explanation_request`: explicit, backward-compatible, mirrors review wait behavior.

## Todos
- [x] Add `wait_for_explanation_request` MCP tool.
- [x] Reuse event stream and filter for `explanation_requested`.
- [x] Add tests for immediate, blocking, timeout behavior.
- [x] Update docs.
- [x] Run gofmt/tests/vet.

## Acceptance Criteria
- [ ] Agent can block waiting only for explanation requests.
- [ ] Tool returns selected file/range/request metadata.
- [ ] Existing review submit wait still works.
- [ ] MCP stdout remains JSON-RPC only.

## Notes
Do not overwrite unrelated user changes. Existing active work files left alone.
