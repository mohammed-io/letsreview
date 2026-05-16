# fix_live_session_file_swap

Status: active
Created: 2026-05-16 09:45:09

## Context

Line-click flicker still occurs. Earlier fix avoided direct active file replacement after session creation, but `ensureReviewSession()` can call `refreshSessions()`, which auto-selects a stored/new session even while Live mode is active. That swaps live file data with session snapshot data until live poll restores it, causing transient row/color mismatch.

## Value Proposition

Live mode should never replace active diff file with a session snapshot just because comment session metadata refreshed.

## Alternatives Considered

1. Disable live polling while inline review opens
   - Pros: hides snapback.
   - Cons: does not fix source, less live.
2. Prevent session auto-selection in Live mode
   - Pros: correct state ownership; small targeted fix.
   - Cons: must preserve Sessions mode auto-restore.
3. Split live/session active file state
   - Pros: robust long-term.
   - Cons: larger refactor; not needed now.

Decision: prevent `refreshSessions()` from selecting sessions unless Sessions mode is active.

## Todos

- [x] Patch session refresh auto-selection logic.
- [x] Verify live comment opening keeps live active file.
- [x] Run required verification.
- [x] Complete work.

## Acceptance Criteria

- Live mode session refresh does not call `selectSession()`.
- Sessions mode still restores/auto-selects sessions.
- Required Go verification commands pass.
