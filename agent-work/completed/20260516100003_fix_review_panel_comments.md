# fix_review_panel_comments

Status: active
Created: 2026-05-16 10:00:03

## Context

Review cockpit comment list has three UX bugs: Cmd/Ctrl+Enter can save same comment twice, feedback cards stretch to fill available panel space, and clicking a feedback card jumps to code but does not open the inline comment thread for that line.

## Value Proposition

Review feedback should be trustworthy, compact, and navigable. Keyboard save must be single-action; feedback list should behave like a scrollable queue; clicking feedback should restore full line context and comments.

## Alternatives Considered

1. Dedupe repeated comments server-side
   - Pros: hides duplicate symptom.
   - Cons: wrong root cause; user explicitly rejected dedupe.
2. Keep both keyboard handlers but add lock flag
   - Pros: small.
   - Cons: still duplicated event ownership.
3. Single global Cmd/Ctrl+Enter owner
   - Pros: root cause fix; one shortcut path.
   - Cons: must preserve textarea save behavior.
   - Decision: best fit.
4. CSS only list height fix
   - Pros: simple.
   - Cons: must also open inline thread on click.

## Todos

- [x] Fix duplicate Cmd/Ctrl+Enter by removing duplicate event path.
- [x] Make review feedback cards fixed-height scrollable queue items.
- [x] Open inline comment thread when clicking feedback card.
- [x] Run required verification.
- [x] Complete work.

## Acceptance Criteria

- Cmd/Ctrl+Enter from comment textarea calls save once.
- Cmd/Ctrl+Enter outside textarea submits review once.
- Review panel cards do not stretch to fill remaining height.
- Review panel scrolls with multiple cards.
- Clicking a review panel card selects code line and opens inline comments for that range.
- Required verification passes.
