# github_style_trash_icon

Status: active
Created: 2026-05-16 10:10:17

## Context

Feedback queue delete control currently uses a red ×. User wants GitHub-style trash icon instead.

## Value Proposition

Trash icon is familiar, clearer, and visually consistent with GitHub-style review UI.

## Alternatives Considered

1. Use text label "Delete"
   - Pros: accessible/clear.
   - Cons: too large for compact header.
2. Use × icon
   - Pros: tiny.
   - Cons: ambiguous/weird; rejected.
3. Inline SVG trash icon
   - Pros: no deps, GitHub-like, accessible with aria-label.
   - Cons: small markup addition.
   - Decision: best fit.

## Todos

- [x] Replace compact delete × with inline trash SVG.
- [x] Adjust icon button CSS to match GitHub-style compact danger affordance.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- Compact feedback delete button shows trash can icon, not ×.
- Button remains accessible via aria-label.
- Existing delete behavior unchanged.
- Verification passes.
