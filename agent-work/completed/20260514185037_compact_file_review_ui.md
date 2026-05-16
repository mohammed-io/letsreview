# compact_file_review_ui

## Status: completed (20260514185350)

## Context
Current UI still has a left sidebar `<aside>` with mode/comparison/reviews. User wants aside removed, file list more compact like GitHub, and a GitHub-style Viewed checkbox that collapses file diff and marks file in list. File list should show comment count per file as `(2)`.

## Value Proposition
Reduce vertical space and align review workflow with GitHub: compact file navigation, visible reviewed status, and fast file collapse while preserving Canvas diff rendering.

## Alternatives considered (with trade-offs)
- Hide aside with CSS only: fast, but dead DOM and controls disappear.
- Move controls into topbar and keep compact file rail: best fit, removes aside while preserving functionality.
- Remove Sessions mode entirely: simpler UI, but loses existing review flow.
- Store viewed state server-side: durable, but overkill for current local review UI.

## Todos
- [x] Inspect current HTML/CSS/JS.
- [x] Remove sidebar aside and relocate controls to topbar.
- [x] Add per-file viewed state and collapse current file diff.
- [x] Show checkmark and `(n)` comment count in compact file list rows.
- [x] Update tests/static UI assertions.
- [x] Verify in browser and run tests.

## Acceptance Criteria
- No `<aside>` element remains for sidebar.
- File list rows use less vertical space and remain scannable.
- Viewed checkbox toggles active file collapsed state.
- Viewed files show a checkmark in file list.
- File list shows comment counts like `(2)` per file.
- Canvas renderer remains present.

## Notes
Viewed state is client-local for now.
