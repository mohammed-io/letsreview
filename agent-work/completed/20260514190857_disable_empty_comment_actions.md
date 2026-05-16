# disable_empty_comment_actions

## Status: completed (20260514191020)

## Context
Comments and Agent Payload buttons should be disabled when there are zero saved feedback comments. Disabled styling should be visibly gray normally, not only on hover.

## Value Proposition
Prevent empty modals/exports and make unavailable actions visually clear.

## Alternatives considered (with trade-offs)
- Only disable Comments: incomplete because payload is also comment-driven.
- Disable both based on total active-session comments: best fit for payload and file comments.
- Hide buttons: less discoverable and unlike GitHub-style disabled actions.

## Todos
- [x] Disable Comments when active file has zero comments.
- [x] Disable Agent Payload when active session has zero comments.
- [x] Strengthen disabled CSS for normal state.
- [x] Verify in browser and tests.

## Acceptance Criteria
- Comments button disabled with zero comments on active file.
- Agent Payload button disabled with zero comments in active session.
- Disabled buttons appear gray without hover.
- Tests pass.

## Notes
No backend change needed.
