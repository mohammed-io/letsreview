# github_hunk_context

## Status: completed (20260514190256)

## Context
User clarified desired behavior: GitHub-style compact hunks with only a few surrounding context lines around each patch. Context selector is not wanted.

## Value Proposition
Make diff view focus on patches, not surrounding file content, matching GitHub's default review density.

## Alternatives considered (with trade-offs)
- Keep selector at 8/20/80/200: flexible, but misses GitHub-like default and clutters UI.
- Use `--unified=3`: simple, GitHub-like, good current fit.
- Implement per-hunk expand controls now: closest to GitHub, but more Canvas hit-testing/rendering work.

## Todos
- [x] Clarify desired GitHub-style hunk context.
- [x] Remove context selector UI/state.
- [x] Change default diff context to 3 lines.
- [x] Keep API accepting optional context for future/internal use but no visible selector.
- [x] Update tests and verify.

## Acceptance Criteria
- UI has no context selector.
- Default git diff uses `--unified=3`.
- Diff display shows compact hunks around patches by default.
- Tests pass.

## Notes
Per-hunk expand controls can be added later if user wants exact GitHub expand buttons.
Verification note: email-client still has large hunks because git reports long contiguous changed regions. `--unified=3` only limits unchanged context around hunks; hiding unchanged runs inside huge hunks needs Canvas-side folding.
