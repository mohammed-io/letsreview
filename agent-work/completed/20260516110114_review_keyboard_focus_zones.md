# review_keyboard_focus_zones

Status: active
Created: 2026-05-16 11:01:14

## Context

Keyboard review flow needs focus zones. Space should open inline review for selected diff line. Tab should toggle visible focus between code diff and feedback queue. In diff zone, j/k move source lines. In queue zone, j/k move comments. Shift+j/k should be removed. Input should not auto-focus when opening inline review; `i` should focus textarea, and Esc should leave textarea/inline input focus.

## Value Proposition

Clear, Vim-like review navigation with simple modes improves speed and avoids hidden shortcut complexity.

## Alternatives Considered

1. Keep Shift+j/k for comment navigation
   - Pros: existing behavior.
   - Cons: user rejected; less discoverable.
2. Use browser Tab naturally
   - Pros: standard.
   - Cons: no clear app focus zone model.
3. Add two explicit focus zones: diff and queue
   - Pros: simple mental model; visible border; clean handlers.
   - Cons: intercepts Tab in app shell.
   - Decision: best fit.

## Todos

- [x] Add focus zone state and visual indicators.
- [x] Make Tab toggle diff/queue zones.
- [x] Make j/k zone-aware and remove Shift+j/k behavior.
- [x] Add Space to open inline review without focusing input.
- [x] Add i/Esc behavior for inline review textarea focus.
- [x] Update shortcut help.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- Default focus zone is code diff.
- Tab toggles visible focus border between diff and queue.
- Diff zone j/k and arrows move selected code line.
- Queue zone j/k move selected feedback item.
- Space opens inline review only when line is selected.
- Opening inline review does not focus textarea by default.
- `i` focuses comment textarea; Esc blurs it/leaves input focus.
- Verification passes.
