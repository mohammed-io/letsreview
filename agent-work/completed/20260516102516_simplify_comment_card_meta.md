# simplify_comment_card_meta

Status: active
Created: 2026-05-16 10:25:16

## Context

Compact feedback comments should be default. Extra `compact` class is unnecessary. Header height should be smaller, and trash icon should use provided Heroicons-style SVG with reddish color.

## Value Proposition

Simpler markup and more polished comment card controls.

## Alternatives Considered

1. Keep compact option
   - Pros: supports multiple layouts.
   - Cons: YAGNI; user asked to remove.
2. Make all comment nodes compact
   - Pros: simplest.
   - Cons: changes modal/inline comment list too.
3. Use `variant` only for legacy lists
   - Pros: keeps review queue default compact while preserving existing inline/modal look.
   - Cons: small branch remains.
   - Decision: make default compact; legacy only when explicitly requested.

## Todos

- [x] Remove `compact` class/option from review comments.
- [x] Make default comment item header shorter.
- [x] Replace trash icon with provided SVG and reddish styling.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- Review feedback markup has `comment-meta` only, no `compact` class.
- Header is shorter.
- Delete icon uses requested SVG.
- Delete icon is reddish.
- Verification passes.
