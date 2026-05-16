# Fix Event Delivery for Agents

## Context
Two problems:
1. `explanation_submitted` not published as event (partially done, needs test)
2. OpenCode doesn't receive MCP notifications — agents use tools (request/response), not raw protocol subscriptions. Agent calls `subscribe_review_events`, gets response, then nothing. OpenCode doesn't process out-of-band JSON-RPC notifications on stdio.

## Root Cause
MCP clients like OpenCode work via tool call/response. They don't have an event loop for async notifications. The `subscriptions/listen` protocol method and `writeNotify` notifications are ignored by tool-only clients.

## Solution
Add `get_pending_events` tool — non-blocking fetch of events since last call. Agent calls it periodically between user interactions. No long-poll, no blocking. Returns immediately with new events or empty list.

Keep subscription infrastructure for MCP clients that DO support notifications (future-proof). But add the polling tool that returns immediately (non-blocking) so agents can check for events without blocking.

## Todos
- [ ] Add `get_pending_events` tool — non-blocking, returns events since `afterSeq`
- [ ] Add `explanation_submitted` event test
- [ ] Update `request_code_review` description to mention `get_pending_events`
- [ ] Update tool descriptions to guide agent toward `get_pending_events`
- [ ] Update tests
- [ ] Run build + vet + test
