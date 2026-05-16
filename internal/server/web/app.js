const state = {
  projectID: new URLSearchParams(window.location.search).get("project"),
  sessions: [],
  activeSession: null,
  activeFile: null,
  codeLineCount: 0,
  flatLines: [],
  selected: { start: 0, end: 0 },
  liveData: null,
  pollInterval: null,
  creatingSession: null,
  inlineReviewOpen: false,
  pendingReviewOpen: false,
  viewedFiles: new Set(),
  focusedCommentIndex: -1,
  explanations: [],
  explanationPollInterval: null,
  motionCount: "",
  focusZone: "diff",
};

const els = {
  repo: document.querySelector("#repo-label"),
  title: document.querySelector("#session-title"),
  summary: document.querySelector("#session-summary"),
  statFiles: document.querySelector("#stat-files"),
  statAdd: document.querySelector("#stat-add"),
  statDel: document.querySelector("#stat-del"),
  files: document.querySelector("#files"),
  activeFile: document.querySelector("#active-file"),
  diffArea: document.querySelector(".diff-area"),
  canvas: document.querySelector("#diff-canvas"),
  explain: document.querySelector("#explain-selection"),
  explanation: document.querySelector("#explanation"),
  feedback: document.querySelector("#feedback"),
  closeFeedback: document.querySelector("#close-feedback"),
  inlineReview: document.querySelector("#inline-review"),
  inlineReviewTitle: document.querySelector("#inline-review-title"),
  commentList: document.querySelector("#comment-list"),
  saveFeedback: document.querySelector("#save-feedback"),
  commentsModal: document.querySelector("#comments-modal"),
  closeCommentsModal: document.querySelector("#close-comments-modal"),
  fileCommentList: document.querySelector("#file-comment-list"),
  clearSession: document.querySelector("#clear-session"),
  agentPayloadModal: document.querySelector("#agent-payload-modal"),
  closeAgentPayload: document.querySelector("#close-agent-payload"),
  payload: document.querySelector("#agent-payload"),
  viewedFile: document.querySelector("#viewed-file"),
  metricComments: document.querySelector("#metric-comments"),
  metricDrafts: document.querySelector("#metric-drafts"),
  metricViewed: document.querySelector("#metric-viewed"),
  panelAddComment: document.querySelector("#panel-add-comment"),
  panelSubmitReview: document.querySelector("#panel-submit-review"),
  reviewCommentList: document.querySelector("#review-comment-list"),
  reviewSection: document.querySelector(".review-section.grow"),
  showShortcuts: document.querySelector("#show-shortcuts"),
  shortcutsModal: document.querySelector("#shortcuts-modal"),
  closeShortcuts: document.querySelector("#close-shortcuts"),
};

{
  const shortcut = els.saveFeedback.querySelector(".shortcut");
  const mod = (navigator.platform.includes("Mac") ? "⌘" : "Ctrl") + " + Enter";
  if (shortcut) shortcut.textContent = mod;
  document.querySelectorAll(".mod-enter").forEach((node) => { node.textContent = mod; });
}

const ctx = els.canvas.getContext("2d");
const canvasTheme = {
  bg: "#0d1117",
  border: "#30363d",
  gutter: "#161b22",
  gutterText: "#6e7681",
  text: "#c9d1d9",
  muted: "#8b949e",
  hunkBg: "#0d419d26",
  hunkText: "#58a6ff",
  addBg: "#033a16",
  addGutter: "#0f5323",
  addText: "#aff5b4",
  delBg: "#67060c",
  delGutter: "#78191b",
  delText: "#ffdcd7",
  selected: "#f2cc6033",
  selectedBorder: "#f2cc60",
  string: "#a5d6ff",
  keyword: "#ff7b72",
  number: "#79c0ff",
  type: "#d2a8ff",
};
const rowHeight = 21;
const gutterWidth = 98;
const codePadding = 18;
let scrollY = 0;
let commentMarkers = [];

const storageKey = (name) => `letsreview:${state.projectID || "default"}:${name}`;

function readStoredState() {
  try {
    return JSON.parse(sessionStorage.getItem(storageKey("state")) || "{}");
  } catch {
    return {};
  }
}

function writeStoredState(values = {}) {
  const current = readStoredState();
  sessionStorage.setItem(storageKey("state"), JSON.stringify({ ...current, ...values }));
}

function draftKey() {
  const range = selectedLineRange() || { start: 0, end: 0 };
  const start = Math.min(range.start, range.end);
  const end = Math.max(range.start, range.end);
  return storageKey(`draft:${state.activeFile?.path || "none"}:${start}:${end}`);
}

function clearProjectStorage() {
  Object.keys(sessionStorage)
    .filter((key) => key.startsWith(`letsreview:${state.projectID || "default"}:`))
    .forEach((key) => sessionStorage.removeItem(key));
}

async function api(path, options = {}) {
  const response = await fetch(scopedApiPath(path), {
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options,
  });
  const body = await response.json();
  if (!response.ok) throw new Error(body.error || "Request failed");
  return body;
}

function scopedApiPath(path) {
  if (!state.projectID || !path.startsWith("/api/")) return path;
  return path.replace("/api", `/api/projects/${encodeURIComponent(state.projectID)}`);
}

function tokenColor(token) {
  if (/^(func|const|var|type|return|if|else|case|switch|for|range|package|import)\b/.test(token)) return canvasTheme.keyword;
  if (/^["'`]/.test(token)) return canvasTheme.string;
  if (/^\d+$/.test(token)) return canvasTheme.number;
  if (/^(true|false|nil|null|undefined)$/.test(token)) return canvasTheme.number;
  if (/^[A-Z][A-Za-z0-9_]+$/.test(token)) return canvasTheme.type;
  return canvasTheme.text;
}

function resizeCanvas() {
  const rect = els.canvas.getBoundingClientRect();
  const scale = window.devicePixelRatio || 1;
  els.canvas.width = Math.max(1, Math.floor(rect.width * scale));
  els.canvas.height = Math.max(1, Math.floor(rect.height * scale));
  ctx.setTransform(scale, 0, 0, scale, 0, 0);
  renderDiff();
}

function flatten(file) {
  return file.hunks.flatMap((hunk) => [
    { kind: "hunk", text: hunk.header, oldNumber: "", newNumber: "" },
    ...hunk.lines,
  ]);
}

function renderDiff() {
  const rect = els.canvas.getBoundingClientRect();
  commentMarkers = [];
  ctx.save();
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, els.canvas.width, els.canvas.height);
  ctx.restore();
  ctx.clearRect(0, 0, rect.width, rect.height);
  ctx.fillStyle = canvasTheme.bg;
  ctx.fillRect(0, 0, rect.width, rect.height);
  ctx.font = "13px ui-monospace, SFMono-Regular, Menlo, monospace";
  ctx.textBaseline = "middle";

  const first = Math.max(0, Math.floor(scrollY / rowHeight));
  const visible = Math.ceil(rect.height / rowHeight) + 2;
  const rows = state.flatLines.slice(first, first + visible);
  rows.forEach((line, offset) => {
    const index = first + offset + 1;
    const y = (index - 1) * rowHeight - scrollY;
    drawRow(line, index, y, rect.width);
    drawCommentMarker(index, y, rect.width);
    drawExplanationBlock(index, y, rect.width);
  });

  ctx.fillStyle = canvasTheme.border;
  ctx.fillRect(gutterWidth - 1, 0, 1, rect.height);

  if (state.activeFile && isFileViewed(state.activeFile)) {
    ctx.fillStyle = canvasTheme.muted;
    ctx.textAlign = "center";
    ctx.fillText("Viewed", rect.width / 2, Math.max(48, rect.height / 3));
    ctx.textAlign = "left";
  }
}

function commentsForActiveFile() {
  return (state.activeSession?.feedback || []).filter((comment) => comment.filePath === state.activeFile?.path);
}

function explanationsForActiveFile() {
  return (state.explanations || []).filter((e) => e.filePath === state.activeFile?.path);
}

function commentsForRange(start, end) {
  const low = Math.min(start, end);
  const high = Math.max(start, end);
  return commentsForActiveFile().filter((comment) => {
    const commentLow = Math.min(comment.startLine, comment.endLine);
    const commentHigh = Math.max(comment.startLine, comment.endLine);
    return commentLow <= high && commentHigh >= low;
  });
}

function lineNumberForRow(row) {
  const line = state.flatLines[row - 1];
  if (!line || line.kind === "hunk") return 0;
  return line.newNumber || line.oldNumber || 0;
}

function selectedRows() {
  if (!state.selected.start || !state.selected.end) return [];
  const start = Math.min(state.selected.start, state.selected.end);
  const end = Math.max(state.selected.start, state.selected.end);
  return state.flatLines.slice(start - 1, end).map((_, offset) => start + offset);
}

function selectedLineRange() {
  const numbers = selectedRows().map(lineNumberForRow).filter(Boolean);
  if (numbers.length === 0) return null;
  return { start: Math.min(...numbers), end: Math.max(...numbers) };
}

function rowsForLineRange(startLine, endLine) {
  const low = Math.min(startLine, endLine);
  const high = Math.max(startLine, endLine);
  const rows = state.flatLines
    .map((_, index) => ({ row: index + 1, lineNumber: lineNumberForRow(index + 1) }))
    .filter((item) => item.lineNumber >= low && item.lineNumber <= high);
  if (rows.length === 0) return null;
  return { start: rows[0].row, end: rows[rows.length - 1].row };
}

function rangeLabel(range, prefix = "Line") {
  if (!range) return `${prefix} ?`;
  const start = Math.min(range.start, range.end);
  const end = Math.max(range.start, range.end);
  return start === end ? `${prefix} ${start}` : `${prefix}s ${start}-${end}`;
}

function markerGroups() {
  return Object.values(commentsForActiveFile().reduce((groups, comment) => {
    const start = Math.min(comment.startLine, comment.endLine);
    const end = Math.max(comment.startLine, comment.endLine);
    const key = `${start}:${end}`;
    const rows = rowsForLineRange(start, end);
    return {
      ...groups,
      [key]: {
        start,
        end,
        rowStart: rows?.start || 0,
        rowEnd: rows?.end || 0,
        count: (groups[key]?.count || 0) + 1,
      },
    };
  }, {})).filter((group) => group.rowStart);
}

function commentCountForFile(path) {
  return (state.activeSession?.feedback || []).filter((comment) => comment.filePath === path).length;
}

function totalCommentCount() {
  return state.activeSession?.feedback?.length || 0;
}

function allFiles() {
  return state.liveData?.files || [];
}

function draftCount() {
  const prefix = storageKey("draft:");
  return Object.keys(sessionStorage).filter((key) => key.startsWith(prefix) && sessionStorage.getItem(key)?.trim()).length;
}

function viewedCount(files = allFiles()) {
  return files.filter((file) => isFileViewed(file)).length;
}

function fileViewedKey(file) {
  return `${state.activeSession?.id || "live"}:${file?.path || ""}`;
}

function isFileViewed(file) {
  return state.viewedFiles.has(fileViewedKey(file));
}

function setActiveFile(file) {
  state.activeFile = file;
  state.selected = { start: 0, end: 0 };
  state.inlineReviewOpen = false;
  scrollY = readStoredState().scrollByFile?.[file.path] || 0;
  writeStoredState({ activeFilePath: file.path });
  renderAll();
}

function drawCommentMarker(row, y, width) {
  const group = markerGroups().find((item) => item.rowStart === row);
  if (!group) return;

  const label = String(group.count);
  ctx.save();
  ctx.font = "700 12px ui-monospace, SFMono-Regular, Menlo, monospace";
  const markerWidth = Math.max(30, ctx.measureText(label).width + 18);
  const markerHeight = 20;
  const x = 8;
  const markerY = y + Math.floor((rowHeight - markerHeight) / 2);
  ctx.fillStyle = "#1f6feb";
  roundRect(x, markerY, markerWidth, markerHeight, 10);
  ctx.fill();
  ctx.strokeStyle = "#79c0ff";
  ctx.lineWidth = 1;
  roundRect(x + 0.5, markerY + 0.5, markerWidth - 1, markerHeight - 1, 10);
  ctx.stroke();
  ctx.fillStyle = "#ffffff";
  ctx.textAlign = "center";
  ctx.fillText(label, x + markerWidth / 2, y + rowHeight / 2);
  ctx.restore();
  commentMarkers.push({ ...group, x, y: markerY, width: markerWidth, height: markerHeight });
}

function drawExplanationBlock(row, y, width) {
  const explanation = explanationsForActiveFile().find((e) => rowsForLineRange(e.startLine, e.endLine)?.end === row);
  if (!explanation) return;

  const label = "E";
  const markerWidth = 20;
  const markerHeight = 16;
  const commentGroup = markerGroups().find((item) => item.rowStart === row);
  const markerOffset = commentGroup ? 26 : 0;
  const mx = width - markerWidth - 12 - markerOffset;
  const markerY = y + Math.floor((rowHeight - markerHeight) / 2);
  ctx.fillStyle = "#8b5cf6";
  roundRect(mx, markerY, markerWidth, markerHeight, 8);
  ctx.fill();
  ctx.fillStyle = "#ffffff";
  ctx.textAlign = "center";
  ctx.fillText(label, mx + markerWidth / 2, y + rowHeight / 2);
  ctx.textAlign = "left";
  const expRows = rowsForLineRange(explanation.startLine, explanation.endLine);
  commentMarkers.push({ start: explanation.startLine, end: explanation.endLine, rowStart: expRows?.start || row, rowEnd: expRows?.end || row, x: mx, y: markerY, width: markerWidth, height: markerHeight, explanation: true });

  const blockY = y + rowHeight;
  const pad = 8;
  const lh = 18;
  const maxW = width - gutterWidth - codePadding * 2 - pad * 2;
  const bodyLines = wrapText(explanation.body, maxW);
  const blockH = bodyLines.length * lh + pad * 2 + lh;

  ctx.fillStyle = "#1a1033";
  roundRect(gutterWidth, blockY, width - gutterWidth, blockH, 4);
  ctx.fill();
  ctx.strokeStyle = "#8b5cf644";
  ctx.lineWidth = 1;
  roundRect(gutterWidth, blockY, width - gutterWidth, blockH, 4);
  ctx.stroke();
  ctx.lineWidth = 1;

  ctx.textAlign = "left";
  let ty = blockY + pad + lh / 2;
  ctx.fillStyle = "#8b5cf6";
  ctx.fillText("AI Explainer:", gutterWidth + pad, ty);
  ty += lh;
  ctx.fillStyle = "#c9d1d9";
  for (const ln of bodyLines) {
    ctx.fillText(ln, gutterWidth + pad, ty);
    ty += lh;
  }
}

function wrapText(text, maxWidth) {
  const words = text.split(/\s+/);
  const lines = [];
  let current = "";
  for (const word of words) {
    const test = current ? `${current} ${word}` : word;
    if (ctx.measureText(test).width > maxWidth && current) {
      lines.push(current);
      current = word;
    } else {
      current = test;
    }
  }
  if (current) lines.push(current);
  return lines.length ? lines : [text];
}

function roundRect(x, y, width, height, radius) {
  ctx.beginPath();
  ctx.moveTo(x + radius, y);
  ctx.arcTo(x + width, y, x + width, y + height, radius);
  ctx.arcTo(x + width, y + height, x, y + height, radius);
  ctx.arcTo(x, y + height, x, y, radius);
  ctx.arcTo(x, y, x + width, y, radius);
  ctx.closePath();
}

const canvasBottomPadding = 20;

function drawRow(line, index, y, width) {
  if (line.kind === "padding") {
    ctx.fillStyle = canvasTheme.bg;
    ctx.fillRect(0, y, width, rowHeight);
    return;
  }
  const selected = state.selected.start && index >= Math.min(state.selected.start, state.selected.end) && index <= Math.max(state.selected.start, state.selected.end);
  const bg = line.kind === "add" ? canvasTheme.addBg : line.kind === "del" ? canvasTheme.delBg : line.kind === "hunk" ? canvasTheme.hunkBg : canvasTheme.bg;
  const gutterBg = line.kind === "add" ? canvasTheme.addGutter : line.kind === "del" ? canvasTheme.delGutter : line.kind === "hunk" ? canvasTheme.hunkBg : canvasTheme.gutter;
  ctx.fillStyle = bg;
  ctx.fillRect(0, y, width, rowHeight);
  ctx.fillStyle = gutterBg;
  ctx.fillRect(0, y, gutterWidth, rowHeight);

  if (selected) {
    ctx.fillStyle = canvasTheme.selected;
    ctx.fillRect(0, y, width, rowHeight);
    ctx.fillStyle = canvasTheme.selectedBorder;
    ctx.fillRect(0, y, 3, rowHeight);
  }

  const marker = line.kind === "add" ? "+" : line.kind === "del" ? "-" : " ";
  ctx.fillStyle = line.kind === "add" ? canvasTheme.addText : line.kind === "del" ? canvasTheme.delText : line.kind === "hunk" ? canvasTheme.hunkText : canvasTheme.muted;
  ctx.fillText(marker, gutterWidth + 6, y + rowHeight / 2);

  ctx.fillStyle = line.kind === "hunk" ? canvasTheme.hunkText : canvasTheme.gutterText;
  ctx.textAlign = "right";
  ctx.fillText(String(line.oldNumber || ""), 38, y + rowHeight / 2);
  ctx.fillText(String(line.newNumber || ""), 78, y + rowHeight / 2);
  ctx.textAlign = "left";

  drawHighlightedText(line.text || "", gutterWidth + codePadding, y + rowHeight / 2, line.kind);
}

function drawHighlightedText(text, x, y, kind) {
  const tokens = text.split(/(\s+|[()[\]{}.,;:+\-*/=<>!]+)/);
  tokens.reduce((cursor, token) => {
    ctx.fillStyle = kind === "hunk" ? canvasTheme.hunkText : tokenColor(token);
    ctx.fillText(token, cursor, y);
    return cursor + ctx.measureText(token).width;
  }, x);
}

function rowFromEvent(event) {
  const rect = els.canvas.getBoundingClientRect();
  return clampToCodeLines(Math.floor((event.clientY - rect.top + scrollY) / rowHeight) + 1);
}

function selectSession(session) {
  state.activeSession = session;
  const stored = readStoredState();
  state.activeFile = session.files.find((file) => file.path === stored.activeFilePath) || session.files[0] || null;
  state.selected = { start: 0, end: 0 };
  state.inlineReviewOpen = false;
  writeStoredState({ activeSessionID: session.id, activeFilePath: state.activeFile?.path || "" });
  scrollY = state.activeFile ? stored.scrollByFile?.[state.activeFile.path] || 0 : 0;
  renderAll();
}

function selectFile(file) {
  setActiveFile(file);
}

function renderFocusZone() {
  els.diffArea.classList.toggle("keyboard-focus", state.focusZone === "diff");
  els.reviewSection.classList.toggle("keyboard-focus", state.focusZone === "queue");
}

function setFocusZone(zone, options = {}) {
  state.focusZone = zone;
  renderFocusZone();
  if (options.focus) {
    (zone === "queue" ? els.reviewSection : els.diffArea).focus({ preventScroll: true });
  }
}

function toggleFocusZone() {
  setFocusZone(state.focusZone === "diff" ? "queue" : "diff", { focus: true });
}

function renderAll() {
  const data = state.liveData;
  els.title.textContent = data ? "Live: Working Tree" : "Loading...";
  if (els.summary) els.summary.textContent = data ? data.summary : "Fetching diff...";
  els.statFiles.textContent = `${data?.stats?.files || 0} files`;
  els.statAdd.textContent = `+${data?.stats?.additions || 0}`;
  els.statDel.textContent = `-${data?.stats?.deletions || 0}`;
  els.repo.textContent = data?.meta?.repo || "local repo";

  const files = data?.files || [];
  els.files.replaceChildren(...files.map((file) => {
    const button = document.createElement("button");
    button.className = `file ${state.activeFile?.path === file.path ? "active" : ""} ${isFileViewed(file) ? "viewed" : ""}`;
    const icon = file.status === "added" ? "A" : file.status === "deleted" ? "D" : file.status === "renamed" ? "R" : "M";
    const comments = commentCountForFile(file.path);
    button.innerHTML = `<span class="file-status">${isFileViewed(file) ? "✓" : icon}</span><strong>${escapeHtml(file.path)}${comments ? ` <em>(${comments})</em>` : ""}</strong><small>+${file.additions} -${file.deletions}</small>`;
    button.addEventListener("click", () => setActiveFile(file));
    return button;
  }));
  const codeLines = state.activeFile && !isFileViewed(state.activeFile) ? flatten(state.activeFile) : [];
  state.flatLines = codeLines.length
    ? [...codeLines, ...Array.from({ length: canvasBottomPadding }, () => ({ kind: "padding" }))]
    : [];
  state.codeLineCount = codeLines.length;
  els.activeFile.textContent = state.activeFile ? state.activeFile.path : "No file selected";
  els.viewedFile.checked = Boolean(state.activeFile && isFileViewed(state.activeFile));
  els.viewedFile.disabled = !state.activeFile;
  const hasSelection = Boolean(selectedLineRange());
  const canReviewSelection = Boolean(state.activeFile && hasSelection);
  els.explain.disabled = !canReviewSelection;
  els.saveFeedback.disabled = !canReviewSelection;
  els.clearSession.disabled = !state.activeSession && !sessionStorage.getItem(storageKey("state"));
  renderInlineReview();
  renderReviewPanel();
  renderFocusZone();
  renderDiff();
}

function renderInlineReview() {
  const selectedRange = selectedLineRange();
  const hasSelection = Boolean(selectedRange);
  const canComment = Boolean(state.inlineReviewOpen && state.activeFile && hasSelection);
  els.inlineReview.classList.toggle("visible", canComment);
  if (!canComment) return;

  const start = Math.min(state.selected.start, state.selected.end);
  const end = Math.max(state.selected.start, state.selected.end);
  const stage = els.canvas.getBoundingClientRect();
  const selTop = (start - 1) * rowHeight - scrollY;
  const selBottom = end * rowHeight - scrollY;
  const anchorY = selBottom + 6;
  if (selBottom < 0 || selTop > stage.height) {
    els.inlineReview.classList.remove("visible");
    return;
  }
  els.inlineReview.style.top = `${anchorY}px`;
  els.inlineReviewTitle.textContent = `Comment on ${rangeLabel(selectedRange, "line")}`;
  renderCommentList(selectedRange.start, selectedRange.end);
  const exp = explanationsForActiveFile().find((e) => e.startLine === selectedRange.start && e.endLine === selectedRange.end);
  els.explanation.textContent = exp?.body || "Select diff rows, then ask for an explanation.";
  els.feedback.value = sessionStorage.getItem(draftKey()) || "";
}

function clampInlineReview() {
  if (!state.inlineReviewOpen || !state.selected.start) return;
  const start = Math.min(state.selected.start, state.selected.end);
  const end = Math.max(state.selected.start, state.selected.end);
  const stage = els.canvas.getBoundingClientRect();
  const selTop = (start - 1) * rowHeight - scrollY;
  const selBottom = end * rowHeight - scrollY;
  const anchorY = selBottom + 6;
  if (selBottom < 0 || selTop > stage.height) {
    els.inlineReview.classList.remove("visible");
    return;
  }
  els.inlineReview.style.top = `${anchorY}px`;
  els.inlineReview.classList.add("visible");
}

function morphReviewComments(nodes) {
  if (window.Idiomorph?.morph) {
    window.Idiomorph.morph(els.reviewCommentList, nodes.map((node) => node.outerHTML).join(""), { morphStyle: "innerHTML" });
    return;
  }
  els.reviewCommentList.replaceChildren(...nodes);
}

function renderCommentList(start, end) {
  const comments = commentsForRange(start, end);
  els.commentList.replaceChildren(...commentNodes(comments));
}

function commentNodes(comments, options = {}) {
  return comments.map((comment, index) => {
    const item = document.createElement("article");
    const resolvedClass = comment.resolved ? " resolved" : "";
    item.className = `comment-item${resolvedClass} ${index === state.focusedCommentIndex ? "focused" : ""}`;
    item.dataset.feedbackId = comment.id;
    const range = rangeLabel({ start: comment.startLine, end: comment.endLine });
    const created = new Date(comment.createdAt).toLocaleString();
    const resolvedBadge = comment.resolved
      ? `<span class="resolved-badge">Resolved</span>`
      : "";
    item.innerHTML = options.legacy ? `
      <div class="comment-meta">
        <div>
          <strong>${escapeHtml(range)}</strong>
          <span>${escapeHtml(created)}</span>
          ${resolvedBadge}
        </div>
        <button class="secondary delete-comment" data-feedback-id="${escapeHtml(comment.id)}">Delete</button>
      </div>
      <p>${escapeHtml(comment.body)}</p>
    ` : `
      <div class="comment-meta">
        <strong>${escapeHtml(range)}</strong>
        <div class="comment-meta-actions">
          ${resolvedBadge}
          <button class="delete-comment icon-delete" data-feedback-id="${escapeHtml(comment.id)}" aria-label="Delete comment">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
            </svg>
          </button>
        </div>
      </div>
      <p>${escapeHtml(comment.body)}</p>
    `;
    return item;
  });
}

function renderReviewPanel() {
  const files = allFiles();
  const comments = state.activeSession?.feedback || [];
  const selectedRange = selectedLineRange();
  const unresolvedCount = comments.filter(c => !c.resolved).length;
  els.metricComments.textContent = String(comments.length);
  els.metricDrafts.textContent = String(draftCount());
  els.metricViewed.textContent = `${viewedCount(files)}/${files.length}`;
  els.panelAddComment.disabled = !state.activeFile || !selectedRange || state.reviewSubmitted;
  els.panelSubmitReview.disabled = state.reviewSubmitted || unresolvedCount === 0;
  morphReviewComments(commentNodes(comments));
}

function focusComment(comment, options = {}) {
  const file = allFiles().find((item) => item.path === comment.filePath);
  if (!file) return;
  setActiveFile(file);
  const rows = rowsForLineRange(comment.startLine, comment.endLine);
  if (rows) {
    state.selected = { start: rows.start, end: rows.end };
    scrollY = Math.max(0, (rows.start - 4) * rowHeight);
    state.inlineReviewOpen = true;
  }
  renderAll();
  if (options.focusInput) els.feedback.focus();
}


async function saveFeedback() {
  const range = selectedLineRange();
  if (!state.activeFile || !range || !els.feedback.value.trim() || state.reviewSubmitted) return;
  const filePath = state.activeFile.path;
  const session = await ensureReviewSession();
  await api(`/api/sessions/${session.id}/feedback`, {
    method: "POST",
    body: JSON.stringify({ filePath, startLine: range.start, endLine: range.end, body: els.feedback.value }),
  });
  const selected = { ...state.selected };
  const savedScrollY = scrollY;
  sessionStorage.removeItem(draftKey());
  els.feedback.value = "";
  state.inlineReviewOpen = false;
  await refreshSessions();
  state.selected = selected;
  scrollY = savedScrollY;
  renderAll();
}

async function submitReview() {
  if (!state.activeSession || totalCommentCount() === 0) return;
  if (!window.confirm("Submit this review? The agent will receive all comments and may start making changes.")) return;
  await api(`/api/sessions/${state.activeSession.id}/submit-review`, { method: "POST" });
  renderAll();
}

async function deleteComment(feedbackID) {
  if (!feedbackID || !state.activeSession) return;
  await api(`/api/sessions/${state.activeSession.id}/feedback/${feedbackID}`, { method: "DELETE" });
  await refreshSessions();
  if (els.commentsModal.classList.contains("visible")) {
    const comments = commentsForActiveFile();
    els.fileCommentList.replaceChildren(...commentNodes(comments));
    els.commentsModal.classList.toggle("visible", comments.length > 0);
  }
}

async function ensureReviewSession() {
  if (state.activeSession) return state.activeSession;
  const storedSessionID = readStoredState().activeSessionID;
  if (storedSessionID) {
    await refreshSessions();
    const restored = state.sessions.find((session) => session.id === storedSessionID);
    if (restored) {
      state.activeSession = restored;
      return restored;
    }
  }
  if (!state.creatingSession) {
    state.creatingSession = api("/api/sessions", {
      method: "POST",
      body: JSON.stringify({ mode: "working" }),
    }).finally(() => {
      state.creatingSession = null;
    });
  }
  const session = await state.creatingSession;
  state.sessions = [session, ...state.sessions.filter((item) => item.id !== session.id)];
  state.activeSession = session;
  writeStoredState({ activeSessionID: session.id });
  return session;
}

function openInlineReviewForSelection(options = {}) {
  if (!state.activeFile || !selectedLineRange() || state.reviewSubmitted) return;
  state.inlineReviewOpen = true;
  renderAll();
  if (options.focusInput) els.feedback.focus();
}

function escapeHtml(value) {
  return String(value).replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[char]));
}

async function refreshSessions() {
  state.sessions = await api("/api/sessions");
  if (state.activeSession) {
    state.activeSession = state.sessions.find((session) => session.id === state.activeSession.id) || state.activeSession;
  } else if (state.sessions[0]) {
    const storedSessionID = readStoredState().activeSessionID;
    const restored = state.sessions.find((session) => session.id === storedSessionID) || state.sessions[0];
    state.activeSession = restored;
  }
  await fetchExplanations();
  renderAll();
}

async function fetchExplanations() {
  if (!state.activeSession) return;
  try {
    state.explanations = await api(`/api/sessions/${state.activeSession.id}/explanations`);
  } catch {
    state.explanations = [];
  }
}

function startExplanationPoll() {
  if (state.explanationPollInterval) return;
  const count = state.explanations.length;
  state.explanationPollInterval = setInterval(async () => {
    await fetchExplanations();
    renderAll();
    if (state.explanations.length > count) {
      clearInterval(state.explanationPollInterval);
      state.explanationPollInterval = null;
    }
  }, 3000);
}

async function fetchLiveDiff() {
  try {
    const previousPath = state.activeFile?.path;
    const data = await api("/api/live");
    const stored = readStoredState();
    state.liveData = data;
    state.activeFile = data.files.find((file) => file.path === previousPath)
      || data.files.find((file) => file.path === stored.activeFilePath)
      || data.files[0]
      || null;
    if (state.activeFile?.path !== previousPath) {
      state.selected = { start: 0, end: 0 };
      scrollY = state.activeFile ? stored.scrollByFile?.[state.activeFile.path] || 0 : 0;
    }
    renderAll();
  } catch (error) {
    if (els.summary) els.summary.textContent = error.message;
  }
}

els.viewedFile.addEventListener("change", () => {
  if (!state.activeFile) return;
  const key = fileViewedKey(state.activeFile);
  if (els.viewedFile.checked) {
    state.viewedFiles.add(key);
    state.selected = { start: 0, end: 0 };
    state.inlineReviewOpen = false;
  } else {
    state.viewedFiles.delete(key);
  }
  scrollY = 0;
  renderAll();
});

function scrollDiffBy(deltaY) {
  const max = Math.max(0, state.flatLines.length * rowHeight - els.canvas.getBoundingClientRect().height);
  scrollY = Math.round(Math.max(0, Math.min(max, scrollY + deltaY)));
  if (state.activeFile) {
    writeStoredState({ scrollByFile: { ...(readStoredState().scrollByFile || {}), [state.activeFile.path]: scrollY } });
  }
  clampInlineReview();
  renderDiff();
}

function shouldKeepLocalWheel(event) {
  const scrollable = event.target.closest("textarea, .comment-list");
  if (!scrollable || !els.inlineReview.contains(scrollable)) return false;
  const canScroll = scrollable.scrollHeight > scrollable.clientHeight;
  if (!canScroll) return false;
  const atTop = scrollable.scrollTop <= 0;
  const atBottom = scrollable.scrollTop + scrollable.clientHeight >= scrollable.scrollHeight - 1;
  return (event.deltaY < 0 && !atTop) || (event.deltaY > 0 && !atBottom);
}

els.canvas.addEventListener("wheel", (event) => {
  event.preventDefault();
  scrollDiffBy(event.deltaY);
}, { passive: false });

els.inlineReview.addEventListener("wheel", (event) => {
  if (shouldKeepLocalWheel(event)) return;
  event.preventDefault();
  scrollDiffBy(event.deltaY);
}, { passive: false });

els.canvas.addEventListener("mousedown", (event) => {
  state.inlineReviewOpen = false;
  state.pendingReviewOpen = true;
  const rect = els.canvas.getBoundingClientRect();
  const marker = commentMarkers.find((item) => (
    event.clientX - rect.left >= item.x
    && event.clientX - rect.left <= item.x + item.width
    && event.clientY - rect.top >= item.y
    && event.clientY - rect.top <= item.y + item.height
  ));
  if (marker) {
    if (marker.explanation) {
      const exp = explanationsForActiveFile().find((e) => e.startLine === marker.start);
      if (exp) {
        els.explanation.textContent = exp.body;
        state.selected = { start: marker.rowStart, end: marker.rowEnd };
      }
    } else {
      state.selected = { start: marker.rowStart, end: marker.rowEnd };
      els.inlineReview.classList.remove("visible");
    }
    renderAll();
    return;
  }

  const row = rowFromEvent(event);
  state.selected = { start: row, end: row };
  renderAll();
  els.inlineReview.classList.remove("visible");
});

els.canvas.addEventListener("mousemove", (event) => {
  if (event.buttons !== 1 || !state.selected.start) return;
  state.selected.end = rowFromEvent(event);
  renderAll();
});

els.canvas.addEventListener("mouseup", () => {
  if (!state.pendingReviewOpen) return;
  state.pendingReviewOpen = false;
  openInlineReviewForSelection();
});

els.explain.addEventListener("click", async () => {
  const range = selectedLineRange();
  if (!state.activeFile || !range) return;
  const filePath = state.activeFile.path;
  const session = await ensureReviewSession();
  const result = await api(`/api/sessions/${session.id}/explain`, {
    method: "POST",
    body: JSON.stringify({ filePath, startLine: range.start, endLine: range.end }),
  });
  els.explanation.textContent = result.summary || "Explanation requested — waiting for agent response.";
  startExplanationPoll();
});

els.saveFeedback.addEventListener("click", async () => {
  await saveFeedback();
});

els.panelSubmitReview.addEventListener("click", submitReview);
els.panelAddComment.addEventListener("click", openInlineReviewForSelection);

els.closeAgentPayload.addEventListener("click", () => {
  els.agentPayloadModal.classList.remove("visible");
});

els.clearSession.addEventListener("click", () => {
  if (!window.confirm("Clear all data for this session? This removes comments, viewed state, drafts, active file, and scroll position.")) return;
  clearProjectStorage();
  window.location.reload();
});

els.closeCommentsModal.addEventListener("click", () => {
  els.commentsModal.classList.remove("visible");
});

els.showShortcuts.addEventListener("click", () => {
  els.shortcutsModal.classList.add("visible");
});

els.closeShortcuts.addEventListener("click", () => {
  els.shortcutsModal.classList.remove("visible");
});

els.commentList.addEventListener("click", async (event) => {
  const button = event.target.closest(".delete-comment");
  await deleteComment(button?.dataset.feedbackId);
});

els.fileCommentList.addEventListener("click", async (event) => {
  const button = event.target.closest(".delete-comment");
  await deleteComment(button?.dataset.feedbackId);
});

els.reviewCommentList.addEventListener("click", async (event) => {
  const button = event.target.closest(".delete-comment");
  if (button) {
    event.stopPropagation();
    await deleteComment(button.dataset.feedbackId);
    return;
  }
  const item = event.target.closest(".comment-item");
  const comment = (state.activeSession?.feedback || []).find((entry) => entry.id === item?.dataset.feedbackId);
  if (comment) focusComment(comment);
});

els.diffArea.addEventListener("focus", () => setFocusZone("diff"));
els.reviewSection.addEventListener("focus", () => setFocusZone("queue"));

els.feedback.addEventListener("input", () => {
  if (!state.activeFile || !selectedLineRange()) return;
  sessionStorage.setItem(draftKey(), els.feedback.value);
});

els.closeFeedback.addEventListener("click", () => {
  state.selected = { start: 0, end: 0 };
  state.inlineReviewOpen = false;
  renderAll();
});

function isTypingTarget(target) {
  return ["INPUT", "TEXTAREA", "SELECT"].includes(target?.tagName) || target?.isContentEditable;
}

function moveFile(delta) {
  const files = allFiles();
  if (!files.length) return;
  const current = Math.max(0, files.findIndex((file) => file.path === state.activeFile?.path));
  setActiveFile(files[Math.max(0, Math.min(files.length - 1, current + delta))]);
}

function hunkRows() {
  return state.flatLines
    .map((line, index) => ({ line, row: index + 1 }))
    .filter((item) => item.line.kind === "hunk")
    .map((item) => item.row);
}

function moveHunk(delta) {
  const rows = hunkRows();
  if (!rows.length) return;
  const current = Math.max(1, Math.floor(scrollY / rowHeight) + 1);
  const previous = rows.reduce((match, row) => (row < current ? row : match), rows[0]);
  const target = delta > 0
    ? rows.find((row) => row > current) || rows[rows.length - 1]
    : previous;
  scrollY = Math.max(0, (target - 1) * rowHeight);
  state.selected = { start: target, end: target };
  renderAll();
}

function clampToCodeLines(line) {
  const max = state.codeLineCount || state.flatLines.length;
  return Math.max(1, Math.min(max, line));
}

function moveSelectedLine(delta) {
  if (!state.flatLines.length) return;
  const current = state.selected.end || state.selected.start || Math.max(1, Math.floor(scrollY / rowHeight) + 1);
  const target = clampToCodeLines(current + delta);
  state.selected = { start: target, end: target };
  const rect = els.canvas.getBoundingClientRect();
  const rowTop = (target - 1) * rowHeight;
  if (rowTop < scrollY) scrollY = rowTop;
  if (rowTop + rowHeight > scrollY + rect.height) scrollY = rowTop + rowHeight - rect.height;
  if (state.activeFile) {
    writeStoredState({ scrollByFile: { ...(readStoredState().scrollByFile || {}), [state.activeFile.path]: scrollY } });
  }
  state.inlineReviewOpen = false;
  renderAll();
}

function takeMotionCount() {
  const count = Math.max(1, Number.parseInt(state.motionCount || "1", 10));
  state.motionCount = "";
  return count;
}

function moveComment(delta) {
  const comments = state.activeSession?.feedback || [];
  if (!comments.length) return;
  const current = state.focusedCommentIndex < 0 ? (delta > 0 ? -1 : 0) : state.focusedCommentIndex;
  state.focusedCommentIndex = Math.max(0, Math.min(comments.length - 1, current + delta));
  setFocusZone("queue");
  focusComment(comments[state.focusedCommentIndex]);
}

function toggleViewedActiveFile() {
  if (!state.activeFile || els.viewedFile.disabled) return;
  els.viewedFile.checked = !els.viewedFile.checked;
  els.viewedFile.dispatchEvent(new Event("change"));
}

function listenToShiftKey() {
  let pressed = false;
  window.addEventListener("keydown", (e) => { if (e.key === "Shift") pressed = true; });
  window.addEventListener("keyup", (e) => { if (e.key === "Shift") pressed = false; });
  return { isShiftPressed: () => pressed };
}

function registerReviewKeyboardNavigation() {
  const { isShiftPressed } = listenToShiftKey();

  const zoneMotion = (delta) => {
    const count = takeMotionCount();
    if (state.focusZone === "queue") moveComment(delta * count);
    else moveSelectedLine(delta * count);
  };

  const zoneExtend = (delta) => {
    const count = takeMotionCount();
    if (state.focusZone === "queue") { moveComment(delta * count); return; }
    const current = state.selected.end || state.selected.start || Math.max(1, Math.floor(scrollY / rowHeight) + 1);
    const target = clampToCodeLines(current + delta * count);
    state.selected = { start: state.selected.start || target, end: target };
    const rect = els.canvas.getBoundingClientRect();
    const rowTop = (target - 1) * rowHeight;
    if (rowTop < scrollY) scrollY = rowTop;
    if (rowTop + rowHeight > scrollY + rect.height) scrollY = rowTop + rowHeight - rect.height;
    state.inlineReviewOpen = false;
    renderAll();
  };

  const actions = {
    "?": () => els.shortcutsModal.classList.add("visible"),
    tab: () => toggleFocusZone(),
    " ": () => openInlineReviewForSelection({ focusInput: state.focusZone === "diff" }),
    arrowdown: () => zoneMotion(1),
    arrowup: () => zoneMotion(-1),
    j: () => isShiftPressed() ? zoneExtend(1) : zoneMotion(1),
    k: () => isShiftPressed() ? zoneExtend(-1) : zoneMotion(-1),
    i: () => {
      if (!state.inlineReviewOpen) openInlineReviewForSelection();
      if (state.inlineReviewOpen) els.feedback.focus();
    },
    n: () => moveFile(1),
    p: () => moveFile(-1),
    v: () => toggleViewedActiveFile(),
    f: () => els.files.querySelector("button")?.focus(),
    escape: () => {
      state.motionCount = "";
      els.shortcutsModal.classList.remove("visible");
      els.commentsModal.classList.remove("visible");
      state.inlineReviewOpen = false;
      renderAll();
    },
  };

  window.addEventListener("keydown", async (event) => {
    if (event.key === "Escape" && isTypingTarget(event.target)) {
      event.preventDefault();
      event.target.blur();
      setFocusZone("diff");
      return;
    }
    if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
      event.preventDefault();
      if (isTypingTarget(event.target)) await saveFeedback();
      else await submitReview();
      return;
    }
    if (isTypingTarget(event.target) || event.metaKey || event.ctrlKey || event.altKey) return;

    const key = event.key.toLowerCase();
    if (/^[0-9]$/.test(key)) {
      if (key !== "0" || state.motionCount) {
        event.preventDefault();
        state.motionCount = `${state.motionCount}${key}`.slice(0, 3);
      }
      return;
    }

    const action = actions[key];
    if (!action) {
      state.motionCount = "";
      return;
    }
    event.preventDefault();
    action(event);
  });
}

registerReviewKeyboardNavigation();

window.addEventListener("resize", resizeCanvas);
refreshSessions();
fetchLiveDiff();
state.pollInterval = setInterval(async () => {
  await Promise.all([fetchLiveDiff(), refreshSessions()]);
}, 2000);
resizeCanvas();

{
  const sessionID = new URLSearchParams(window.location.search).get("session");
  if (sessionID) {
    const check = async () => {
      await refreshSessions();
      const found = state.sessions.find((s) => s.id === sessionID);
      if (found) {
        selectSession(found);
      }
    };
    check();
  }
}
