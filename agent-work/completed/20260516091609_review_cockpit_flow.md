# review_cockpit_flow

Status: active
Created: 2026-05-16 09:16:09

## Context

Review flow needs stronger UX for human-in-loop code review. Current UI has inline feedback, but comments/feedback are not prominent enough and keyboard workflow needs more power-user shortcuts.

## Value Proposition

A cockpit layout keeps code context central while making review state, comments, drafts, and submit actions visible at all times. This reduces missed feedback, speeds review, and makes letsreview feel purpose-built instead of generic diff viewer.

## Alternatives Considered

1. GitHub-like inline review
   - Pros: familiar, low learning curve.
   - Cons: comments can get buried in long diffs.
2. Review cockpit layout
   - Pros: feedback always visible, strong review progress, keyboard-friendly.
   - Cons: more UI/layout work.
   - Decision: best fit for letsreview human-in-loop review.
3. IDE-like keyboard-first review
   - Pros: fastest for power users.
   - Cons: lower discoverability.
4. Feedback-first sidebar only
   - Pros: easy emphasis for comments.
   - Cons: less cohesive review-flow model.

## Todos

- [x] Inspect current web UI structure and data flow.
- [x] Design cockpit layout with persistent review panel and emphasized feedback cards.
- [x] Add keyboard shortcuts for file, hunk, comment, review, and submit actions.
- [x] Add/adjust tests for observable review UI behavior where practical.
- [x] Run gofmt/build/test/vet checks.
- [x] Review edge cases and polish.

## Acceptance Criteria

- Comments and feedback are visible outside inline code context.
- Review panel shows open/draft counts and submit affordance clearly.
- Keyboard shortcut overlay or help is discoverable.
- Shortcuts do not hijack typing in inputs/textareas.
- Existing review/session/live flows still work.
- Required Go verification commands pass.

## Notes

Do not touch unrelated `.kilo/`, old `agent-work/`, or `mcp` binary unless user requests.
