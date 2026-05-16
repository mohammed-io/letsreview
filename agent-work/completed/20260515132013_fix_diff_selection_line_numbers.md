# fix_diff_selection_line_numbers

## Status: completed (20260515132232)

## Context
Diff canvas selection currently uses visible row indexes as line numbers. When context lines are hidden/collapsed, UI shows ranges like `Lines 13-15` even when actual file lines are `22-24`.

## Value Proposition
Comments and explain requests must reference actual file line numbers so agents can edit correct code.

## Alternatives considered (with trade-offs)
1. Keep row indexes and add offset math: fragile across hunks and deleted lines.
2. Store both row indexes and actual line numbers: best fit; canvas still selects rows, payload uses file line numbers.
3. Expand all hidden context: avoids mismatch, but worsens performance and UI density.

## Todos
- [x] Inspect selection/comment/explain code paths.
- [x] Add row-to-file-line conversion in UI.
- [x] Send/display actual file line ranges.
- [x] Adjust server selection summary lookup if needed.
- [x] Add/adjust tests for actual line payloads.
- [x] Run tests/build checks.

## Acceptance Criteria
- [x] Selected changed lines display actual file line numbers.
- [x] Feedback payload uses actual file line numbers.
- [x] Explain request uses actual file line numbers.
- [x] Existing comment markers still render in correct canvas rows.

## Install

- [x] Copied updated binary to `/Users/mohammed/.local/bin/letsreview`.

## Notes
Do not overwrite unrelated user changes.
