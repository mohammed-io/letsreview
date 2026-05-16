# live_inline_feedback

## Status: completed (20260514155331)

## Context
Inline feedback currently appears only in Sessions mode. User expects clicking diff lines in default Live mode to open GitHub-like review feedback.

## Value Proposition
Make line commenting discoverable: click any diff line in Live or Sessions and get an inline comment form at that row range.

## Alternatives considered (with trade-offs)
- Tell user to switch to Sessions/Create review: no code, but bad UX and hidden workflow.
- Show disabled form in Live: discoverable but cannot save, confusing.
- Auto-create working-tree review session on first Live selection: best UX, slight async cost, keeps existing feedback API.
- Add live feedback API separate from sessions: more direct but duplicates review model and export flow.

## Todos
- [x] Inspect current behavior and repo state.
- [x] Auto-create review session when Live line selection needs feedback.
- [x] Keep inline review positioned under selected rows and focus textarea.
- [x] Add observable coverage for static UI contract.
- [x] Verify tests and browser behavior.

## Acceptance Criteria
- Clicking/dragging Canvas rows in Live mode opens inline feedback.
- Saving feedback uses a review session and remains exportable.
- Sessions mode behavior still works.
- Tests pass.

## Notes
Refactoring UI score remains 8/10: workflow fixed now; deeper color polish deferred.
