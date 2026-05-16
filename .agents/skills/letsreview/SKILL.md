---
name: letsreview
description: >
  Start an interactive code review session with the human via letsreview MCP server.
  Use when user says "/letsreview", "review my code", "start review", or asks for
  human-in-the-loop code review. Also triggers when user wants feedback on changes
  before applying them.
---

Start a code review session with the human reviewer.

## Error Recovery

If `request_code_review` fails with "tool not found" or unknown tool error:
- Tell the user to install letsreview: `go install github.com/mohammed-io/letsreview/cmd/letsreview@latest`
- Tell the user to add MCP config (`.mcp.json` for Claude Code, `~/.codex/mcp.json` for Codex, `opencode.json` for OpenCode):
  ```json
  {
    "mcpServers": {
      "letsreview": {
        "command": "letsreview",
        "args": ["mcp"]
      }
    }
  }
  ```
- Tell the user to restart their agent session after adding the config
- Stop — do not retry until the user confirms it's set up

## Flow

1. Call `request_code_review` with the repo path and diff mode (default: "working")
2. Tell the user the review session is open and a browser launched
3. Enter the event polling loop:
   - Call `get_pending_events` every few seconds with the session ID and last event seq
   - On `explanation_requested`: read the explanation request, understand the code context, call `submit_explanation` with a clear answer
   - On `review_submitted`: call `get_review_result` to get all comments, then APPLY each comment as a code change, then call `resolve_feedback` for each processed comment
   - Update `afterSeq` from `lastSeq` after each poll
4. After applying review feedback, ask the human if they want another review session to verify the changes
5. If they agree, go back to step 1

## Rules

- ALWAYS apply changes after receiving `review_submitted` feedback — do not just acknowledge, actually edit the files
- After applying each comment's change, call `resolve_feedback` with the comment's `id` to mark it resolved in the web UI
- After applying all changes, ask the human if they want another review session to verify your fixes
- When submitting explanations, be concise and specific to the lines asked about
- Use `get_pending_events` — it never blocks, call it between other work
- Do not ask the user to manually check the review — poll for events yourself

## Example

```
User: /letsreview

Assistant:
1. Checks prerequisites (binary installed, MCP configured)
2. Calls request_code_review({ repoPath: "/path/to/repo", mode: "working" })
3. Gets back sessionId, url, lastEventSeq
4. Tells user: "Review session started. Browser opened at <url>. I'll watch for your feedback."
5. Polls get_pending_events every few seconds
6. When review_submitted arrives with comments like:
   - "main.go:15-20: This function should handle nil input"
   - "auth.go:42: Rename this variable for clarity"
 7. Applies each change to the code
 8. Calls `resolve_feedback` for each applied comment
 9. Asks the human if they want to review the changes
```
