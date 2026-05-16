# GitHub-like Live Diff UI (Canvas-based)

**Status:** in_progress
**Created:** 2026-05-14T15:31:05

---

## Context

Current UI uses canvas-based rendering. User wants:
1. Keep canvas rendering (NO DOM switch)
2. Live diff that auto-updates when files change
3. GitHub-like styling on canvas

## Value Proposition

- Real-time diff viewing without manual refresh
- Familiar GitHub-like colors/layout
- Fast canvas performance maintained

## Todos

- [x] Add `/api/live` endpoint for working tree diff
- [x] Add auto-refresh polling (2s interval)
- [x] Improve canvas styling to GitHub-like appearance
- [x] Add file tree with status icons (A/M/D/R)
- [x] Add live mode toggle in sidebar
- [x] Keep session/feedback system

## Acceptance Criteria

- [x] Diff auto-updates every 2s in "live" mode
- [x] GitHub-like colors (green/red on dark)
- [x] File tree shows status icons
- [x] Sessions still work for feedback workflow

## Acceptance Criteria

- [ ] Diff auto-updates every 2s in "live" mode
- [ ] GitHub-like colors (green/red on dark)
- [ ] File tree shows status icons
- [ ] Sessions still work for feedback workflow

## Notes

- Keep canvas rendering
- Poll `/api/live` endpoint
- Auto-refresh when file changes detected
