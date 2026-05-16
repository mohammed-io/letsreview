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

From repo root:

```sh
go build -o letsreview ./cmd/letsreview
```

Or run without building:

```sh
go run ./cmd/letsreview /path/to/git/repo
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

- `request_code_review`
- `wait_for_review_event`
- `check_review_status`
- `get_review_result`
- `cancel_review`
- `get_explanation_requests`
- `wait_for_explanation_request`
- `submit_explanation`

See [HANDBOOK.md](./HANDBOOK.md) for full usage.

## Development

```sh
GOCACHE=/private/tmp/letsreview-gocache go test ./...
GOCACHE=/private/tmp/letsreview-gocache go vet ./...
GOCACHE=/private/tmp/letsreview-gocache go build ./cmd/letsreview
```
