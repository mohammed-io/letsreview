# fix_idiomorph_comment_scope

Status: active
Created: 2026-05-16 10:21:06

## Context

Idiomorph update throws `HierarchyRequestError: Failed to execute 'insertBefore' on 'Node': The new child element contains the parent.` User wants Idiomorph only for comments section, not inline comments above canvas.

## Value Proposition

Limit morphing to review feedback queue only and avoid DOM hierarchy errors while preserving stable inline canvas overlay behavior.

## Alternatives Considered

1. Keep morph helper for all comment lists
   - Pros: consistent.
   - Cons: causes error and affects inline canvas overlay.
2. Disable Idiomorph entirely
   - Pros: safe.
   - Cons: user asked to use Idiomorph for comments section.
3. Scope Idiomorph to review queue with HTML-string children
   - Pros: avoids parent containment issue and respects user scope.
   - Cons: modal/inline lists use normal replacement.
   - Decision: best fit.

## Todos

- [x] Scope Idiomorph to `review-comment-list` only.
- [x] Restore normal replacement for inline/modal comment lists.
- [x] Avoid parent-containing morph input.
- [x] Run focused verification.
- [x] Complete work.

## Acceptance Criteria

- Idiomorph runs only for review feedback queue.
- Inline comment list above canvas does not use Idiomorph.
- No parent containment morph input is passed.
- Existing comment click/delete behavior works.
- Verification passes.
