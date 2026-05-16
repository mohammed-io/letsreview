# clear_review_session

## Status: in_progress

## Context
User wants topbar button to clear current session, with confirmation. Clearing should remove stored position, active file, feedback/comments, viewed state, drafts, and related session data.

## Value Proposition
Provide an explicit reset for review state when local session data or comments should be discarded.

## Alternatives considered (with trade-offs)
- Client-only clear: clears UI state, but server comments remain and come back.
- Server-only delete session: clears comments, but local file/scroll/drafts remain.
- Combined server delete + sessionStorage clear: best fit, complete reset.

## Todos
- [x] Add server endpoint to delete review session and feedback.
- [x] Add Clear session button with confirmation.
- [x] Clear sessionStorage project keys and local UI state.
- [x] Fix clear-session to clear client state + reload page (no server delete needed).
- [x] Add visible shortcut label to Save feedback button (CMD/Ctrl+Enter).
- [x] Verify tests pass.

## Acceptance Criteria
- Clear button exists in topbar/actions.
- Clear asks for confirmation.
- Server feedback/session data is removed.
- Client storage file/scroll/draft/viewed state is removed.
- UI returns to clean current live diff state.

## Notes
Use browser `confirm` for now; custom modal can come later if needed.
