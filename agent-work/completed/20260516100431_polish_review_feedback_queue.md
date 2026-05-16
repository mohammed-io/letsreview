# polish_review_feedback_queue

Status: active
Created: 2026-05-16 10:04:31

## Context

Review feedback queue has more UX issues: existing feedback is not shown after refresh until adding a new comment, card header is too noisy with dates and large delete button, list items squeeze instead of scrolling, Active selection is unnecessary, and C shortcut for comment should be removed.

## Value Proposition

Feedback queue should be immediately trustworthy after reload, compact, scrollable, and not hijack normal keyboard habits.

## Alternatives Considered

1. Select full session in Live mode to populate comments
   - Pros: comments appear.
   - Cons: reintroduces live diff file swap/flicker.
2. Attach active session metadata in Live mode without selecting session files
   - Pros: comments populate without changing live diff state.
   - Cons: slightly more state nuance.
   - Decision: best fit.
3. CSS-only list tweaks
   - Pros: quick.
   - Cons: must also simplify markup and remove shortcut.

## Todos

- [x] Populate review feedback queue on refresh without swapping live diff files.
- [x] Remove Active selection UI and C comment shortcut.
- [x] Compact feedback card header with line + red delete icon only.
- [x] Make feedback queue scroll with fixed-height non-squeezed items.
- [x] Run required verification.
- [x] Complete work.

## Acceptance Criteria

- Existing comments show immediately after page refresh.
- Live mode diff does not flicker/swap due to session restore.
- Active selection panel is gone.
- No C/Cmd+C shortcut opens comments.
- Feedback card header has line and compact red delete icon, no date.
- Feedback cards retain fixed height and queue scrolls.
- Required verification passes.
