const state = {
  sessions: [],
  activeSession: null,
  activeFile: null,
  flatLines: [],
  selected: { start: 0, end: 0 },
};

const els = {
  repo: document.querySelector("#repo-label"),
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
  saveFeedback: document.querySelector("#save-feedback"),
  exportAgent: document.querySelector("#export-agent"),
  payload: document.querySelector("#agent-payload"),
};

const ctx = els.canvas.getContext("2d");
const rowHeight = 22;
const gutterWidth = 92;
let scrollY = 0;

async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options,
  });
  const body = await response.json();
  if (!response.ok) throw new Error(body.error || "Request failed");
  return body;
}

function tokenColor(token) {
  if (/^(func|const|var|type|return|if|else|case|switch|for|range|package|import)\b/.test(token)) return "#78a6d8";
  if (/^["'`]/.test(token)) return "#d8b878";
  if (/^\d+$/.test(token)) return "#b690d6";
  if (/^(true|false|nil|null|undefined)$/.test(token)) return "#ef8f88";
  if (/^[A-Z][A-Za-z0-9_]+$/.test(token)) return "#78c698";
  return "#f3f0e8";
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
  ctx.clearRect(0, 0, rect.width, rect.height);
  ctx.fillStyle = "#101213";
  ctx.fillRect(0, 0, rect.width, rect.height);
  ctx.font = "13px ui-monospace, SFMono-Regular, Menlo, monospace";
  ctx.textBaseline = "middle";

  const first = Math.max(0, Math.floor(scrollY / rowHeight));
  const visible = Math.ceil(rect.height / rowHeight) + 2;
  for (let i = first; i < Math.min(state.flatLines.length, first + visible); i += 1) {
    const line = state.flatLines[i];
    const y = i * rowHeight - scrollY;
    drawRow(line, i + 1, y, rect.width);
  }
}

function drawRow(line, index, y, width) {
  const selected = state.selected.start && index >= Math.min(state.selected.start, state.selected.end) && index <= Math.max(state.selected.start, state.selected.end);
  const bg = line.kind === "add" ? "#13251b" : line.kind === "del" ? "#2a1717" : line.kind === "hunk" ? "#17212a" : selected ? "#2b2819" : "#101213";
  ctx.fillStyle = selected ? "#3a321c" : bg;
  ctx.fillRect(0, y, width, rowHeight);

  ctx.fillStyle = "#6f7a7a";
  ctx.fillText(String(line.oldNumber || ""), 12, y + rowHeight / 2);
  ctx.fillText(String(line.newNumber || ""), 52, y + rowHeight / 2);

  ctx.fillStyle = line.kind === "add" ? "#78c698" : line.kind === "del" ? "#ef8f88" : line.kind === "hunk" ? "#78a6d8" : "#9aa6a6";
  ctx.fillText(line.kind === "add" ? "+" : line.kind === "del" ? "-" : " ", gutterWidth, y + rowHeight / 2);

  drawHighlightedText(line.text || "", gutterWidth + 18, y + rowHeight / 2);
}

function drawHighlightedText(text, x, y) {
  const tokens = text.split(/(\s+|[()[\]{}.,;:+\-*/=<>!]+)/);
  let cursor = x;
  for (const token of tokens) {
    ctx.fillStyle = tokenColor(token);
    ctx.fillText(token, cursor, y);
    cursor += ctx.measureText(token).width;
  }
}

function rowFromEvent(event) {
  const rect = els.canvas.getBoundingClientRect();
  return Math.max(1, Math.min(state.flatLines.length, Math.floor((event.clientY - rect.top + scrollY) / rowHeight) + 1));
}

function selectSession(session) {
  state.activeSession = session;
  state.activeFile = session.files[0] || null;
  state.selected = { start: 0, end: 0 };
  renderAll();
}

function selectFile(file) {
  state.activeFile = file;
  state.selected = { start: 0, end: 0 };
  scrollY = 0;
  renderAll();
}

function renderAll() {
  renderSessions();
  renderHeader();
  renderFiles();
  state.flatLines = state.activeFile ? flatten(state.activeFile) : [];
  els.activeFile.textContent = state.activeFile ? state.activeFile.path : "No file selected";
  renderDiff();
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
    button.className = `file ${state.activeFile?.path === file.path ? "active" : ""}`;
    button.innerHTML = `<strong>${escapeHtml(file.path)}</strong><small>${file.status} · +${file.additions} -${file.deletions}</small>`;
    button.addEventListener("click", () => selectFile(file));
    return button;
  }));
}

function escapeHtml(value) {
  return String(value).replace(/[&<>"']/g, (char) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[char]));
}

async function refreshSessions() {
  state.sessions = await api("/api/sessions");
  if (!state.activeSession && state.sessions[0]) selectSession(state.sessions[0]);
  renderAll();
}

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
  scrollY = Math.max(0, Math.min(max, scrollY + event.deltaY));
  renderDiff();
}, { passive: false });

els.canvas.addEventListener("mousedown", (event) => {
  const row = rowFromEvent(event);
  state.selected = { start: row, end: row };
  renderDiff();
});

els.canvas.addEventListener("mousemove", (event) => {
  if (event.buttons !== 1 || !state.selected.start) return;
  state.selected.end = rowFromEvent(event);
  renderDiff();
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
  if (!state.activeSession || !state.activeFile) return;
  await api(`/api/sessions/${state.activeSession.id}/feedback`, {
    method: "POST",
    body: JSON.stringify({ filePath: state.activeFile.path, startLine: state.selected.start, endLine: state.selected.end, body: els.feedback.value }),
  });
  els.feedback.value = "";
  await refreshSessions();
});

els.exportAgent.addEventListener("click", async () => {
  if (!state.activeSession) return;
  const payload = await api(`/api/sessions/${state.activeSession.id}/agent-payload`);
  els.payload.textContent = JSON.stringify(payload, null, 2);
});

window.addEventListener("resize", resizeCanvas);
refreshSessions().then(resizeCanvas).catch((error) => {
  els.summary.textContent = error.message;
  resizeCanvas();
});
