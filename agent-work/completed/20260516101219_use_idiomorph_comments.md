# use_idiomorph_comments

Status: active
Created: 2026-05-16 10:12:19

## Context

User wants Idiomorph installed and comment list replacement to use Idiomorph instead of direct `replaceChildren`, preserving DOM identity and reducing UI churn.

## Value Proposition

Morphing comment lists keeps scroll/focus/DOM continuity better than full replacement and aligns with modern HTML-over-the-wire style updates.

## Alternatives Considered

1. Keep `replaceChildren`
   - Pros: simple.
   - Cons: full DOM replacement; user requested Idiomorph.
2. Add npm package + bundle step
   - Pros: package managed.
   - Cons: project has no frontend build pipeline; adds complexity.
3. Vendor browser build and load as static script
   - Pros: no build pipeline, simple stdlib-style static UI.
   - Cons: vendored JS file.
   - Decision: best current fit.

## Todos

- [x] Get Idiomorph browser build/docs fallback and install static asset.
- [x] Load Idiomorph in UI.
- [x] Replace comment list DOM updates with Idiomorph morphing.
- [x] Run required verification.
- [x] Complete work.

## Acceptance Criteria

- Idiomorph static asset is present and loaded before app JS.
- Comment list updates use Idiomorph when available.
- Safe fallback exists if Idiomorph is unavailable.
- Existing comment delete/click behavior still works.
- Required verification passes.
