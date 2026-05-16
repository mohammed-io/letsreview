# forward_inline_review_wheel

Status: active
Created: 2026-05-16 10:34:22

## Context

When cursor is over inline review overlay, wheel events do not reach canvas, so diff scrolling stops. User wants scroll to behave at canvas/diff level even when pointer is over inline review.

## Value Proposition

Overlay should not trap scroll. Reviewers can keep scrolling naturally while comment box is open.

## Alternatives Considered

1. `pointer-events: none` on inline review
   - Pros: wheel reaches canvas.
   - Cons: breaks textarea/buttons.
2. Duplicate wheel handler for inline review
   - Pros: preserves interactions and forwards scroll semantics.
   - Cons: small helper refactor.
   - Decision: best fit.
3. Global window wheel handler
   - Pros: broad.
   - Cons: risks hijacking modals/page controls.

## Todos

- [x] Extract diff scroll logic into helper.
- [x] Reuse helper for canvas and inline review wheel events.
- [x] Preserve textarea internal scroll when it can scroll.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- Wheel over inline review scrolls diff canvas.
- Wheel over feedback textarea still scrolls textarea when it has overflow room.
- Existing canvas wheel behavior unchanged.
- Verification passes.
