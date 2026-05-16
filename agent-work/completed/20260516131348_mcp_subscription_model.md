# MCP Subscription Model

## Context
Agent (MCP client) needs async event notifications from letsreview MCP server. Currently uses long-polling (`wait_for_review_event`) which blocks one request channel. Need MCP-spec `subscriptions/listen` support so agent can receive events while handling other tool calls.

## Value Proposition
- True async: agent waits for review events AND responds to other messages
- MCP spec compliant: `subscriptions/listen` + `notifications/*`
- Multi-subscription: multiple concurrent event streams
- Backward compatible: old long-poll tools still work

## Alternatives Considered
| Approach | Verdict |
|----------|---------|
| A. Full `subscriptions/listen` (MCP spec) | **CHOSEN** - spec compliant, true async |
| B. Enhanced long-poll | Rejected - still blocks channel |
| C. SSE stream (HTTP only) | Rejected - not stdio compatible |

## Todos
- [x] Add subscription state + types to MCPServer struct
- [x] Implement `subscriptions/listen` handler
- [x] Implement `notifications/cancelled` handler for subscription cleanup
- [x] Add notification dispatch: connect Store events → MCP subscriptions
- [x] Update `handleInitialize` to advertise subscription capabilities
- [x] Update `handleMessage` switch with new methods
- [x] Add subscription-aware notification writer (goroutine per sub)
- [x] Write tests for subscription lifecycle
- [x] Run build + vet + test

## Acceptance Criteria
- `subscriptions/listen` opens stream, sends ack
- Server pushes `notifications/review/*` on events
- Agent can call other tools while subscribed
- Cancellation works via `notifications/cancelled`
- Multiple subscriptions coexist
- Old long-poll tools still work
- `go build`, `go vet`, `go test` all pass

## Notes
- stdio: all messages share stdout, use writeMu for serialization
- subscriptionId = request ID from `subscriptions/listen`
- Buffered channels (cap 64) prevent blocking publishers
