# build_diff_review_ui

## Status: completed (20260514122706)

## Context
Build a greenfield web-based diff review tool opened from a CLI command like `letsreview .`. It should compare current committed changes or arbitrary Git refs, support multiple comparison sessions, render performant highlighted diffs, and collect feedback that can be handed to AI coding agents.

## Value Proposition
Gives the user a local, fast review surface for Git diffs with AI-oriented context capture. The CLI should make review friction low while the UI keeps multiple reviews visible and switchable.

## Alternatives considered (with trade-offs)
1. Static HTML mock: fastest, but no real CLI, Git integration, or agent handoff.
2. Electron/Tauri app: richer desktop integration, but too much packaging and runtime weight for first usable version.
3. Ruby CLI + local HTTP UI: highly productive and scriptable, but weaker single-binary distribution and concurrent server ergonomics.
4. Go CLI + local HTTP UI: fast startup, simple single-binary distribution, strong process handling, and good fit for canvas-backed web UI assets.
5. Full server/database product: useful for teams, but unnecessary for a local review assistant MVP.

Selected approach: Go CLI + local HTTP UI with canvas-rendered diff rows and file-backed session/feedback state. It gives real behavior with low complexity and avoids TypeScript entirely.

## Todos
- [x] Scaffold Go project and `letsreview` CLI entrypoint.
- [x] Implement Git diff service with staged, unstaged, HEAD, and arbitrary ref comparisons.
- [x] Implement HTTP API for sessions, files, summaries, feedback, and agent handoff export.
- [x] Build web UI for session switching, ref selection, file navigation, canvas diff rendering, and feedback capture.
- [x] Add observable behavior tests for Git diff parsing and API behavior.
- [x] Run build/tests and review changed files for edge cases.

## Acceptance Criteria
- [x] `letsreview .` starts a local web UI for the supplied repository path.
- [x] UI can create and switch between multiple diff sessions.
- [x] UI can compare working tree changes and two Git refs.
- [x] Diff rows render with real syntax-style color highlighting in a canvas-backed viewer.
- [x] User can request a local explanation/summary for selected diff lines and save feedback.
- [x] Feedback can be exported as an agent instruction payload.
- [x] Tests cover observable behavior without asserting internal details.

## Notes
No Context7 MCP tool is configured in this environment, so library documentation lookup falls back to conservative use of stable Go standard-library APIs and local verification.
