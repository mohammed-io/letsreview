# redesign_review_cockpit

Status: active
Created: 2026-05-16 10:45:06

## Context

Review cockpit needs redesign using frontend-design/refactoring-ui principles. Diff toolbar should be simplified to filename + Viewed checkbox only. Explain selection belongs in inline review modal. Submit review should no longer appear in diff actions. Clear session should move to topbar.

## Value Proposition

Cleaner review flow: canvas stays focused on code, inline review owns line-level actions, topbar owns session-level actions, cockpit owns review feedback/submission.

## Alternatives Considered

1. Keep existing toolbar and restyle
   - Pros: lowest risk.
   - Cons: keeps action clutter near code.
2. Move all actions into cockpit
   - Pros: very clean toolbar.
   - Cons: explain selection is line-specific and fits inline modal better.
3. Topbar session actions + inline line actions + cockpit feedback actions
   - Pros: clear hierarchy by scope; best UX.
   - Cons: touches markup/CSS/JS.
   - Decision: best fit.

## Todos

- [x] Move Clear session to topbar and simplify diff actions to filename + Viewed only.
- [x] Move Explain selection into inline review modal.
- [x] Redesign review cockpit visual hierarchy and feedback queue.
- [x] Update JS selectors/event behavior for moved controls.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- Diff toolbar only shows active filename and Viewed checkbox.
- Inline review modal contains Explain selection action.
- Topbar contains Clear session action.
- Submit review not present in diff actions.
- Review cockpit feels intentional, compact, and visually polished.
- Existing save/explain/delete/submit behavior works.
- Verification passes.
