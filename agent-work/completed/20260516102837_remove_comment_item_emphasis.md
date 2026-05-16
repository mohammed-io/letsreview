# remove_comment_item_emphasis

Status: active
Created: 2026-05-16 10:28:37

## Context

`.comment-item-emphasis` is causing feedback card height/line-height issues. User wants it removed and CSS cleaned up.

## Value Proposition

Simpler comment card styling with fewer special-case classes means fewer layout bugs.

## Alternatives Considered

1. Tweak `.comment-item-emphasis`
   - Pros: small.
   - Cons: user asked to remove it.
2. Remove emphasis class and fold needed fixed sizing into base comment item/list behavior
   - Pros: simpler, fixes source.
   - Cons: affects comment cards consistently.
   - Decision: best fit.

## Todos

- [x] Remove `.comment-item-emphasis` class generation.
- [x] Remove `.comment-item-emphasis` CSS.
- [x] Preserve scrollable feedback list behavior without emphasis class.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- No `comment-item-emphasis` remains in JS/CSS.
- Feedback list still scrolls.
- Comment cards no longer get forced line-height/height from emphasis class.
- Verification passes.
