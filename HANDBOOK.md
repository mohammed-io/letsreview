# letsreview Handbook

`letsreview` is a local code-review assistant. It opens a browser UI for Git diffs, lets a human leave line feedback, then exposes submitted feedback to an AI agent through MCP.

## Mental Model

There are two binaries:

- `letsreview`: CLI + local HTTP server + embedded web UI.
- `letsreview --mcp`: same binary in stdio MCP server mode for agents.

There are two review surfaces:

- **Live**: always shows current working tree vs `HEAD`, refreshed every 2 seconds.
- **Sessions**: snapshots a chosen comparison so review comments attach to stable diff rows.

The app never initializes Git repos. It accepts a path, but diff loading expects that path to already be a Git repository. Non-Git paths fail when Live or Sessions diff data is requested.

## Build

From repo root:

```sh
go build -o letsreview ./cmd/letsreview
```

During sandboxed/local dev, use writable Go cache if needed:

```sh
GOCACHE=/private/tmp/letsreview-gocache go build -o letsreview ./cmd/letsreview
```

## Start UI

```sh
./letsreview /path/to/git/repo
```

If no repo path is supplied, `.` is used:

```sh
./letsreview
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

Open URL in browser.

## CLI Flags

```sh
letsreview [flags] [repo]
```

Available flags:

```txt
-addr string
    Address to listen on. Default: 127.0.0.1:55492

-open
    Prints browser-open hint only. It does not launch browser.
```

Examples:

```sh
./letsreview ~/Projects/email-client
./letsreview -addr 127.0.0.1:6000 ~/Projects/email-client
./letsreview -open .
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
- file comment count appears beside filename
- `Comments` button opens file comment modal

API:

```txt
POST /api/sessions/{id}/feedback
DELETE /api/sessions/{id}/feedback/{feedbackID}
GET /api/sessions/{id}/agent-payload
```

Agent payload is intentionally compact:

```json
{
  "comments": [
    {
      "filePath": "main.go",
      "startLine": 1,
      "endLine": 2,
      "body": "Rename this for clarity.",
      "createdAt": "2026-05-15T..."
    }
  ]
}
```

It does not include full diff, repo path, internal feedback IDs, or session data.

## Submit Review

`Submit review` marks review ready for agent.

Behavior:

- requires at least one comment
- asks for browser confirmation
- stores submitted review in memory
- UI shows submitted status
- comments become agent-readable through MCP

API:

```txt
POST /api/sessions/{id}/submit-review
GET /api/sessions/{id}/review-status
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
./letsreview --mcp
```

Default HTTP address:

```txt
127.0.0.1:55492
```

Override:

```sh
./letsreview --mcp -addr 127.0.0.1:6000
```

MCP protocol is JSON-RPC over stdio. It implements:

- `initialize`
- `ping`
- `tools/list`
- `tools/call`

Protocol version:

```txt
2024-11-05
```

Server info:

```txt
name: letsreview
version: 0.1.0
```

## MCP Tools

### `request_code_review`

Starts HTTP server if needed, registers repo, creates review session, returns browser URL.

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

After this returns, call `wait_for_review_event` to block until the reviewer asks for explanation or submits review.

### `wait_for_review_event`

Long-polls until browser UI produces an event.

Input:

```json
{
  "sessionId": "...",
  "afterSeq": 0,
  "timeoutSeconds": 3600
}
```

Event when reviewer asks for explanation:

```json
{
  "status": "event",
  "event": {
    "seq": 1,
    "type": "explanation_requested",
    "sessionId": "...",
    "explanationRequest": {
      "filePath": "main.go",
      "startLine": 12,
      "endLine": 18
    }
  }
}
```

Event when reviewer submits comments:

```json
{
  "status": "event",
  "event": {
    "seq": 2,
    "type": "review_submitted",
    "sessionId": "...",
    "review": {
      "comments": []
    }
  }
}
```

Timeout:

```json
{
  "status": "timeout",
  "sessionId": "...",
  "afterSeq": 0
}
```

Use returned `seq` as next `afterSeq` to wait for the following event.

### `check_review_status`

Input:

```json
{
  "sessionId": "..."
}
```

Returns:

```json
{
  "status": "pending",
  "sessionId": "..."
}
```

Status becomes `submitted` after human clicks `Submit review`.

### `get_review_result`

Input:

```json
{
  "sessionId": "..."
}
```

Returns submitted review. If not submitted yet, returns tool error text.

### `cancel_review`

Input:

```json
{
  "sessionId": "..."
}
```

Current behavior returns cancelled status. It does not remove server state.

### `get_explanation_requests`

Input:

```json
{
  "sessionId": "..."
}
```

Returns pending/resolved explanation requests created by browser UI.

### `wait_for_explanation_request`

Long-polls until reviewer asks for explanation.

Input:

```json
{
  "sessionId": "...",
  "afterSeq": 0,
  "timeoutSeconds": 3600
}
```

Returns:

```json
{
  "status": "explanation_requested",
  "seq": 3,
  "sessionId": "...",
  "projectID": "...",
  "explanationRequest": {
    "id": "...",
    "filePath": "main.go",
    "startLine": 12,
    "endLine": 18,
    "resolved": false
  }
}
```

If no explanation request arrives before timeout:

```json
{
  "status": "timeout",
  "sessionId": "...",
  "afterSeq": 3
}
```

Use this when agent should pause until human clicks `Explain selection`.

### `submit_explanation`

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

Stores explanation for browser UI.

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
GET    /api/sessions/{id}/agent-payload
POST   /api/sessions/{id}/submit-review
GET    /api/sessions/{id}/review-status
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
GET    /api/projects/{projectID}/sessions/{id}/agent-payload
POST   /api/projects/{projectID}/sessions/{id}/submit-review
GET    /api/projects/{projectID}/sessions/{id}/review-status
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
- comments
- submitted reviews
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
./letsreview --mcp -addr 127.0.0.1:55492
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
GOCACHE=/private/tmp/letsreview-gocache go build ./cmd/letsreview
```
