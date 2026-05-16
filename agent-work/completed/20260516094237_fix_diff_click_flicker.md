# fix_diff_click_flicker

Status: active
Created: 2026-05-16 09:42:37

## Context

Clicking a diff line to add a comment flickers: visible rows jump down briefly, red/green backgrounds get visually corrupted, then recover. This likely comes from repeated full render/layout cycles while opening inline review and reading layout-dependent dimensions.

## Value Proposition

Stable line selection/comment opening makes review flow feel reliable and avoids visual confusion around changed lines.

## Alternatives Considered

1. Debounce render after mouseup
   - Pros: small change.
   - Cons: delays UI and can hide root cause.
2. Avoid render loops during inline panel positioning
   - Pros: direct fix for layout thrash/flicker.
   - Cons: needs careful state/render order.
3. Move inline panel out of diff stage
   - Pros: no canvas overlay layout interaction.
   - Cons: larger UI change; unnecessary now.

Decision: inspect click/render flow and remove layout thrash with minimal targeted changes.

## Todos

- [x] Inspect click-to-comment render path.
- [x] Fix flicker/root cause without broad UI rewrite.
- [x] Run required verification.
- [x] Review edge cases and complete work.

## Acceptance Criteria

- Clicking a line opens comment UI without transient row jump/corrupted diff colors.
- Drag selection still works.
- Comment markers still selectable.
- Keyboard shortcut `C` still opens comment UI.
- Required Go verification commands pass.
