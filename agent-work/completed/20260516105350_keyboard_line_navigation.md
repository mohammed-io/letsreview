# keyboard_line_navigation

Status: active
Created: 2026-05-16 10:53:50

## Context

Reviewer wants keyboard navigation over source diff rows: ArrowUp/ArrowDown and Vim `j`/`k`, including numeric prefixes like `10j`. Selected line should have yellowish highlight.

## Value Proposition

Keyboard line navigation makes review flow faster and reduces mouse usage while keeping selected line visually obvious.

## Alternatives Considered

1. Keep `j/k` for hunk navigation
   - Pros: existing behavior.
   - Cons: user explicitly wants line movement.
2. Add arrow keys only
   - Pros: avoids shortcut conflict.
   - Cons: misses Vim flow and count prefixes.
3. Make `j/k` line movement with numeric prefixes, keep Shift+J/K for comments
   - Pros: matches request and preserves comment navigation.
   - Cons: hunk nav loses `j/k` shortcut.
   - Decision: best fit.

## Todos

- [x] Add keyboard cursor/count state and line movement helper.
- [x] Wire ArrowUp/ArrowDown and Vim `j/k` with numeric prefixes.
- [x] Change selected row highlight to yellow-ish.
- [x] Update shortcut help.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- ArrowDown/ArrowUp move selected row by one.
- `j`/`k` move selected row by one.
- `10j` moves selected row down ten rows; numeric prefix resets after motion.
- Selected row is highlighted yellow-ish.
- Typing in inputs/textareas is not hijacked.
- Verification passes.
