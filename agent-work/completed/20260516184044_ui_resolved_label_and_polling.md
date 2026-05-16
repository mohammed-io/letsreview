# UI: Resolved Label Position + Backend Polling

## Context
When MCP agent resolves feedback via `resolve_feedback`, the UI doesn't reflect changes until manual interaction. Also, the "Resolved" badge should be visually next to the trash icon in comment items.

## Value
Agent-driven feedback resolution becomes visible in real-time. Cleaner comment item layout.

## Alternatives
1. **Combined poll (chosen)**: Poll `refreshSessions` alongside `fetchLiveDiff` in live mode. Simple, leverages existing functions.
2. **WebSocket push**: Real-time but overkill for local tool. Adds complexity.
3. **Lightweight feedback-only poll**: More efficient but requires new endpoint.

## Todos
- [x] Remove sessions mode, comparison panel, mode toggle (HTML)
- [x] Clean up CSS: remove mode styles, add comment-meta-actions
- [x] Clean up JS: remove mode state/els/listeners/functions + live-only init
- [x] Move Resolved badge next to trash icon in `commentNodes()`
- [x] Add session polling in live init (polls both fetchLiveDiff + refreshSessions every 2s)
- [x] `renderInlineReview()` and `renderReviewPanel()` update on poll (via renderAll)
- [x] Existing tests cover observable behavior — all pass
- [x] go vet/test/build pass

## Acceptance Criteria
- Resolved badge appears right next to trash icon
- In live mode, resolved comments update automatically within ~2s
- Queue and inline-review pane reflect backend changes on poll
