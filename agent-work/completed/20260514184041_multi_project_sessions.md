# multi_project_sessions

## Status: completed (20260514184731)

## Context
User needs one fixed-port server that can host multiple project sessions concurrently. Starting CLI for same folder should reuse same session ID, computed as md5 of absolute folder path. CLI should join existing server when possible and heartbeat session every 10 seconds. Feedback inline modal should open after mouseup, not mousedown.

## Value Proposition
Avoid duplicate servers on port 55492, support multiple repositories in one web server, make project session URLs deterministic, and improve line-selection UX.

## Alternatives considered (with trade-offs)
- Keep one-repo server: smallest change, but fails multi-project requirement.
- Project-scoped APIs plus CLI join existing fixed-port server: best current fit, deterministic and low process risk.
- Full background daemon: smoother UX, but needs lifecycle/process supervision not present in repo.
- External registry/socket: too much complexity for local-only fixed-port tool.

## Todos
- [x] Inspect current server, CLI, UI, and tests.
- [x] Add project session model keyed by md5 absolute repo path.
- [x] Add scoped project APIs and keep legacy routes working for current tests.
- [x] Update CLI to join existing server or start one server, then heartbeat every 10 seconds.
- [x] Update frontend to read `project` query param and call scoped APIs.
- [x] Move inline feedback opening from mousedown to mouseup.
- [x] Add observable tests for md5 project IDs, project scoping, heartbeat, and CLI config behavior.
- [x] Run tests and browser verification.

## Acceptance Criteria
- Same absolute folder path always maps to same md5 project session ID.
- Different project folders can coexist in one server.
- CLI on existing port registers/joins project instead of starting another server.
- UI URL includes project ID and API calls are scoped to that project.
- Feedback modal opens after mouseup.
- Tests pass.

## Notes
Keep names stable where possible: existing review sessions remain `Session`; new repo-level session is `Project`.
