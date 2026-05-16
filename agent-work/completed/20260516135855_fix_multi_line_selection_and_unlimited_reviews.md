# fix_multi_line_selection_and_unlimited_reviews

## Status: completed (20260516140722)

## Context
1. Shift+J/K in review UI doesn't extend selection — `moveSelectedLine` always resets `start=end=target`
2. Review submit is one-shot: `submittedReviews` map + `reviewSubmitted` flag locks UI after first submit. User wants unlimited reviews per session.

## Value Proposition
- Shift+J/K = vim-style visual line extension (like V in vim)
- Unlimited reviews = reviewer can submit incremental feedback, agent gets each as separate `review_submitted` event

## Alternatives considered (with trade-offs)
1. **Separate extendSelectedLine func** vs **mode flag in moveSelectedLine** → mode flag simpler, less code duplication
2. **Keep submittedReviews as list** vs **remove entirely, use events** → events already track all reviews. Remove map, `check_review_status`, `review-status` endpoint. `get_review_result` returns latest from events.

## Todos
- [x] Fix Shift+J/K to extend selection in app.js
- [x] Remove `submittedReviews` map from Store, `GetSubmittedReview`, `SetSubmittedReview`
- [x] Remove `check_review_status` MCP tool
- [x] Remove `review-status` HTTP endpoints (both `/api/` and `/api/projects/...`)
- [x] Update `get_review_result` to return latest review from events
- [x] Remove `reviewSubmitted` state from app.js, remove "Review submitted" banner
- [x] Update `submitReview()` in app.js — no guard, allow re-submit
- [x] Update tests (server_test.go, mcp/server_test.go)
- [x] Verify all builds + tests pass

## Acceptance Criteria
- Shift+J extends selection downward, Shift+K extends upward
- User can submit multiple reviews per session, each creates `review_submitted` event
- No `reviewSubmitted` flag anywhere in UI
- No `check_review_status` tool, no `/review-status` endpoint
- `get_review_result` returns latest submitted review from events
- All tests pass

## Notes
