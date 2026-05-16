# persist_review_state

## Status: completed (20260514190825)

## Context
User wants app viewport to avoid double scrolling and wants current file, current Canvas scroll, and feedback comments to persist per project in `sessionStorage`.

## Value Proposition
Refreshing or navigating back to a project should keep review context: same file, same Canvas position, same active review session/comments.

## Alternatives considered (with trade-offs)
- Persist only file/scroll: solves navigation but comments vanish until session is manually restored.
- Persist saved comments in sessionStorage: duplicates server state and risks stale deletes.
- Persist active review session id and restore from server: best fit, comments remain source-of-truth server-side.

## Todos
- [x] Make layout full viewport height with pane-only scrolling.
- [x] Persist active file path and Canvas scroll per project.
- [x] Persist active review session id and restore comments/counts per project.
- [x] Persist feedback draft per project/file/range.
- [x] Verify browser behavior and tests.

## Acceptance Criteria
- Body/page does not double-scroll; Canvas/file panes use available window height.
- Refresh restores active file per project.
- Refresh restores Canvas scroll per project/file.
- Refresh restores active review session comments/counts.
- Feedback draft is not lost on refresh.

## Notes
Use `sessionStorage`, not durable storage.
