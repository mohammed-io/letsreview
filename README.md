# letsreview

Local web UI for reviewing Git diffs, collecting line feedback, and handing that feedback back to an AI agent through MCP.

`letsreview` is intentionally local-first:

- no repo initialization
- no cloud service
- no background database
- no network AI calls by default
- one Go binary for CLI/web UI/MCP

## Status

Early local tool. Core flows exist:

- live working-tree diff
- snapshot review sessions
- multiple repos in one running server
- canvas diff rendering
- line comments
- viewed files
- submit review for agent pickup
- MCP review request/result tools
- MCP explanation request/response tools

## Install

With Go (installs latest from GitHub):

```sh
go install github.com/mohammed-io/letsreview/cmd/letsreview@latest
```

This places `letsreview` in `$GOPATH/bin` (or `$HOME/go/bin`). Make sure it's on your `PATH`.

Or build from source:

```sh
git clone https://github.com/mohammed-io/letsreview.git
cd letsreview
go build -o letsreview ./cmd/letsreview
```

## Run

```sh
./letsreview /path/to/git/repo
```

Default address:

```txt
127.0.0.1:55492
```

Output looks like:

```txt
letsreview is running at http://127.0.0.1:55492?project=<project-id>
reviewing /path/to/git/repo
```

Open URL in browser.

![letsreview screenshot](./screenshot.png)

## CLI

```sh
letsreview [flags] [repo]
```

Flags:

```txt
-addr string   address to listen on (default "127.0.0.1:55492")
-open          print browser-open hint only; browser launch is left to caller
```

`repo` defaults to current directory.

## MCP

Run MCP server:

```sh
./letsreview --mcp
```

Optional address override:

```sh
./letsreview --mcp -addr 127.0.0.1:55492
```

Main MCP tools:

- `request_code_review` — start review session, open browser
- `get_pending_events` — non-blocking poll for new events (review_submitted, explanation_requested, explanation_submitted)
- `get_review_result` — get latest submitted review comments
- `cancel_review` — cancel a review session
- `submit_explanation` — respond to an explanation request

See [HANDBOOK.md](./HANDBOOK.md) for full usage.

## Configure MCP Clients

Build and place the binary somewhere on your PATH:

```sh
go build -o letsreview ./cmd/letsreview
mv letsreview /usr/local/bin/
```

### Claude Code

Add to your project's `.mcp.json`:

```json
{
  "mcpServers": {
    "letsreview": {
      "command": "letsreview",
      "args": ["--mcp"]
    }
  }
}
```

Or globally in `~/.claude/mcp.json`.

### Codex (OpenAI)

Add to `~/.codex/mcp.json`:

```json
{
  "mcpServers": {
    "letsreview": {
      "command": "letsreview",
      "args": ["--mcp"]
    }
  }
}
```

### OpenCode

Add to your project's `opencode.json`:

```json
{
  "mcp": {
    "letsreview": {
      "command": "letsreview",
      "args": ["--mcp"]
    }
  }
}
```

All clients communicate via stdio JSON-RPC. No API keys needed — everything runs locally.

## Development

```sh
GOCACHE=/private/tmp/letsreview-gocache go test ./...
GOCACHE=/private/tmp/letsreview-gocache go vet ./...
GOCACHE=/private/tmp/letsreview-gocache go build ./cmd/letsreview
```
