Start a code review session with the human via the letsreview MCP server.

## Steps

1. Call `request_code_review` with the current repo path and mode "working"
2. Tell the user the browser opened and you're watching for feedback
3. Poll `get_pending_events` every few seconds using the sessionId and lastEventSeq
4. On `explanation_requested`: call `submit_explanation` with a clear, concise answer about the code
5. On `review_submitted`: call `get_review_result`, then APPLY every comment as an actual code change, then call `resolve_feedback` for each processed comment using its `id`
6. After applying all changes, ask the human if they want another review session to verify your fixes
7. If they agree, go back to step 1

## Rules

- ALWAYS apply code changes from review feedback — do not just acknowledge comments
- After applying each comment's fix, call `resolve_feedback` with the comment `id` to mark it resolved in the web UI
- After applying changes, ask the human if they want another review session for verification
- `get_pending_events` never blocks — call it between other work
- Be concise in explanations
