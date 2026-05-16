# mcp_explain_lines

## Status: completed (20260515124722)

## Context
User wants web UI → agent explanation loop. User selects lines, clicks "Explain selection". Web UI posts request. Agent picks up via MCP, responds. Web UI polls and shows explanation inline on diff.

## Flow
1. User selects lines → clicks "Explain selection"
2. Web UI POSTs `/api/sessions/{id}/explain` → stores `ExplanationRequest` on server
3. Agent polls MCP `get_explanation_requests(sessionId)` → sees pending requests
4. Agent calls MCP `submit_explanation(sessionId, filePath, startLine, endLine, body)` → stores `Explanation`
5. Web UI polls GET `/api/sessions/{id}/explanations` → renders inline on diff

## Todos
- [ ] Add `ExplanationRequest` + `Explanation` types, storage in Store/Project
- [ ] Update `POST /explain` to store request (instead of local-only)
- [ ] Add `GET /explanations` endpoint for web UI
- [ ] Add `GET /explanation-requests` endpoint for web UI status
- [ ] MCP tools: `get_explanation_requests`, `submit_explanation`
- [ ] UI: explain button stores request, poll for explanation, render inline
- [ ] Tests

## Acceptance Criteria
- Explain button posts request, UI shows "awaiting explanation"
- Agent sees request via MCP, responds with explanation
- Web UI shows explanation inline on diff (distinct color from comments)
