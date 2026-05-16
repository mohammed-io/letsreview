# improve_comment_actions

## Status: completed (20260514191237)

## Context
User wants Cmd+Enter to save feedback. Delete works inline but not inside file comments modal because events only delegate from inline list. Comment display should be cleaner.

## Value Proposition
Make comment workflows fast and reliable, with readable GitHub-like comment cards.

## Alternatives considered (with trade-offs)
- Add key handler only: partial; delete modal remains broken.
- Add separate delete handlers per list: works, but duplicate logic.
- Use shared delete delegation and shared card renderer: best fit, minimal duplication.

## Todos
- [x] Add Cmd/Ctrl+Enter save feedback shortcut.
- [x] Fix delete from file comments modal.
- [x] Improve comment card display.
- [x] Verify browser behavior and tests.

## Acceptance Criteria
- Cmd+Enter and Ctrl+Enter save feedback.
- Delete works from inline comments and file comments modal.
- File comments modal updates/ closes disabled state after delete.
- Comment cards are easier to read.

## Notes
Frontend-only change.
