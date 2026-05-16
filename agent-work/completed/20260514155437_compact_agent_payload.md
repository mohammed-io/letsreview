# compact_agent_payload

## Status: completed (20260514183450)

## Context
Export agent payload currently returns the repo path, full session, all files/diffs, and directive. User wants export to contain only comments tied to specific code lines or line ranges. User also wants per-line/range comment markers, click-to-view comments, and delete support.
User also wants `WEB_UI_ROOT` env support so dev server can serve static UI files from disk without restarting CLI after HTML/CSS/JS edits.

## Value Proposition
Keep exported agent input focused and small: comments with exact file path and selected line range only.

## Alternatives considered (with trade-offs)
- UI-only filter after full payload fetch: easy, but still sends too much data from API.
- Replace export with raw feedback list: compact, but less explicit as agent payload.
- Server returns `comments` array with file/range/body/timestamp: best fit, small and precise.
- Include selected code snippets too: useful context, but user explicitly said only comments on specific lines/ranges.

## Todos
- [x] Inspect current export payload and tests.
- [x] Change server export payload to comments-only shape.
- [x] Add delete feedback API and observable tests.
- [x] Render Canvas comment markers/counts for matching line ranges.
- [x] Show existing comments in inline panel and support delete.
- [x] Update tests to reject full session/diff payload.
- [x] Add `WEB_UI_ROOT` static file override and tests.
- [x] Verify UI still exports compact JSON and tests pass.

## Acceptance Criteria
- `/api/sessions/{id}/agent-payload` returns only `comments`.
- Each exported comment includes only `filePath`, `startLine`, `endLine`, `body`, and `createdAt`.
- Payload does not include `session`, full files/diffs, repo path, or directive.
- Diff shows count markers for comments on relevant line/range.
- Clicking a comment marker opens comments for that line/range.
- Existing comments can be deleted.
- `WEB_UI_ROOT=/path/to/web` serves static files from disk instead of embedded assets.
- Existing feedback save flow still works.

## Notes
Frontend remains Canvas-rendered; marker drawing stays on Canvas, comment editor/list remains DOM overlay.
