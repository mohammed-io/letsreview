const state = {
  projectID: new URLSearchParams(window.location.search).get("project"),
  mode: "live",
  sessions: [],
  activeSession: null,
  activeFile: null,
  flatLines: [],
  selected: { start: 0, end: 0 },
  liveData: null,
  pollInterval: null,
  creatingSession: null,
  inlineReviewOpen: false,
  pendingReviewOpen: false,
  viewedFiles: new Set(),
};

const els = {
  repo: document.querySelector("#repo-label"),
  modeLive: document.querySelector("#mode-live"),
  modeSessions: document.querySelector("#mode-sessions"),
  comparisonPanel: document.querySelector("#comparison-panel"),
  sessionsPanel: document.querySelector("#sessions-panel"),
  mode: document.querySelector("#mode"),
  refs: document.querySelector("#refs"),
  baseRef: document.querySelector("#base-ref"),
  headRef: document.querySelector("#head-ref"),
  create: document.querySelector("#create-session"),
  sessions: document.querySelector("#sessions"),
  title: document.querySelector("#session-title"),
  summary: document.querySelector("#session-summary"),
  statFiles: document.querySelector("#stat-files"),
  statAdd: document.querySelector("#stat-add"),
  statDel: document.querySelector("#stat-del"),
  files: document.querySelector("#files"),
  activeFile: document.querySelector("#active-file"),
  canvas: document.querySelector("#diff-canvas"),
  explain: document.querySelector("#explain-selection"),
  explanation: document.querySelector("#explanation"),
  feedback: document.querySelector("#feedback"),
  closeFeedback: document.querySelector("#close-feedback"),
  inlineReview: document.querySelector("#inline-review"),
  inlineReviewTitle: document.querySelector("#inline-review-title"),
  commentList: document.querySelector("#comment-list"),
  saveFeedback: document.querySelector("#save-feedback"),
  showFileComments: document.querySelector("#show-file-comments"),
  commentsModal: document.querySelector("#comments-modal"),
  closeCommentsModal: document.querySelector("#close-comments-modal"),
  fileCommentList: document.querySelector("#file-comment-list"),
  exportAgent: document.querySelector("#export-agent"),
  clearSession: document.querySelector("#clear-session"),
  agentPayloadModal: document.querySelector("#agent-payload-modal"),
  closeAgentPayload: document.querySelector("#close-agent-payload"),
  payload: document.querySelector("#agent-payload"),
  viewedFile: document.querySelector("#viewed-file"),
};

{
  const shortcut = els.saveFeedback.querySelector(".shortcut");
  if (shortcut) shortcut.textContent = (navigator.platform.includes("Mac") ? "⌘" : "Ctrl") + " + Enter";
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
  selected: "#1f6feb33",
  selectedBorder: "#1f6feb",
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
  const start = Math.min(state.selected.start || 0, state.selected.end || 0);
  const end = Math.max(state.selected.start || 0, state.selected.end || 0);
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

function commentsForRange(start, end) {
  const low = Math.min(start, end);
  const high = Math.max(start, end);
  return commentsForActiveFile().filter((comment) => {
    const commentLow = Math.min(comment.startLine, comment.endLine);
    const commentHigh = Math.max(comment.startLine, comment.endLine);
    return commentLow <= high && commentHigh >= low;
  });
}

function markerGroups() {
  return Object.values(commentsForActiveFile().reduce((groups, comment) => {
    const start = Math.min(comment.startLine, comment.endLine);
    const end = Math.max(comment.startLine, comment.endLine);
    const key = `${start}:${end}`;
    return {
      ...groups,
      [key]: {
        start,
        end,
        count: (groups[key]?.count || 0) + 1,
      },
    };
  }, {}));
}

function commentCountForFile(path) {
  return (state.activeSession?.feedback || []).filter((comment) => comment.filePath === path).length;
}

function totalCommentCount() {
  return state.activeSession?.feedback?.length || 0;
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
  const group = markerGroups().find((item) => item.start === row);
  if (!group) return;

  const label = String(group.count);
  const markerWidth = Math.max(22, ctx.measureText(label).width + 14);
  const markerHeight = 16;
  const x = width - markerWidth - 12;
  const markerY = y + Math.floor((rowHeight - markerHeight) / 2);
  ctx.fillStyle = "#1f6feb";
  roundRect(x, markerY, markerWidth, markerHeight, 8);
  ctx.fill();
  ctx.fillStyle = "#ffffff";
  ctx.textAlign = "center";
  ctx.fillText(label, x + markerWidth / 2, y + rowHeight / 2);
  ctx.textAlign = "left";
  commentMarkers.push({ ...group, x, y: markerY, width: markerWidth, height: markerHeight });
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

function drawRow(line, index, y, width) {
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
  return Math.max(1, Math.min(state.flatLines.length, Math.floor((event.clientY - rect.top + scrollY) / rowHeight) + 1));
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

function renderAll() {
  if (state.mode === "live") {
    renderLive();
  } else {
    renderSessions();
    renderHeader();
    renderFiles();
  }
  state.flatLines = state.activeFile && !isFileViewed(state.activeFile) ? flatten(state.activeFile) : [];
  els.activeFile.textContent = state.activeFile ? state.activeFile.path : "No file selected";
  els.viewedFile.checked = Boolean(state.activeFile && isFileViewed(state.activeFile));
  els.viewedFile.disabled = !state.activeFile;
  const hasSelection = Boolean(state.selected.start && state.selected.end);
  const sessionReady = Boolean(state.activeSession && state.activeFile);
  els.explain.disabled = !sessionReady || !hasSelection;
  els.saveFeedback.disabled = !sessionReady || !hasSelection;
  els.showFileComments.disabled = !state.activeFile || commentsForActiveFile().length === 0;
  els.exportAgent.disabled = totalCommentCount() === 0;
  els.clearSession.disabled = !state.activeSession && !sessionStorage.getItem(storageKey("state"));
  renderInlineReview();
  renderDiff();
}

function renderInlineReview() {
  const hasSelection = Boolean(state.selected.start && state.selected.end);
  const canComment = Boolean(state.inlineReviewOpen && state.activeSession && state.activeFile && hasSelection);
  els.inlineReview.classList.toggle("visible", canComment);
  if (!canComment) return;

  const start = Math.min(state.selected.start, state.selected.end);
  const end = Math.max(state.selected.start, state.selected.end);
  const stage = els.canvas.getBoundingClientRect();
  const panelHeight = els.inlineReview.offsetHeight || 220;
  const wantedTop = end * rowHeight - scrollY + 6;
  const maxTop = Math.max(8, stage.height - panelHeight - 12);
  const top = Math.max(8, Math.min(wantedTop, maxTop));
  els.inlineReview.style.top = `${top}px`;
  els.inlineReviewTitle.textContent = start === end ? `Comment on line ${start}` : `Comment on lines ${start}-${end}`;
  renderCommentList(start, end);
  els.feedback.value = sessionStorage.getItem(draftKey()) || "";
}

function renderCommentList(start, end) {
  const comments = commentsForRange(start, end);
  els.commentList.replaceChildren(...commentNodes(comments));
}

function commentNodes(comments) {
  return comments.map((comment) => {
    const item = document.createElement("article");
    item.className = "comment-item";
    const range = comment.startLine === comment.endLine ? `Line ${comment.startLine}` : `Lines ${comment.startLine}-${comment.endLine}`;
    const created = new Date(comment.createdAt).toLocaleString();
    item.innerHTML = `
      <div class="comment-meta">
        <div>
          <strong>${escapeHtml(range)}</strong>
          <span>${escapeHtml(created)}</span>
        </div>
        <button class="secondary delete-comment" data-feedback-id="${escapeHtml(comment.id)}">Delete</button>
      </div>
      <p>${escapeHtml(comment.body)}</p>
    `;
    return item;
  });
}

async function saveFeedback() {
  if (!state.activeSession || !state.activeFile || !els.feedback.value.trim()) return;
  await api(`/api/sessions/${state.activeSession.id}/feedback`, {
    method: "POST",
    body: JSON.stringify({ filePath: state.activeFile.path, startLine: state.selected.start, endLine: state.selected.end, body: els.feedback.value }),
  });
  sessionStorage.removeItem(draftKey());
  els.feedback.value = "";
  state.selected = { start: 0, end: 0 };
  state.inlineReviewOpen = false;
  await refreshSessions();
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

async function openInlineReviewForSelection() {
  if (!state.activeFile || !state.selected.start) return;
  const selectedPath = state.activeFile.path;
  const session = await ensureReviewSession();
  state.activeFile = session.files.find((file) => file.path === selectedPath) || state.activeFile;
  state.inlineReviewOpen = true;
  renderAll();
  els.feedback.focus();
}

function renderLive() {
  const data = state.liveData;
  els.title.textContent = data ? "Live: Working Tree" : "Loading...";
  els.summary.textContent = data ? data.summary : "Fetching diff...";
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
}

function renderSessions() {
  els.sessions.replaceChildren(...state.sessions.map((session) => {
    const button = document.createElement("button");
    button.className = `session ${state.activeSession?.id === session.id ? "active" : ""}`;
    button.innerHTML = `<strong>${escapeHtml(session.title)}</strong><small>${session.stats.files} files · +${session.stats.additions} -${session.stats.deletions}</small>`;
    button.addEventListener("click", () => selectSession(session));
    return button;
  }));
}

function renderHeader() {
  const session = state.activeSession;
  els.title.textContent = session ? session.title : "No review loaded";
  els.summary.textContent = session ? session.summary : "Create a review to inspect diffs.";
  els.statFiles.textContent = `${session?.stats.files || 0} files`;
  els.statAdd.textContent = `+${session?.stats.additions || 0}`;
  els.statDel.textContent = `-${session?.stats.deletions || 0}`;
  els.repo.textContent = session?.meta.repo || "local repo";
}

function renderFiles() {
  const files = state.activeSession?.files || [];
  els.files.replaceChildren(...files.map((file) => {
    const button = document.createElement("button");
    button.className = `file ${state.activeFile?.path === file.path ? "active" : ""} ${isFileViewed(file) ? "viewed" : ""}`;
    const comments = commentCountForFile(file.path);
    button.innerHTML = `<span class="file-status">${isFileViewed(file) ? "✓" : escapeHtml(file.status.slice(0, 1).toUpperCase())}</span><strong>${escapeHtml(file.path)}${comments ? ` <em>(${comments})</em>` : ""}</strong><small>+${file.additions} -${file.deletions}</small>`;
    button.addEventListener("click", () => selectFile(file));
    return button;
  }));
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
    selectSession(state.sessions.find((session) => session.id === storedSessionID) || state.sessions[0]);
    return;
  }
  renderAll();
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
    els.summary.textContent = error.message;
  }
}

function setMode(mode) {
  state.mode = mode;
  els.modeLive.classList.toggle("active", mode === "live");
  els.modeSessions.classList.toggle("active", mode === "sessions");
  els.comparisonPanel.style.display = mode === "sessions" ? "grid" : "none";
  els.sessionsPanel.style.display = mode === "sessions" ? "flex" : "none";

  if (mode === "live") {
    refreshSessions();
    fetchLiveDiff();
    state.pollInterval = setInterval(fetchLiveDiff, 2000);
  } else {
    if (state.pollInterval) clearInterval(state.pollInterval);
    refreshSessions();
  }
}

els.modeLive.addEventListener("click", () => setMode("live"));
els.modeSessions.addEventListener("click", () => setMode("sessions"));

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

els.mode.addEventListener("change", () => {
  els.refs.classList.toggle("visible", els.mode.value === "refs");
});

els.create.addEventListener("click", async () => {
  const body = { mode: els.mode.value, baseRef: els.baseRef.value, headRef: els.headRef.value };
  const session = await api("/api/sessions", { method: "POST", body: JSON.stringify(body) });
  state.sessions = [session, ...state.sessions.filter((item) => item.id !== session.id)];
  selectSession(session);
});

els.canvas.addEventListener("wheel", (event) => {
  event.preventDefault();
  const max = Math.max(0, state.flatLines.length * rowHeight - els.canvas.getBoundingClientRect().height);
  scrollY = Math.round(Math.max(0, Math.min(max, scrollY + event.deltaY)));
  if (state.activeFile) {
    writeStoredState({ scrollByFile: { ...(readStoredState().scrollByFile || {}), [state.activeFile.path]: scrollY } });
  }
  renderInlineReview();
  renderDiff();
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
    state.selected = { start: marker.start, end: marker.end };
    renderAll();
    els.inlineReview.classList.remove("visible");
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
  if (!state.activeSession || !state.activeFile) return;
  const result = await api(`/api/sessions/${state.activeSession.id}/explain`, {
    method: "POST",
    body: JSON.stringify({ filePath: state.activeFile.path, startLine: state.selected.start, endLine: state.selected.end }),
  });
  els.explanation.textContent = result.summary;
});

els.saveFeedback.addEventListener("click", async () => {
  await saveFeedback();
});

els.exportAgent.addEventListener("click", async () => {
  if (!state.activeSession) return;
  const payload = await api(`/api/sessions/${state.activeSession.id}/agent-payload`);
  els.payload.textContent = JSON.stringify(payload, null, 2);
  els.agentPayloadModal.classList.add("visible");
});

els.closeAgentPayload.addEventListener("click", () => {
  els.agentPayloadModal.classList.remove("visible");
});

els.clearSession.addEventListener("click", () => {
  if (!window.confirm("Clear all data for this session? This removes comments, viewed state, drafts, active file, and scroll position.")) return;
  clearProjectStorage();
  window.location.reload();
});

els.showFileComments.addEventListener("click", () => {
  els.fileCommentList.replaceChildren(...commentNodes(commentsForActiveFile()));
  els.commentsModal.classList.add("visible");
});

els.closeCommentsModal.addEventListener("click", () => {
  els.commentsModal.classList.remove("visible");
});

els.commentList.addEventListener("click", async (event) => {
  const button = event.target.closest(".delete-comment");
  await deleteComment(button?.dataset.feedbackId);
});

els.fileCommentList.addEventListener("click", async (event) => {
  const button = event.target.closest(".delete-comment");
  await deleteComment(button?.dataset.feedbackId);
});

els.feedback.addEventListener("input", () => {
  if (!state.activeFile || !state.selected.start) return;
  sessionStorage.setItem(draftKey(), els.feedback.value);
});

els.feedback.addEventListener("keydown", async (event) => {
  if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
    event.preventDefault();
    await saveFeedback();
  }
});

els.closeFeedback.addEventListener("click", () => {
  state.selected = { start: 0, end: 0 };
  state.inlineReviewOpen = false;
  renderAll();
});

window.addEventListener("resize", resizeCanvas);
setMode("live");
resizeCanvas();
