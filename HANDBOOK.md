# letsreview Handbook

`letsreview` is a local code-review assistant. It opens a browser UI for Git diffs, lets a human leave line feedback, then exposes submitted feedback to an AI agent through MCP.

## Mental Model

There are two binaries:

- `letsreview`: CLI + local HTTP server + embedded web UI. Subcommands: `mcp`, `stop`.
- `letsreview mcp`: same binary in stdio MCP server mode for agents.

There are two review surfaces:

- **Live**: always shows current working tree vs `HEAD`, refreshed every 2 seconds.
- **Sessions**: snapshots a chosen comparison so review comments attach to stable diff rows.

The app never initializes Git repos. It accepts a path, but diff loading expects that path to already be a Git repository. Non-Git paths fail when Live or Sessions diff data is requested.

## Build

From repo root:

```sh
go build -o letsreview .
```

During sandboxed/local dev, use writable Go cache if needed:

```sh
GOCACHE=/private/tmp/letsreview-gocache go build -o letsreview .
```

## Start UI

```sh
./letsreview /path/to/git/repo
```

Default address:

```txt
127.0.0.1:55492
```

Example output:

```txt
letsreview is running at http://127.0.0.1:55492?project=89a35363ec8de7131a16c2ed7419999a
reviewing /path/to/git/repo
```

Open URL in browser. Running without arguments shows help.

## CLI Flags

```sh
letsreview <repo>
```

A repo path is required. No default is assumed.

Available flags:

```txt
-addr string
    Address to listen on. Default: 127.0.0.1:55492

-no-open
    Don't open browser automatically.
```

Examples:

```sh
./letsreview ~/Projects/email-client
./letsreview -addr 127.0.0.1:6000 ~/Projects/email-client
./letsreview -no-open .
```

## Multiple Repos

First `letsreview` process starts HTTP server.

If another `letsreview` process starts with same `-addr`, it does not start a second server. It registers its repo with existing server through:

```txt
POST /api/projects
```

Then it prints a project-scoped URL:

```txt
http://127.0.0.1:55492?project=<project-id>
```

That lets multiple repos be open in same UI server.

Project IDs are MD5 hashes of absolute repo paths.

## Live Mode

Live mode is default UI mode.

It calls:

```txt
GET /api/live
GET /api/projects/{projectID}/live
```

Behavior:

- shows working tree vs `HEAD`
- refreshes every 2 seconds
- uses default 3 context lines
- keeps active file/scroll in browser `sessionStorage`
- can mark files as viewed

Live mode is best for active work while files keep changing.

## Sessions Mode

Sessions mode creates snapshot reviews.

Supported diff types:

- `working`: working tree vs `HEAD`
- `staged`: staged changes
- `refs`: two refs, needs `baseRef` and `headRef`

Create session through UI or API:

```txt
POST /api/sessions
POST /api/projects/{projectID}/sessions
```

Example request:

```json
{
  "mode": "refs",
  "baseRef": "main",
  "headRef": "feature-branch"
}
```

Session diffs are snapshots. If files change later, create a new session.

## Diff Rendering

Diff lines render on `<canvas id="diff-canvas">`.

Why:

- large diffs stay fast
- scrolling avoids huge DOM row count
- row highlights, comment markers, explanation markers draw cheaply

Line syntax highlighting is lightweight token coloring in browser JavaScript. It is not language-server semantic highlighting.

## Keyboard Navigation

In review mode (session active):

- `j` / `k` — move selection down/up
- `Shift+j` / `Shift+k` — extend selection down/up (multi-line)
- `Tab` — toggle focus between diff and comment queue
- `Space` — open inline review for selection
- `i` — open inline review and focus input
- `n` / `p` — next/previous file
- `v` — toggle viewed status on current file
- `f` — focus file list
- `?` — show keyboard shortcuts
- `Escape` — close modals / blur input
- `Cmd+Enter` / `Ctrl+Enter` — save feedback (in input) or submit review
- Number prefix before motion (e.g. `3j` moves 3 lines)

## Selecting Lines

In canvas:

- click row to select single line
- drag to select range
- mouse wheel scrolls diff
- selecting opens inline review box when session is available

In Live mode, selecting/commenting auto-creates a working-tree review session if needed. Comments attach to session snapshot.

## Comments

Selected lines can receive comments.

Comment behavior:

- `Save feedback` stores comment on active session
- `Cmd+Enter` on macOS saves
- `Ctrl+Enter` elsewhere saves
- comments can be deleted
- comments can be resolved by AI agent (marked with green badge, dimmed)
- file comment count appears beside filename
- `Comments` button opens file comment modal

API:

```txt
POST   /api/sessions/{id}/feedback
DELETE /api/sessions/{id}/feedback/{feedbackID}
PATCH  /api/sessions/{id}/feedback/{feedbackID}/resolve
GET    /api/sessions/{id}/agent-payload
```

Agent payload includes comment ID and resolved status:

```json
{
  "comments": [
    {
      "id": "abc123",
      "filePath": "main.go",
      "startLine": 1,
      "endLine": 2,
      "body": "Rename this for clarity.",
      "createdAt": "2026-05-15T...",
      "resolved": false,
      "resolvedAt": null
    }
  ]
}
```

The `id` field lets agents call `resolve_feedback` after processing each comment.

## Submit Review

`Submit review` sends unresolved review comments to agent. Reviews are unlimited — reviewer can submit multiple times per session.

Behavior:

- requires at least one unresolved comment
- asks for browser confirmation
- publishes `review_submitted` event
- only unresolved comments are included (resolved comments are excluded)
- each submit creates a new event; agent picks up all via `get_pending_events`
- comments remain editable after submit; reviewer can add more and submit again

API:

```txt
POST /api/sessions/{id}/submit-review
POST /api/projects/{projectID}/sessions/{id}/submit-review
```

Submitted review includes:

- session ID
- comments
- submitted timestamp
- repo path
- touched files
- summary

## Clear Session

`Clear session` clears browser-side state for current project, then reloads page.

It removes browser `sessionStorage` entries for:

- active session
- active file
- scroll position
- drafts
- viewed state

Current UI implementation reloads page after clear.

## Explain Selection

`Explain selection` sends selected range to server.

API:

```txt
POST /api/sessions/{id}/explain
GET /api/sessions/{id}/explanation-requests
GET /api/sessions/{id}/explanations
POST /api/sessions/{id}/explanations
```

Current behavior:

- server returns local summary immediately
- server stores explanation request
- UI polls explanations every 3 seconds
- MCP agent can read requests and submit explanation
- explanation marker appears on canvas when received

This creates human-to-agent loop:

1. Human selects lines.
2. Human clicks `Explain selection`.
3. Agent sees request via MCP.
4. Agent submits explanation.
5. Browser shows explanation inline/marker.

## MCP Server

Start MCP server:

```sh
./letsreview mcp
```

Default HTTP address:

```txt
127.0.0.1:55492
```

Override:

```sh
./letsreview mcp -addr 127.0.0.1:6000
```

MCP protocol is JSON-RPC over stdio. It implements:

- `initialize`
- `ping`
- `tools/list`
- `tools/call`
- `subscriptions/listen` (for MCP clients that support async notifications)

Protocol version:

```txt
2024-11-05
```

Server info:

```txt
name: letsreview
version: 0.1.0
```

Capabilities advertised in `initialize`:

- `tools`
- `subscriptions`

## Event Model

All review activity is tracked as events. Each event has a monotonically increasing `seq` number.

Event types:

- `explanation_requested` — reviewer asked for explanation on code lines
- `explanation_submitted` — agent submitted explanation for code lines
- `review_submitted` — reviewer submitted comments

Events are stored in memory per session. Agent reads them via `get_pending_events` (non-blocking poll).

## MCP Tools

### `request_code_review`

Starts HTTP server if needed, registers repo, creates review session, opens browser, returns session info with event cursor.

Input:

```json
{
  "repoPath": "/absolute/path/to/repo",
  "mode": "working",
  "baseRef": "",
  "headRef": ""
}
```

`mode` can be:

- `working`
- `staged`
- `refs`

For `refs`, provide `baseRef` and `headRef`.

Output includes:

- `sessionId`
- `url`
- `projectID`
- `files`
- `summary`
- `lastEventSeq`

After this returns, periodically call `get_pending_events` with `sessionId` and `lastEventSeq` to detect new events.

### `get_pending_events`

Non-blocking poll for new events. Returns immediately — never blocks. Call this periodically between user interactions.

Input:

```json
{
  "sessionId": "...",
  "afterSeq": 0
}
```

Use `lastEventSeq` from `request_code_review` for initial `afterSeq`. Update `afterSeq` to the latest `seq` from returned events on each call.

Output:

```json
{
  "sessionId": "...",
  "events": [
    {
      "seq": 1,
      "type": "explanation_requested",
      "sessionId": "...",
      "explanationRequest": {
        "id": "...",
        "filePath": "main.go",
        "startLine": 12,
        "endLine": 18
      }
    }
  ],
  "count": 1,
  "lastSeq": 1
}
```

Event types in `events` array:

- `explanation_requested` — reviewer wants explanation (has `explanationRequest` field)
- `explanation_submitted` — agent submitted explanation (has `explanation` field)
- `review_submitted` — reviewer submitted comments (has `review` field with comments array)

### `get_review_result`

Returns the latest submitted review for a session. Call after `get_pending_events` returns a `review_submitted` event.

Input:

```json
{
  "sessionId": "..."
}
```

Returns submitted review or error if no review submitted yet. Multiple reviews can be submitted per session; this returns the most recent.

### `cancel_review`

Input:

```json
{
  "sessionId": "..."
}
```

Current behavior returns cancelled status. It does not remove server state.

### `resolve_feedback`

Mark a feedback comment as resolved after the AI agent has applied the requested change. The comment remains visible in the web UI with a resolved indicator (green badge, dimmed). Submit review only sends unresolved comments.

Input:

```json
{
  "sessionId": "...",
  "commentId": "abc123"
}
```

Returns `resolved` status. Returns error if session or comment not found.

### `submit_explanation`

Submit an explanation for specific code lines. Call when `get_pending_events` returns an `explanation_requested` event.

Input:

```json
{
  "sessionId": "...",
  "filePath": "main.go",
  "startLine": 12,
  "endLine": 18,
  "explanation": "This changes validation path..."
}
```

Creates `explanation_submitted` event. Explanation appears inline in reviewer's browser.

## Agent Flow

Recommended agent flow:

1. Call `request_code_review` → get `sessionId` + `lastEventSeq`
2. Periodically call `get_pending_events` with `sessionId` + `afterSeq`
3. On `explanation_requested` → call `submit_explanation`
4. On `review_submitted` → call `get_review_result` to get comments
5. Apply each comment as a code change, then call `resolve_feedback` with the comment `id`
6. Update `afterSeq` from `lastSeq` after each poll

## HTTP API Summary

Legacy/default-project routes:

```txt
GET    /api/health
GET    /api/live
GET    /api/sessions
POST   /api/sessions
GET    /api/sessions/{id}
DELETE /api/sessions/{id}
POST   /api/sessions/{id}/explain
POST   /api/sessions/{id}/feedback
DELETE /api/sessions/{id}/feedback/{feedbackID}
PATCH  /api/sessions/{id}/feedback/{feedbackID}/resolve
GET    /api/sessions/{id}/agent-payload
POST   /api/sessions/{id}/submit-review
POST   /api/sessions/{id}/explanations
GET    /api/sessions/{id}/explanations
GET    /api/sessions/{id}/explanation-requests
```

Project-scoped routes:

```txt
POST   /api/projects
POST   /api/projects/{projectID}/heartbeat
GET    /api/projects/{projectID}/live
GET    /api/projects/{projectID}/sessions
POST   /api/projects/{projectID}/sessions
GET    /api/projects/{projectID}/sessions/{id}
DELETE /api/projects/{projectID}/sessions/{id}
POST   /api/projects/{projectID}/sessions/{id}/explain
POST   /api/projects/{projectID}/sessions/{id}/feedback
DELETE /api/projects/{projectID}/sessions/{id}/feedback/{feedbackID}
PATCH  /api/projects/{projectID}/sessions/{id}/feedback/{feedbackID}/resolve
GET    /api/projects/{projectID}/sessions/{id}/agent-payload
POST   /api/projects/{projectID}/sessions/{id}/submit-review
POST   /api/projects/{projectID}/sessions/{id}/explanations
GET    /api/projects/{projectID}/sessions/{id}/explanations
GET    /api/projects/{projectID}/sessions/{id}/explanation-requests
```

## Context Lines

Git diff context defaults to 3 lines.

`ContextLines` is accepted by server request structs and query parsing for live diff. Values:

- `<= 0` becomes `3`
- `> 200` becomes `200`

## State And Persistence

Server state is in memory:

- projects
- sessions
- comments (with resolved status)
- review events (explanation_requested, explanation_submitted, review_submitted)
- explanations
- explanation requests

Browser state is in `sessionStorage` per project:

- active session
- active file
- scroll position
- drafts
- viewed files

Restarting server loses in-memory state.

Refreshing page keeps browser state and reloads server state if server is still running.

## Static Assets

By default, web UI is embedded from:

```txt
internal/server/web
```

For development, override static root:

```sh
WEB_UI_ROOT=/path/to/web/root ./letsreview /path/to/repo
```

## Safety Boundaries

`letsreview`:

- reads Git metadata/diffs
- does not initialize repos
- does not commit
- does not stage files
- does not run agents directly
- does not call remote AI APIs
- binds localhost unless configured otherwise

Agent behavior happens outside this binary. MCP only passes review requests/results.

## PID File

Server PID file location:

```txt
~/.local/share/letsreview/server.pid
```

Format: one entry per line (`<pid> <addr>`).

```txt
12345 127.0.0.1:55492
12346 127.0.0.1:55492
```

Each `letsreview` server process appends its PID on start. On exit, it removes its own entry. If the file becomes empty, it is deleted.

`letsreview stop` reads all entries, kills live processes, removes stale entries.

## Troubleshooting

### `listen on 127.0.0.1:55492` fails

Another server may already be running. Current CLI tries to join it by registering repo. If join also fails, use different address:

```sh
./letsreview -addr 127.0.0.1:6000 /path/to/repo
```

### Non-Git directory error

Path must already be Git repo:

```sh
git -C /path/to/repo rev-parse --show-toplevel
```

If this fails, fix repo outside `letsreview`.

### No diff shown

Check current mode:

- Live: needs working tree changes vs `HEAD`.
- Staged: needs staged changes.
- Refs: both refs must exist and differ.

### Comments missing after restart

Expected for now. Server state is in memory.

### MCP cannot connect

Make sure address matches:

```sh
./letsreview mcp -addr 127.0.0.1:55492
```

If MCP starts HTTP server itself, it uses same address value.

## Development Checklist

Before changing code:

```sh
git status --short
```

After Go changes:

```sh
gofmt -w <changed-go-files>
GOCACHE=/private/tmp/letsreview-gocache go test ./...
GOCACHE=/private/tmp/letsreview-gocache go vet ./...
```

Before release/build:

```sh
GOCACHE=/private/tmp/letsreview-gocache go build .
```
