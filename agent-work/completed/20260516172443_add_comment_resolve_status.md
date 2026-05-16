# add_comment_resolve_status

## Status: completed (20260516180924)

## Context
AI agents need to mark feedback comments as "resolved" after applying changes. The web UI must show which comments are resolved. Submit review only sends unresolved comments.

## Value Proposition
Human reviewer can see which of their comments the AI has acted on, without losing the comment history.

## Alternatives considered (with trade-offs)
1. Add `Resolved` bool + `ResolvedAt` time to `Feedback` struct — simple, no new types, resolved is just a flag on existing data. Winner.
2. Separate `ResolvedComment` type — more complex, duplication, no real benefit.
3. Delete + archive — loses context, user asked to keep comments visible.

## Todos
- [ ] Add `Resolved`/`ResolvedAt` fields to `Feedback` and `AgentComment` structs
- [ ] Add `resolveFeedbackFor` store method
- [ ] Add HTTP API endpoint `PATCH /api/.../feedback/{id}/resolve`
- [ ] Update `submitReviewFor` to only include non-resolved feedback
- [ ] Add `resolve_feedback` MCP tool
- [ ] Update web UI to show resolved badge/indicator on comments
- [ ] Add tests for resolve behavior
- [ ] Run full test suite + build

## Acceptance Criteria
- Feedback has `resolved` + `resolvedAt` fields
- MCP agent can mark individual comments as resolved
- Web UI shows resolved state (strikethrough/badge, not deleted)
- `submitReview` only includes unresolved comments
- `get_review_result` returns all comments but indicates which are resolved
- Existing tests still pass

## Notes
- Submit review filters to unresolved only
- AgentComment in SubmittedReview also carries resolved status
