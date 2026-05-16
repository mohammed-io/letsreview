# fix_canvas_comment_marker_and_inline_anchor

Status: active
Created: 2026-05-16 10:31:06

## Context

Canvas comment count marker is too small/subtle and appears on right side. Inline review panel still follows/clamps while scrolling; it should stay anchored to selected code line and disappear when line scrolls away.

## Value Proposition

Comment markers become visible and code-anchored inline review behavior feels spatially correct.

## Alternatives Considered

1. Increase marker only
   - Pros: small.
   - Cons: does not fix wrong side.
2. Move marker to gutter left side and increase size
   - Pros: matches line-associated affordance.
   - Cons: uses gutter space.
   - Decision: best fit.
3. Keep inline review clamped in viewport
   - Pros: always reachable.
   - Cons: violates code anchoring; user rejected.
4. Anchor inline review strictly to row and hide when offscreen
   - Pros: expected behavior.
   - Cons: user may need scroll back to edit.
   - Decision: best fit.

## Todos

- [x] Move canvas comment marker to left/gutter and make bigger.
- [x] Stop inline review from clamping/following during scroll.
- [x] Hide inline review when anchored line is offscreen.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- Comment count marker is bigger and on left side.
- Inline review top is based only on selected line position.
- Inline review does not clamp to viewport bottom/top while scrolling.
- Inline review hides when selected line range leaves viewport.
- Verification passes.
