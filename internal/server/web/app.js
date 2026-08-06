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
    intro: "Airgapped cluster side. Serves the k0s binary over HTTP via busybox httpd " +
           "(already mirrored). Apply in order, running load-k0s.sh after the loader pod is up.",
    files: [
      { name: "02-fileserver/namespace-pvc.yaml" },
      { name: "02-fileserver/binary-loader.yaml" },
      { name: "02-fileserver/load-k0s.sh", run: true },
      { name: "02-fileserver/deployment-service.yaml" },
    ],
  },
  {
    id: "preflight", label: "Preflight",
    intro: "After the fileserver is up and before installing, prove that the registry " +
           "and every node can reach, trust, and pull images. Run check-access.sh anywhere; " +
           "run check-node-pull to test all nodes from inside the cluster.",
    files: [
      { name: "check-access.sh", run: true },
      { name: "check-node-pull/namespace-daemonset.yaml" },
      { name: "check-node-pull/run.sh", run: true },
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
  {
    id: "diagnostics", label: "Diagnostics",
    intro: "If an image will not pull or a component will not install, run this to surface " +
           "pull errors, crashing pods, their events, the Helm release status, and template readiness.",
    files: [{ name: "diagnose.sh", run: true }],
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
    b.addEventListener("click", () => show(p.id));
    nav.appendChild(b);
  });
}

function refreshNav() {
  $$(".nav-item").forEach((b) => {
    const id = b.dataset.id;
    b.classList.toggle("active", id === state.active);
    // Tabs are always navigable; mark not-yet-generated ones as pending.
    b.classList.toggle("pending", id !== "configure" && !state.generated);
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
  "fileserverURL", "fileserverNamespace", "storageClass", "pvcSize",
  "busyboxImage", "namespace", "bundleBaseURL", "outputDir", "caCertPath", "caCertSecretName",
  "acmeServer", "acmeEmail", "acmeIngressClass", "uiPassword",
];

const CA_HINTS = {
  "self-signed": "Provide the registry's own self-signed certificate (PEM). A Secret with this as ca.crt is created before install.",
  "custom-ca": "Provide the CA bundle (PEM) that signed the registry certificate. A Secret with this as ca.crt is created before install.",
};

function fillForm(cfg) {
  TEXT_FIELDS.forEach((f) => { const el = form()[f]; if (el) el.value = cfg[f] ?? ""; });
  $$('input[name="arch"]').forEach((c) => { c.checked = (cfg.architectures || []).includes(c.value); });
  const mode = cfg.tlsMode || "none";
  const radio = $(`input[name="tlsMode"][value="${mode}"]`);
  if (radio) radio.checked = true;
  $('input[name="installUI"]').checked = cfg.installUI !== false;
  updateTLS();
  updateUI();
}

function readForm() {
  const cfg = {};
  TEXT_FIELDS.forEach((f) => { const el = form()[f]; if (el) cfg[f] = el.value.trim(); });
  cfg.architectures = $$('input[name="arch"]:checked').map((c) => c.value);
  const sel = $('input[name="tlsMode"]:checked');
  cfg.tlsMode = sel ? sel.value : "none";
  cfg.installUI = $('input[name="installUI"]').checked;
  return cfg;
}

// Show the UI password field only when the console is being installed.
function updateUI() {
  const on = $('input[name="installUI"]').checked;
  $("#ui-fields").classList.toggle("hidden", !on);
}

// Show the CA cert fields only when a cert mode is selected, and adapt the hint.
function updateTLS() {
  const sel = $('input[name="tlsMode"]:checked');
  const mode = sel ? sel.value : "none";
  const custom = mode === "self-signed" || mode === "custom-ca";
  $("#ca-fields").classList.toggle("hidden", !custom);
  $("#acme-fields").classList.toggle("hidden", mode !== "acme");
  if (custom) $("#ca-hint").textContent = CA_HINTS[mode] || "";
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
  $$('input[name="tlsMode"]').forEach((r) => r.addEventListener("change", updateTLS));
  $('input[name="installUI"]').addEventListener("change", updateUI);
  // Clear a field's error the moment the user edits it.
  form().querySelectorAll("input").forEach((el) =>
    el.addEventListener("input", () => clearFieldError(el.name)));
  $("#cfg-form").addEventListener("submit", onGenerate);
  $("#check-btn").addEventListener("click", onValidateAndCheck);
}

// ---- Per-field validation display ------------------------------------------
function anchorFor(key) {
  if (key === "architectures") return $("#arch-group");
  if (key === "tlsMode") return $("#tls-mode");
  return $(`input[name="${key}"]`);
}

function clearFieldErrors() {
  $$(".field-err").forEach((el) => el.remove());
  $$(".invalid").forEach((el) => el.classList.remove("invalid"));
}

function clearFieldError(key) {
  const a = anchorFor(key);
  if (!a) return;
  a.classList.remove("invalid");
  const next = a.nextElementSibling;
  if (next && next.classList.contains("field-err")) next.remove();
}

function showFieldErrors(fields) {
  clearFieldErrors();
  const keys = Object.keys(fields);
  keys.forEach((key) => {
    const a = anchorFor(key);
    if (!a) return;
    a.classList.add("invalid");
    if (!(a.nextElementSibling && a.nextElementSibling.classList.contains("field-err"))) {
      const err = document.createElement("span");
      err.className = "field-err";
      err.textContent = fields[key];
      a.insertAdjacentElement("afterend", err);
    }
  });
  return keys.length;
}

// ---- Generate --------------------------------------------------------------
async function onGenerate(e) {
  e.preventDefault();
  const btn = $("#generate-btn");
  const status = $("#gen-status");
  btn.disabled = true;
  status.className = "status";
  status.textContent = "Validating…";
  clearFieldErrors();
  try {
    // Static validation first so problems surface per-field, not as one string.
    const v = await postJSON("/api/validate", readForm());
    if (!v.ok) {
      const n = showFieldErrors(v.fields);
      status.className = "status err";
      status.textContent = `✗ Fix ${n} field${n === 1 ? "" : "s"} highlighted above`;
      const firstBad = anchorFor(Object.keys(v.fields)[0]);
      if (firstBad && firstBad.scrollIntoView) firstBad.scrollIntoView({ block: "center" });
      return;
    }
    status.textContent = "Generating…";
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

// ---- Validate & test access ------------------------------------------------
async function onValidateAndCheck() {
  const btn = $("#check-btn");
  const panel = $("#check-panel");
  const status = $("#gen-status");
  btn.disabled = true;
  clearFieldErrors();
  panel.classList.remove("hidden");
  panel.innerHTML = '<div class="check-head">Validating fields…</div>';
  status.textContent = "";
  status.className = "status";
  try {
    const v = await postJSON("/api/validate", readForm());
    if (!v.ok) {
      const n = showFieldErrors(v.fields);
      panel.innerHTML = `<div class="check-head err">✗ ${n === 1 ? "1 field needs" : n + " fields need"} fixing before testing access.</div>`;
      const firstBad = anchorFor(Object.keys(v.fields)[0]);
      if (firstBad && firstBad.scrollIntoView) firstBad.scrollIntoView({ block: "center" });
      return;
    }
    panel.innerHTML = '<div class="check-head">Testing access to targets…</div>';
    const res = await postJSON("/api/check", readForm());
    renderChecks(res.results || []);
  } catch (err) {
    panel.innerHTML = `<div class="check-head err">✗ ${err.message}</div>`;
  } finally {
    btn.disabled = false;
  }
}

const CHECK_ICON = { pass: "✓", fail: "✗", warn: "!", skip: "–" };

function renderChecks(results) {
  const panel = $("#check-panel");
  const counts = { pass: 0, fail: 0, warn: 0, skip: 0 };
  results.forEach((r) => { counts[r.status] = (counts[r.status] || 0) + 1; });

  // Highlight fields tied to hard failures.
  results.forEach((r) => {
    if (r.status === "fail" && r.field) {
      const a = anchorFor(r.field);
      if (a && !a.classList.contains("invalid")) {
        a.classList.add("invalid");
        if (!(a.nextElementSibling && a.nextElementSibling.classList.contains("field-err"))) {
          const err = document.createElement("span");
          err.className = "field-err";
          err.textContent = r.detail;
          a.insertAdjacentElement("afterend", err);
        }
      }
    }
  });

  const head = counts.fail
    ? `<div class="check-head err">✗ ${counts.fail} target check${counts.fail === 1 ? "" : "s"} failed</div>`
    : counts.warn
    ? `<div class="check-head warn">⚠ Reachable with ${counts.warn} warning${counts.warn === 1 ? "" : "s"}</div>`
    : `<div class="check-head ok">✓ All target checks passed</div>`;

  const rows = results.map((r) =>
    `<div class="check-row ${r.status}">
       <span class="cdot ${r.status}">${CHECK_ICON[r.status] || "?"}</span>
       <span class="clabel">${escapeHtml(r.label)}</span>
       <span class="cdetail">${escapeHtml(r.detail)}</span>
     </div>`).join("");

  panel.innerHTML = head + rows;
}

function escapeHtml(s) {
  return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
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

  // Phases show generated files; if nothing has been generated yet, prompt.
  if (!state.generated) {
    const c = callout("Fill in the Configure step and click “Generate artifacts” — " +
      "this step’s files and commands will then appear here.", false);
    const link = document.createElement("button");
    link.className = "linkish";
    link.textContent = "Go to Configure";
    link.addEventListener("click", () => show("configure"));
    c.appendChild(document.createElement("br"));
    c.appendChild(link);
    panel.appendChild(c);
    return;
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
