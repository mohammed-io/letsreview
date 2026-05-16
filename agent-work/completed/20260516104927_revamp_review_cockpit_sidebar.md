# revamp_review_cockpit_sidebar

Status: active
Created: 2026-05-16 10:49:27

## Context

Review cockpit sidebar needs complete visual revamp using frontend-design and refactoring-ui principles, but should stay simple.

## Value Proposition

Sidebar should feel like a focused review command center: clear hierarchy, compact metrics, obvious submit action, and readable feedback queue without noisy decoration.

## Alternatives Considered

1. Dense GitHub-style sidebar
   - Pros: familiar.
   - Cons: can feel flat/no hierarchy.
2. Premium cockpit card system
   - Pros: distinctive and polished.
   - Cons: risk over-design.
3. Simple layered command panel
   - Pros: clean hierarchy, small code changes, practical.
   - Cons: less flashy.
   - Decision: best current fit.

## Todos

- [x] Redesign sidebar structure/visual hierarchy in markup.
- [x] Update cockpit CSS with simple polished system.
- [x] Preserve existing IDs and behavior.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- Sidebar has clearer title, metrics, actions, and feedback queue hierarchy.
- Design is simple, not noisy.
- Existing JS behavior/IDs continue working.
- Verification passes.
