"use strict";

// ---- Phase model -----------------------------------------------------------
// Each phase (besides configure/runbook) maps to one or more generated files.
// `run` marks scripts that live mode can execute.
const PHASES = [
  { id: "configure", label: "Configure", files: [] },
  {
    id: "prep", label: "Prep machine",
    intro: "Run on a machine WITH internet that can also reach your registry. " +
           "Downloads and verifies the bundle, pushes images/charts, fetches k0s binaries.",
    files: [{ name: "01-prepare.sh", run: true }],
  },
  {
    id: "fileserver", label: "Fileserver",
    intro: "Airgapped cluster side. Serves the k0s binary over HTTP via nginx. " +
           "Apply in order, running load-k0s.sh after the loader pod is up.",
    files: [
      { name: "02-fileserver/namespace-pvc.yaml" },
      { name: "02-fileserver/binary-loader.yaml" },
      { name: "02-fileserver/load-k0s.sh", run: true },
      { name: "02-fileserver/configmap-nginx.yaml" },
      { name: "02-fileserver/deployment-service.yaml" },
    ],
  },
  {
    id: "install", label: "Install",
    intro: "Airgapped cluster side. Review the values, then install the kcm chart from your registry.",
    files: [
      { name: "03-values.yaml" },
      { name: "04-install.sh", run: true },
    ],
  },
  {
    id: "verify", label: "Verify",
    intro: "Wait 5–10 minutes after install, then check pods, the Management object, and templates.",
    files: [{ name: "05-verify.sh", run: true }],
  },
  { id: "runbook", label: "Runbook", files: [{ name: "RUNBOOK.md", md: true }] },
];

const state = { live: false, generated: false, active: "configure" };

const $ = (sel, el = document) => el.querySelector(sel);
const $$ = (sel, el = document) => Array.from(el.querySelectorAll(sel));

// ---- Boot ------------------------------------------------------------------
async function boot() {
  buildNav();
  await loadMeta();
  await loadConfig();
  wireForm();
  show("configure");
}

async function loadMeta() {
  try {
    const m = await getJSON("/api/meta");
    state.live = !!m.live;
  } catch (_) {}
  const badge = $("#mode-badge");
  if (state.live) {
    badge.textContent = "● Live mode — can execute";
    badge.className = "mode live";
  } else {
    badge.textContent = "Generator mode — read only";
    badge.className = "mode gen";
  }
}

async function loadConfig() {
  try {
    const cfg = await getJSON("/api/config");
    fillForm(cfg);
    updateDerived();
  } catch (e) { /* keep HTML defaults */ }
}

// ---- Navigation ------------------------------------------------------------
function buildNav() {
  const nav = $("#nav");
  nav.innerHTML = "";
  PHASES.forEach((p, i) => {
    const b = document.createElement("button");
    b.className = "nav-item";
    b.dataset.id = p.id;
    b.innerHTML = `<span class="num">${i === 0 ? "⚙" : i}</span><span>${p.label}</span>`;
    b.addEventListener("click", () => {
      if (p.id !== "configure" && !state.generated) return;
      show(p.id);
    });
    nav.appendChild(b);
  });
}

function refreshNav() {
  $$(".nav-item").forEach((b) => {
    const id = b.dataset.id;
    b.classList.toggle("active", id === state.active);
    b.classList.toggle("locked", id !== "configure" && !state.generated);
  });
}

function show(id) {
  state.active = id;
  $$(".panel").forEach((p) => p.classList.toggle("hidden", p.dataset.phase !== id));
  refreshNav();
  const phase = PHASES.find((p) => p.id === id);
  if (phase && id !== "configure") renderPhase(phase);
}

// ---- Config form <-> object ------------------------------------------------
const TEXT_FIELDS = [
  "kcmVersion", "k0sVersion", "skopeoVersion", "registryHost", "registryProject",
  "fileserverURL", "fileserverNamespace", "storageClass", "pvcSize", "nginxImage",
  "busyboxImage", "namespace", "bundleBaseURL", "outputDir", "caCertSecretName",
];

function fillForm(cfg) {
  TEXT_FIELDS.forEach((f) => { const el = form()[f]; if (el) el.value = cfg[f] ?? ""; });
  $$('input[name="arch"]').forEach((c) => { c.checked = (cfg.architectures || []).includes(c.value); });
  $('input[name="selfSignedCA"]').checked = !!cfg.selfSignedCA;
}

function readForm() {
  const cfg = {};
  TEXT_FIELDS.forEach((f) => { const el = form()[f]; if (el) cfg[f] = el.value.trim(); });
  cfg.architectures = $$('input[name="arch"]:checked').map((c) => c.value);
  cfg.selfSignedCA = $('input[name="selfSignedCA"]').checked;
  return cfg;
}

const form = () => document.forms["cfg-form"] || $("#cfg-form");

function updateDerived() {
  const host = (form().registryHost.value || "registry.local").replace(/\/+$/, "");
  const proj = (form().registryProject.value || "k0rdent-enterprise").replace(/^\/+|\/+$/g, "");
  $("#derived-registry").textContent = `${host}/${proj}`;
}

function wireForm() {
  form().registryHost.addEventListener("input", updateDerived);
  form().registryProject.addEventListener("input", updateDerived);
  $("#cfg-form").addEventListener("submit", onGenerate);
}

// ---- Generate --------------------------------------------------------------
async function onGenerate(e) {
  e.preventDefault();
  const btn = $("#generate-btn");
  const status = $("#gen-status");
  btn.disabled = true;
  status.className = "status";
  status.textContent = "Generating…";
  try {
    const res = await postJSON("/api/generate", readForm());
    state.generated = true;
    status.className = "status ok";
    status.textContent = `✓ Wrote ${res.files.length} files to ${res.outputDir}/`;
    refreshNav();
  } catch (err) {
    status.className = "status err";
    status.textContent = "✗ " + err.message;
  } finally {
    btn.disabled = false;
  }
}

// ---- Phase rendering -------------------------------------------------------
async function renderPhase(phase) {
  const panel = $(`.panel[data-phase="${phase.id}"]`);
  panel.innerHTML = "";
  const idx = PHASES.indexOf(phase);

  const h = document.createElement("h2");
  h.textContent = `${idx} · ${phase.label}`;
  panel.appendChild(h);

  if (phase.intro) {
    const p = document.createElement("p");
    p.className = "lead";
    p.textContent = phase.intro;
    panel.appendChild(p);
  }

  if (phase.id === "prep") {
    panel.appendChild(callout("This is the only step that needs internet access. " +
      "Everything after runs inside the airgap.", false));
    if (state.live) await renderPreflight(panel, "prep");
  }
  if (phase.id === "runbook") {
    await renderMarkdown(panel, phase.files[0].name);
    return;
  }

  for (const f of phase.files) {
    panel.appendChild(await fileCard(f));
  }
}

function callout(text, warn) {
  const d = document.createElement("div");
  d.className = "callout" + (warn ? " warn" : "");
  d.textContent = text;
  return d;
}

async function fileCard(f) {
  const wrap = document.createElement("div");
  wrap.className = "file";

  const head = document.createElement("div");
  head.className = "file-head";
  head.innerHTML = `<span class="name">${f.name}</span>`;
  const btns = document.createElement("div");
  btns.className = "btns";

  let content = "";
  try {
    const data = await getJSON("/api/file?name=" + encodeURIComponent(f.name));
    content = data.content;
  } catch (e) {
    content = "// " + e.message;
  }

  const copyBtn = document.createElement("button");
  copyBtn.textContent = "Copy";
  copyBtn.addEventListener("click", () => {
    navigator.clipboard.writeText(content).then(() => {
      copyBtn.textContent = "Copied ✓";
      setTimeout(() => (copyBtn.textContent = "Copy"), 1500);
    });
  });
  btns.appendChild(copyBtn);

  const pre = document.createElement("pre");
  pre.className = "code";
  pre.textContent = content;

  let logEl = null;
  if (f.run && state.live) {
    const runBtn = document.createElement("button");
    runBtn.className = "run";
    runBtn.textContent = "▶ Run";
    runBtn.addEventListener("click", () => {
      if (!logEl) { logEl = document.createElement("pre"); logEl.className = "log"; wrap.appendChild(logEl); }
      logEl.textContent = "";
      runScript(f.name, runBtn, logEl);
    });
    btns.appendChild(runBtn);
  } else if (f.run && !state.live) {
    const note = document.createElement("button");
    note.textContent = "Run (needs --live)";
    note.disabled = true;
    btns.appendChild(note);
  }

  head.appendChild(btns);
  wrap.appendChild(head);
  wrap.appendChild(pre);
  return wrap;
}

// ---- Live execution (SSE) --------------------------------------------------
function runScript(name, btn, logEl) {
  btn.disabled = true;
  const orig = btn.textContent;
  btn.textContent = "Running…";
  const es = new EventSource("/api/run?name=" + encodeURIComponent(name));
  es.onmessage = (ev) => {
    let line; try { line = JSON.parse(ev.data); } catch { return; }
    if (line.Done) {
      es.close();
      btn.disabled = false;
      btn.textContent = orig;
      if (line.Err) appendLog(logEl, "✗ " + line.Err, "err");
      else appendLog(logEl, "✓ completed", "ok");
      return;
    }
    appendLog(logEl, line.Text, "");
  };
  es.onerror = () => {
    es.close();
    btn.disabled = false;
    btn.textContent = orig;
    appendLog(logEl, "✗ connection lost", "err");
  };
}

function appendLog(logEl, text, cls) {
  const span = document.createElement("span");
  if (cls) span.className = "l-" + cls;
  span.textContent = text + "\n";
  logEl.appendChild(span);
  logEl.scrollTop = logEl.scrollHeight;
}

// ---- Preflight -------------------------------------------------------------
async function renderPreflight(panel, side) {
  try {
    const { tools } = await getJSON("/api/preflight");
    const box = document.createElement("div");
    box.className = "preflight";
    tools.filter((t) => t.side === side || t.side === "both").forEach((t) => {
      const row = document.createElement("div");
      row.className = "tool";
      row.innerHTML =
        `<span class="dot ${t.found ? "on" : "off"}"></span>` +
        `<span class="tname">${t.name}</span>` +
        `<span class="tside">${t.side}</span>` +
        `<span class="tver">${t.found ? (t.version || "found") : "missing"}</span>`;
      row.title = t.purpose;
      box.appendChild(row);
    });
    panel.appendChild(box);
  } catch (_) {}
}

// ---- Minimal Markdown renderer (offline, no deps) --------------------------
async function renderMarkdown(panel, name) {
  const div = document.createElement("div");
  div.className = "runbook-md";
  try {
    const data = await getJSON("/api/file?name=" + encodeURIComponent(name));
    div.innerHTML = mdToHtml(data.content);
  } catch (e) {
    div.textContent = e.message;
  }
  panel.appendChild(div);
}

function esc(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}
function inline(s) {
  return esc(s)
    .replace(/`([^`]+)`/g, "<code>$1</code>")
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2">$1</a>');
}
function mdToHtml(md) {
  const lines = md.split("\n");
  let html = "", i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (line.startsWith("```")) {
      let code = ""; i++;
      while (i < lines.length && !lines[i].startsWith("```")) { code += esc(lines[i]) + "\n"; i++; }
      i++; html += `<pre><code>${code}</code></pre>`; continue;
    }
    if (/^\|/.test(line) && /\|/.test(lines[i + 1] || "") && /^[\s|:-]+$/.test(lines[i + 1] || "")) {
      const rows = []; while (i < lines.length && /^\|/.test(lines[i])) { rows.push(lines[i]); i++; }
      html += renderTable(rows); continue;
    }
    if (/^### /.test(line)) { html += `<h3>${inline(line.slice(4))}</h3>`; i++; continue; }
    if (/^## /.test(line)) { html += `<h2>${inline(line.slice(3))}</h2>`; i++; continue; }
    if (/^# /.test(line)) { html += `<h1>${inline(line.slice(2))}</h1>`; i++; continue; }
    if (/^---+$/.test(line)) { html += "<hr>"; i++; continue; }
    if (/^\s*[-*] /.test(line)) {
      html += "<ul>";
      while (i < lines.length && /^\s*[-*] /.test(lines[i])) { html += `<li>${inline(lines[i].replace(/^\s*[-*] /, ""))}</li>`; i++; }
      html += "</ul>"; continue;
    }
    if (/^\d+\. /.test(line)) {
      html += "<ol>";
      while (i < lines.length && /^\d+\. /.test(lines[i])) { html += `<li>${inline(lines[i].replace(/^\d+\. /, ""))}</li>`; i++; }
      html += "</ol>"; continue;
    }
    if (line.trim() === "") { i++; continue; }
    let para = line; i++;
    while (i < lines.length && lines[i].trim() !== "" && !/^[#`|>-]/.test(lines[i]) && !/^\d+\. /.test(lines[i])) { para += " " + lines[i]; i++; }
    html += `<p>${inline(para.replace(/^> ?/, ""))}</p>`;
  }
  return html;
}
function renderTable(rows) {
  const cells = (r) => r.replace(/^\||\|$/g, "").split("|").map((c) => c.trim());
  const head = cells(rows[0]);
  let h = "<table><thead><tr>" + head.map((c) => `<th>${inline(c)}</th>`).join("") + "</tr></thead><tbody>";
  for (let r = 2; r < rows.length; r++) {
    h += "<tr>" + cells(rows[r]).map((c) => `<td>${inline(c)}</td>`).join("") + "</tr>";
  }
  return h + "</tbody></table>";
}

// ---- fetch helpers ---------------------------------------------------------
async function getJSON(url) {
  const r = await fetch(url);
  const data = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(data.error || r.statusText);
  return data;
}
async function postJSON(url, body) {
  const r = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  const data = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(data.error || r.statusText);
  return data;
}

boot();
