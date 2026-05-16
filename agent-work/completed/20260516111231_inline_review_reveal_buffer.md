# inline_review_reveal_buffer

Status: active
Created: 2026-05-16 11:12:31

## Context

When selecting last visible canvas line, inline review is still too close to/out of view. Need extra buffer when revealing inline review.

## Value Proposition

Comment box opens comfortably with breathing room, even near bottom of viewport.

## Alternatives Considered

1. Increase fixed overflow padding
   - Pros: simple.
   - Cons: may over-scroll slightly.
   - Decision: best fit.
2. Reposition panel above selected line near bottom
   - Pros: always visible.
   - Cons: violates line-anchored behavior expectation.

## Todos

- [x] Add larger reveal buffer for inline review.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- Opening inline review near bottom scrolls enough extra space.
- Inline review remains line anchored.
- Verification passes.
