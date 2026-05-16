# tabindex_focus_zones

Status: active
Created: 2026-05-16 11:05:38

## Context

User does not want visible "Diff focus" badge. Focus zones should use native `tabindex` where possible, with border-only indication.

## Value Proposition

Native focusable regions improve accessibility and reduce custom focus-model weirdness while keeping keyboard review navigation clear.

## Alternatives Considered

1. Keep custom badge
   - Pros: explicit.
   - Cons: user rejected; noisy on canvas.
2. Use `tabindex` with focus/blur zone sync
   - Pros: native, simple, accessible.
   - Cons: still need global shortcut dispatch for Vim keys.
   - Decision: best fit.
3. Only use CSS `:focus-within`
   - Pros: minimal JS.
   - Cons: queue/diff zone state needed for j/k behavior.

## Todos

- [x] Add tabindex to diff and queue focus zones.
- [x] Sync focus zone state from native focus.
- [x] Remove Diff focus badge CSS, keep border only.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- No "Diff focus" text appears.
- Diff and queue are focusable via tabindex.
- Border indicates active zone.
- Tab still toggles zones.
- Verification passes.
