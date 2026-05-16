# compact_diff_context

## Status: completed (20260514185745)

## Context
Diff loader currently asks git for 80 context lines, which makes Canvas show too much unchanged code around actual changes. User wants 5-10 context lines by default, with option to show more like GitHub.

## Value Proposition
Make reviews denser and focused by default while preserving a clear way to request more surrounding code.

## Alternatives considered (with trade-offs)
- Hard-code `--unified=8`: smallest, but no way to show more lines.
- Add global context selector: good current fit, simple server/UI, supports more context on demand.
- Add GitHub-style per-hunk expand controls in Canvas: best long-term UX, more hit-testing/rendering work.
- Load full diff and hide context client-side: wasteful and still sends too much data.

## Todos
- [x] Inspect current diff loader and UI.
- [x] Add `contextLines` request field with default 8 and validation.
- [x] Wire live/session APIs and UI context selector.
- [x] Add tests for default and custom unified context.
- [x] Verify UI refresh and tests.

## Acceptance Criteria
- Default git diff context is 8 lines.
- User can request more context from UI.
- Live diff and created review sessions honor selected context.
- Tests pass.

## Notes
Per-hunk expand can be added later if needed; global selector unblocks current review density issue.
