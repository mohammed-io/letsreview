# remove_session_summary_line

Status: active
Created: 2026-05-16 09:56:50

## Context

Header session summary line can show noisy text like "Failed to fetch" and change summaries. User wants this line removed entirely.

## Value Proposition

Cleaner review header: repo/session title only, no distracting transient fetch or diff summary text.

## Alternatives Considered

1. Hide summary only on errors
   - Pros: preserves useful summary.
   - Cons: user explicitly wants line removed completely.
2. Keep DOM hidden with CSS
   - Pros: small.
   - Cons: stale JS keeps writing unused state.
3. Remove/ignore summary element in markup and JS
   - Pros: clean, no noisy UI.
   - Cons: slightly broader edit.

Decision: remove summary element and guard JS references.

## Todos

- [x] Remove session summary line from markup.
- [x] Update JS so missing summary element is safe/no-op.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- Header no longer renders `session-summary` line.
- Fetch errors do not appear in removed summary line.
- UI JS does not throw when summary element is absent.
- Verification passes.
