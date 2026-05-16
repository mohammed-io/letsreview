# reproduce_diff_click_flicker

Status: active
Created: 2026-05-16 09:47:27

## Context

User still sees flicker when clicking diff lines. Need browser-level reproduction, but chrome-devtools MCP tools are not exposed in this harness. Must inspect event/render path and fix likely visual race without overwriting unrelated changes.

## Value Proposition

Stable review selection is core UX. Flicker during comment creation makes code colors and line positions untrustworthy.

## Alternatives Considered

1. Browser reproduce with chrome-devtools MCP
   - Pros: direct observation.
   - Cons: unavailable in current tool namespace.
2. Add render instrumentation
   - Pros: can diagnose locally.
   - Cons: noisy and not product fix.
3. Remove async session creation from click path
   - Pros: likely fixes delayed second render/snapback and makes click instant.
   - Cons: comment textarea opens before backing session exists; save must ensure session.

Decision: remove async work from click-to-open path. Ensure/create session only when saving/explaining/submitting.

## Todos

- [x] Refactor comment open to be synchronous/no API refresh.
- [x] Ensure save/explain still creates session before API calls.
- [x] Run required verification.
- [x] Complete work.

## Acceptance Criteria

- Clicking/dragging diff selection opens inline review without waiting on session APIs.
- Live mode active file is not swapped during open.
- Save feedback still creates/reuses review session.
- Explain selection still creates/reuses review session.
- Required Go verification commands pass.
