# long_running_mcp_review_events

## Status: completed (20260515130726)

## Context
Current MCP tools start reviews but require polling. User wants a long-running wait flow: agent starts review, then waits until browser UI submits review comments or asks for explanation.

## Value Proposition
Agents can block on one MCP call and react when human reviewer acts, while preserving JSON-RPC/MCP stdout correctness.

## Alternatives considered (with trade-offs)
1. Print raw web UI events to stdout: simple, but breaks MCP JSON-RPC stream.
2. Keep polling tools: already works, but user explicitly wants wait behavior.
3. Make `request_code_review` block until result: simple for submit, but prevents same stdio loop from serving `submit_explanation`.
4. Add `wait_for_review_event` long-poll tool and make MCP request handling concurrent: best fit. Review start returns URL/session, agent calls wait, web UI events wake it, explain/submit both work.

## Todos
- [x] Add event subscription/wait support to server store.
- [x] Emit events for submitted reviews and explanation requests.
- [x] Add MCP `wait_for_review_event` tool.
- [x] Make MCP JSON-RPC request handling concurrent with serialized writes.
- [x] Add observable tests for wait behavior.
- [x] Run gofmt/tests/vet.

## Acceptance Criteria
- [x] Agent can start review and receive URL/session immediately.
- [x] Agent can call wait tool and block until submit/explain event or timeout.
- [x] Browser submit wakes wait tool with all comments.
- [x] Browser explain wakes wait tool with selected line request.
- [x] MCP stdout remains valid JSON-RPC only.

## Install

- [x] Built `letsreview`.
- [x] Copied binary to `/Users/mohammed/.local/bin/letsreview`.

## Notes
Existing active work files remain untouched. Do not overwrite user changes.
