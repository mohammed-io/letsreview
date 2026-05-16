# preserve_cursor_after_comment

Status: active
Created: 2026-05-16 11:08:31

## Context

After saving feedback, selected cursor resets to start/top behavior. User wants cursor/selection to stay where it was. Also Space from diff view should open inline review and focus editor. Inline review should scroll enough to show full review box when near bottom.

## Value Proposition

Commenting should be non-disruptive: reviewer stays on same line, can type immediately when opening with Space, and review box should be fully visible when possible.

## Alternatives Considered

1. Keep clearing selection after save
   - Pros: previous behavior.
   - Cons: breaks keyboard review flow.
2. Preserve selection/scroll after save
   - Pros: matches user intent.
   - Cons: comment box may remain open unless we close it.
   - Decision: preserve cursor/scroll; close inline box after save.
3. Clamp inline review box visually
   - Pros: visible.
   - Cons: user rejected following behavior.
4. Scroll diff to reveal anchored inline review
   - Pros: keeps box line-anchored and visible.
   - Cons: adjusts scroll on open.
   - Decision: best fit.

## Todos

- [x] Preserve selected row and scroll after saving feedback.
- [x] Make Space from diff open inline review and focus textarea.
- [x] Scroll diff enough to reveal full inline review box when opening.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- Save feedback does not reset selected row to top/start.
- Space from diff zone focuses inline review textarea.
- Inline review box is fully visible when possible after opening near bottom.
- Box remains anchored to selected line.
- Verification passes.
