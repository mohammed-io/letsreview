# Replace Polling with Subscriptions

## Context
Agent calls `wait_for_review_event`/`wait_for_explanation_request` in blocking loop. Can't respond to user while waiting. Need true async: agent subscribes, gets notified, stays free.

## Value
- Agent responds to user while monitoring review
- No busy-wait polling loops
- Proper MCP subscription protocol

## Todos
- [x] Replace handleToolsList: remove wait_for_review_event + wait_for_explanation_request, add subscribe/unsubscribe tools
- [x] Update request_code_review description to guide toward subscriptions
- [x] Implement toolSubscribeReviewEvents + toolUnsubscribeReviewEvents + update handleToolsCall
- [x] Update tests: fix tool list test, add subscription tool tests
- [x] Run build + vet + test

## Acceptance
- `tools/list` returns subscribe/unsubscribe tools, NOT wait_for_*
- Agent instructions in tool descriptions guide toward subscription flow
- `subscribe_review_events` returns immediately with subscription ID
- Events delivered async via `notifications/review/*`
- `unsubscribe_review_events` cleans up
- All tests pass
