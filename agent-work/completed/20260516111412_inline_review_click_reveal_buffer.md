# inline_review_click_reveal_buffer

Status: active
Created: 2026-05-16 11:14:12

## Context

Inline review reveal buffer works for Space keyboard open but not mouse click line open. Click and Space should share same open behavior.

## Value Proposition

Consistent comment box reveal regardless of mouse or keyboard interaction.

## Alternatives Considered

1. Duplicate reveal logic in mouseup handler
   - Pros: quick.
   - Cons: drift risk.
2. Route mouseup through `openInlineReviewForSelection()` helper
   - Pros: single behavior.
   - Cons: must preserve no autofocus for mouse.
   - Decision: best fit.

## Todos

- [x] Ensure mouse click uses same reveal helper as Space.
- [x] Preserve mouse click no-autofocus behavior.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- Clicking line near bottom reveals inline review with same buffer.
- Pressing Space behavior unchanged.
- Verification passes.
