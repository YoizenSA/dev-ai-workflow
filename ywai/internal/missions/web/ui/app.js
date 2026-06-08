/* ─── State ─── */
const state = {
  view: 'projects',
  projects: [],
  missions: [],
  currentProject: null,
  currentMission: null,
  currentMissionFeatures: [],
  currentMissionLogs: [],
  proposedPlan: null,
  missionGoals: {},
  filters: { status: 'all' },
  ws: null,
  wsReconnectTimer: null
};

/* ─── DOM shortcuts ─── */
const $ = (id) => document.getElementById(id);
const $$ = (sel) => document.querySelectorAll(sel);
const esc = (s) => {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
};

/* ─── API helper ─── */
async function apiFetch(url, options = {}) {
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options
  });
  const text = await res.text();
  if (!res.ok) {
    let msg;
    try { msg = JSON.parse(text).error || text; } catch { msg = text || res.statusText; }
    throw new Error(msg);
  }
  try { return JSON.parse(text); } catch { return text; }
}

/* ─── Toast notifications ─── */
function toast(msg, type = 'info') {
  const c = $('toast-container');
  const el = document.createElement('div');
  el.className = `toast toast-${type}`;
  el.textContent = msg;
  c.appendChild(el);
  setTimeout(() => { el.style.opacity = '0'; el.style.transition = 'opacity 0.3s'; setTimeout(() => el.remove(), 300); }, 3000);
}

/* ─── Mission count helper ─── */
function countMissions(projectName) {
    if (!state.missions || !Array.isArray(state.missions)) return 0;
    return state.missions.filter(m => m.project === projectName).length;
}

/* ─── Modal ─── */
function showModal(content, wide = false) {
  const overlay = $('modal-overlay');
  const container = $('modal-content');
  container.className = 'modal-content' + (wide ? ' wide' : '');
  container.innerHTML = content;
  overlay.classList.remove('hidden');
}

function hideModal(e) {
  if (e && e.target !== $('modal-overlay')) return;
  $('modal-overlay').classList.add('hidden');
}

/* ─── Navigation ─── */
function navigate(view, params = {}) {
  state.view = view;
  state.navParams = params;

  // Update URL hash for browser history
  const hash = view + (params.projectName ? '/' + encodeURIComponent(params.projectName) : '') + (params.missionId ? '/' + encodeURIComponent(params.missionId) : '');
  history.pushState({ view, params }, '', '#' + hash);

  $$('.view').forEach(el => el.classList.remove('active'));
  const target = $('view-' + view);
  if (target) target.classList.add('active');

  $$('.nav-links li').forEach(li => li.classList.remove('active'));
  const navMap = { projects: 'projects', 'project-detail': 'projects', 'new-mission': 'projects', 'plan-review': 'projects', 'mission-detail': 'missions', missions: 'missions' };
  const nav = navMap[view] || 'projects';
  document.querySelector(`.nav-links li[data-view="${nav}"]`)?.classList.add('active');

  switch (view) {
    case 'projects': renderProjects(); break;
    case 'project-detail': renderProjectDetail(params.projectName); break;
    case 'new-mission': renderNewMission(params.projectName); break;
    case 'plan-review': renderPlanReview(); break;
    case 'mission-detail': renderMissionDetail(params.missionId); break;
    case 'missions': renderMissionsDashboard(); refreshMissionsList(); break;
  }
}

/* ─── WebSocket ─── */
function connectWebSocket() {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const url = `${proto}//${location.host}/ws`;
  const statusEl = $('ws-status');

  function setStatus(cls, label) {
    statusEl.className = `ws-status ${cls}`;
    statusEl.querySelector('.ws-label').textContent = label;
  }

  if (state.ws && state.ws.readyState === WebSocket.OPEN) return;

  try {
    setStatus('ws-connecting', 'Connecting...');
    const ws = new WebSocket(url);

    ws.onopen = () => {
      setStatus('ws-connected', 'Connected');
      state.ws = ws;
      if (state.wsReconnectTimer) { clearTimeout(state.wsReconnectTimer); state.wsReconnectTimer = null; }
    };

    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data);
        handleWsEvent(msg);
      } catch { /* ignore malformed */ }
    };

    ws.onclose = () => {
      setStatus('ws-disconnected', 'Disconnected');
      state.ws = null;
      state.wsReconnectTimer = setTimeout(connectWebSocket, 3000);
    };

    ws.onerror = () => { ws.close(); };
  } catch {
    setStatus('ws-disconnected', 'Disconnected');
    state.wsReconnectTimer = setTimeout(connectWebSocket, 3000);
  }
}

function handleWsEvent(msg) {
  const { type, missionId, featureId, status, error } = msg;

  if (type === 'mission_status_changed' || type === 'mission_update') {
    if (state.currentMission?.id === missionId) {
      state.currentMission.status = status || state.currentMission.status;
    }
    refreshMissionsList();

    // In-place update: update the status badge and action buttons without full re-render
    if (state.view === 'mission-detail' && state.currentMission?.id === missionId) {
      const st = (status || 'pending').toLowerCase().replace(/\s+/g, '_');
      const statusBadge = document.querySelector('.detail-header .status-badge');
      if (statusBadge) {
        statusBadge.className = `status-badge status-${st}`;
        statusBadge.innerHTML = `<span class="status-dot"></span> ${st.replace(/_/g, ' ')}`;
      }
      // Update action buttons visibility based on status
      updateMissionActions(missionId, status);
      return;
    }
  }

  if (type === 'feature_status_changed') {
    if (state.currentMission?.id === missionId) {
      const f = state.currentMissionFeatures.find(x => (x.id || x.feature_id) === featureId);
      if (f) f.status = status;
    }

    // In-place update: update the specific feature's status badge without full re-render
    if (state.view === 'mission-detail' && state.currentMission?.id === missionId) {
      const st = (status || 'pending').toLowerCase().replace(/\s+/g, '_');
      const featureRow = document.querySelector(`.feature-row[data-feature-id="${CSS.escape(featureId)}"]`);
      if (featureRow) {
        const statusEl = featureRow.querySelector('.feature-status');
        if (statusEl) {
          statusEl.className = `feature-status status-${st}`;
          statusEl.innerHTML = `<span class="status-dot"></span> ${st.replace(/_/g, ' ')}`;
        }
        // Show retry button for failed status
        const retryBtn = featureRow.querySelector('.btn-retry');
        if (retryBtn) retryBtn.style.display = st === 'failed' ? '' : 'none';
      }
      // If feature is expanded, clear stale logs when going to in_progress
      if (status === 'in_progress') {
        const logContainer = document.getElementById(`logs-${CSS.escape(featureId)}`);
        if (logContainer && logContainer.querySelector('.log-entry')) {
          logContainer.innerHTML = '';
        }
      }
      return;
    }
  }

  if (type === 'log_update') {
    if (state.currentMission?.id === missionId) {
      // Append log line to the feature's log container in real-time
      const logContainer = document.getElementById(`logs-${featureId}`);
      if (logContainer) {
        // Remove the empty state placeholder if present
        const empty = logContainer.querySelector('.logs-empty');
        if (empty) empty.remove();

        const lineEl = document.createElement('div');
        lineEl.className = 'log-entry log-stream';
        lineEl.textContent = line;
        logContainer.appendChild(lineEl);
        logContainer.scrollTop = logContainer.scrollHeight;
      }
      // Store in state for persistence (used when re-expanding)
      if (!state.currentMissionLogs) state.currentMissionLogs = {};
      if (!state.currentMissionLogs[featureId]) state.currentMissionLogs[featureId] = [];
      state.currentMissionLogs[featureId].push({ message: line, timestamp: msg.timestamp || Date.now(), level: 'info' });
    }
  }

  if (type === 'mission_log') {
    if (state.currentMission?.id === missionId) {
      if (state.view === 'mission-detail') {
        loadMissionLogs(missionId, featureId);
      }
    }
  }

  if (type === 'mission_error' || type === 'error') {
    toast(`Mission ${missionId}: ${error || msg.message || 'Unknown error'}`, 'error');
  }

  if (type === 'feature_update') {
    if (state.currentMission?.id === missionId && state.view === 'mission-detail') {
      loadMission(missionId, true);
    }
  }
}

/* ─── Data Loading ─── */
async function loadProjects() {
  try {
    const data = await apiFetch('/api/projects');
    state.projects = data.projects || [];
    const act = $(`view-projects`);
    if (act && act.classList.contains('active')) renderProjects();
  } catch (e) {
    toast('Failed to load projects: ' + e.message, 'error');
  }
}

async function loadMissions() {
  try {
    const data = await apiFetch('/api/missions');
    state.missions = data.missions || [];
    const act = $(`view-missions`);
    if (act && act.classList.contains('active')) renderMissionsDashboard();
  } catch (e) {
    toast('Failed to load missions: ' + e.message, 'error');
  }
}

async function refreshMissionsList() {
  try {
    const data = await apiFetch('/api/missions');
    state.missions = data.missions || [];
    if ($(`view-missions`).classList.contains('active')) renderMissionsDashboard();
  } catch { /* silent */ }
}

async function loadMission(missionId, silent = false) {
  try {
    const data = await apiFetch(`/api/missions/${missionId}`);
    state.currentMission = data.mission || data;
    state.currentMissionFeatures = data.features || data.mission?.features || [];
    if ($(`view-mission-detail`).classList.contains('active')) renderMissionDetail(missionId);
  } catch (e) {
    if (!silent) toast('Failed to load mission: ' + e.message, 'error');
  }
}

async function loadMissionLogs(missionId, featureId) {
  try {
    const url = `/api/missions/${missionId}/logs` + (featureId ? `?featureId=${featureId}` : '');
    const data = await apiFetch(url);
    state.currentMissionLogs = data.logs || data || [];
  } catch { /* silent */ }
}

/* ================================================================
   VIEWS
   ================================================================ */

/* ─── Projects List View ─── */
function renderProjects() {
  const container = $(`view-projects`);
  container.innerHTML = `
    <div class="page-header">
      <h1>Projects</h1>
      <div class="header-actions">
        <button class="btn btn-primary" onclick="showNewProjectModal()">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
          New Project
        </button>
      </div>
    </div>
    <div id="projects-list"></div>
  `;
  renderProjectsList();
}

function renderProjectsList() {
  const list = $('projects-list');
  if (!state.projects.length) {
    list.innerHTML = `
      <div class="empty-state">
        <div class="empty-state-icon">📁</div>
        <h3>No projects yet</h3>
        <p>Create your first project to get started with Mission Control.</p>
        <button class="btn btn-primary" onclick="showNewProjectModal()">Create Project</button>
      </div>`;
    return;
  }
  list.innerHTML = `<div class="card-grid">${state.projects.map(p => `
    <div class="card card-clickable project-card" onclick="navigate('project-detail',{projectName:'${esc(p.name)}'})">
      <div class="project-card-name">${esc(p.name)}</div>
      <div class="project-card-path">${esc(p.path)}</div>
      ${p.description ? `<div class="project-card-desc">${esc(p.description)}</div>` : ''}
      <div class="project-card-meta">
        <span class="project-card-count">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/><polyline points="12,8 16,12 12,16"/></svg>
          ${countMissions(p.name)} missions
        </span>
        <span>${formatDate(p.createdAt || p.created_at)}</span>
      </div>
    </div>`).join('')}</div>`;
}

/* ─── New Project Modal ─── */
let fileBrowserContext = null;

function showNewProjectModal() {
  const content = `
    <div class="modal-header">
      <h2>New Project</h2>
      <button class="modal-close" onclick="hideModal()">&times;</button>
    </div>
    <div class="modal-body">
      <div class="form-group">
        <label class="form-label" for="np-name">Name</label>
        <input id="np-name" class="form-input" type="text" placeholder="my-project" autofocus>
      </div>
      <div class="form-group">
        <label class="form-label" for="np-path">Path</label>
        <div class="form-input-path">
          <input id="np-path" class="form-input" type="text" placeholder="/path/to/project">
          <button class="btn" onclick="showFileBrowser('np-path')">Browse</button>
        </div>
      </div>
      <div class="form-group">
        <label class="form-label" for="np-desc">Description (optional)</label>
        <input id="np-desc" class="form-input" type="text" placeholder="A short description">
      </div>
    </div>
    <div class="modal-footer">
      <button class="btn btn-ghost" onclick="hideModal()">Cancel</button>
      <button class="btn btn-primary" onclick="createProject()">Create Project</button>
    </div>
  `;
  showModal(content);
  setTimeout(() => $('np-name')?.focus(), 100);
}

async function createProject() {
  const name = $('np-name')?.value.trim();
  const path = $('np-path')?.value.trim();
  const description = $('np-desc')?.value.trim();
  if (!name) { toast('Project name is required', 'error'); return; }
  if (!path) { toast('Project path is required', 'error'); return; }

  try {
    await apiFetch('/api/projects', {
      method: 'POST',
      body: JSON.stringify({ name, path, description })
    });
    hideModal();
    toast('Project created successfully', 'success');
    await loadProjects();
  } catch (e) {
    toast('Failed to create project: ' + e.message, 'error');
  }
}

function deleteProject(name) {
  if (!confirm(`Delete project "${name}"?`)) return;
  apiFetch(`/api/projects/${encodeURIComponent(name)}`, { method: 'DELETE' })
    .then(() => { toast('Project deleted', 'success'); loadProjects(); navigate('projects'); })
    .catch(e => toast('Failed to delete: ' + e.message, 'error'));
}

/* ─── File Browser Modal ─── */
function showFileBrowser(inputId) {
  fileBrowserContext = inputId;
  const currentPath = $(inputId)?.value || process.cwd ? '' : '';
  openFileBrowser(currentPath || '/');
}

let fbCurrentPath = '/';

function openFileBrowser(path) {
  fbCurrentPath = path;
  const content = `
    <div class="modal-header">
      <h2>Browse Files</h2>
      <button class="modal-close" onclick="hideModal()">&times;</button>
    </div>
    <div class="modal-body">
      <div class="file-breadcrumb" id="fb-breadcrumb"></div>
      <div class="file-list" id="fb-list">
        <div class="file-browser-loading">
          <div class="spinner"></div>
          <p>Loading...</p>
        </div>
      </div>
      <div class="file-browser-select">
        <button class="btn btn-primary" onclick="selectFileBrowserFolder()">Select This Folder</button>
      </div>
    </div>
  `;
  showModal(content, true);
  loadFileBrowser(path);
}

async function loadFileBrowser(path) {
  const list = $('fb-list');
  const breadcrumb = $('fb-breadcrumb');
  if (!list || !breadcrumb) return;

  // Breadcrumb
  const parts = path.split('/').filter(Boolean);
  let cum = '';
  const crumbs = [{ name: 'Root', path: '/' }];
  for (const p of parts) {
    cum += '/' + p;
    crumbs.push({ name: p, path: cum });
  }
  breadcrumb.innerHTML = crumbs.map((c, i) => {
    const isLast = i === crumbs.length - 1;
    return isLast
      ? `<span>${esc(c.name)}</span>`
      : `<a onclick="loadFileBrowser('${esc(c.path)}')">${esc(c.name)}</a><span>/</span>`;
  }).join('');

  list.innerHTML = `<div class="file-browser-loading"><div class="spinner"></div><p>Loading...</p></div>`;

  try {
    const data = await apiFetch(`/api/fs/browse?path=${encodeURIComponent(path)}`);
    const entries = data.entries || [];

    // Sort: dirs first
    const dirs = entries.filter(e => e.isDir);
    const files = entries.filter(e => !e.isDir);

    let html = '';
    if (path !== '/') {
      const parent = path.substring(0, path.lastIndexOf('/')) || '/';
      html += `<div class="file-entry is-dir" onclick="loadFileBrowser('${esc(parent)}')">
        <span class="file-entry-icon">..</span>
        <span class="file-entry-name"><em>Go up</em></span>
        <span class="file-entry-size"></span>
      </div>`;
    }

    for (const d of dirs) {
      const fullPath = path.endsWith('/') ? path + d.name : path + '/' + d.name;
      html += `<div class="file-entry is-dir" onclick="loadFileBrowser('${esc(fullPath)}')">
        <span class="file-entry-icon">📁</span>
        <span class="file-entry-name">${esc(d.name)}/</span>
        <span class="file-entry-size"></span>
      </div>`;
    }

    for (const f of files) {
      html += `<div class="file-entry is-file">
        <span class="file-entry-icon">📄</span>
        <span class="file-entry-name">${esc(f.name)}</span>
        <span class="file-entry-size">${formatSize(f.size)}</span>
      </div>`;
    }

    list.innerHTML = html;
  } catch (e) {
    list.innerHTML = `<div class="error-state"><div class="error-icon">⚠</div><p>Failed to load: ${esc(e.message)}</p></div>`;
  }
}

function selectFileBrowserFolder() {
  const input = $(fileBrowserContext);
  if (input) input.value = fbCurrentPath;
  hideModal();
}

function formatSize(bytes) {
  if (!bytes) return '';
  const units = ['B', 'KB', 'MB', 'GB'];
  let i = 0;
  let size = bytes;
  while (size >= 1024 && i < units.length - 1) { size /= 1024; i++; }
  return size.toFixed(1) + ' ' + units[i];
}

/* ─── Project Detail View ─── */
async function renderProjectDetail(projectName) {
  const container = $(`view-project-detail`);
  container.innerHTML = `<div class="loading-state"><div class="spinner"></div><p>Loading project...</p></div>`;

  try {
    const data = await apiFetch('/api/projects');
    const proj = (data.projects || []).find(p => p.name === projectName);
    if (!proj) {
      container.innerHTML = `<div class="error-state"><div class="error-icon">⚠</div><p>Project not found: ${esc(projectName)}</p><button class="btn" onclick="navigate('projects')">Back to Projects</button></div>`;
      return;
    }
    state.currentProject = proj;

    // Load missions for this project
    const mData = await apiFetch('/api/missions');
    const projMissions = (mData.missions || []).filter(m => m.project === projectName);

    const html = `
      <a class="back-link" onclick="navigate('projects')">← Back to Projects</a>
      <div class="detail-header">
        <div class="detail-header-top">
          <h1>${esc(proj.name)}</h1>
        </div>
        <div class="detail-meta">
          <span class="detail-meta-item">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>
            ${esc(proj.path)}
          </span>
          <span class="detail-meta-item">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="4" width="18" height="18" rx="2" ry="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg>
            ${formatDate(proj.createdAt || proj.created_at)}
          </span>
        </div>
        ${proj.description ? `<p style="color:var(--text-secondary);margin-bottom:16px;">${esc(proj.description)}</p>` : ''}
        <div class="detail-actions">
          <button class="btn btn-primary" onclick="navigate('new-mission',{projectName:'${esc(proj.name)}'})">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
            New Mission
          </button>
          <button class="btn btn-ghost btn-danger" onclick="deleteProject('${esc(proj.name)}')">Delete Project</button>
        </div>
      </div>
      <div>
        <h3 style="font-size:16px;font-weight:600;margin-bottom:12px;">Missions ${projMissions.length ? `(${projMissions.length})` : ''}</h3>
        <div id="project-missions">
          ${projMissions.length ? projMissions.map(m => renderMissionItem(m)).join('') : `
            <div class="empty-state" style="padding:40px 24px">
              <div class="empty-state-icon">🚀</div>
              <h3>No missions yet</h3>
              <p>Create a mission to start planning and building.</p>
              <button class="btn btn-primary" onclick="navigate('new-mission',{projectName:'${esc(proj.name)}'})">Create Mission</button>
            </div>`}
        </div>
      </div>`;
    container.innerHTML = html;
  } catch (e) {
    container.innerHTML = `<div class="error-state"><div class="error-icon">⚠</div><p>${esc(e.message)}</p><button class="btn" onclick="navigate('projects')">Back</button></div>`;
  }
}

function renderMissionItem(m) {
  const st = (m.status || 'pending').toLowerCase().replace(/\s+/g, '_');
  const icon = statusIcon(st);
  return `<div class="mission-item" onclick="navigate('mission-detail',{missionId:'${esc(m.id)}'})">
    <div class="mission-item-icon">${icon}</div>
    <div class="mission-item-body">
      <div class="mission-item-name">${esc(m.name || 'Unnamed Mission')}</div>
      <div class="mission-item-meta">
        <span class="status-badge status-${st}">
          <span class="status-dot"></span>
          ${st.replace(/_/g, ' ')}
        </span>
        <span>${m.featureCount || m.feature_count || 0} features</span>
        <span>${formatDate(m.createdAt || m.created_at)}</span>
      </div>
    </div>
    <div class="mission-item-right">
      <span style="font-size:12px;color:var(--text-muted)">${esc(m.project || '')}</span>
    </div>
  </div>`;
}

function statusIcon(status) {
  const map = {
    pending: '⏳',
    in_progress: '🔄',
    running: '▶️',
    completed: '✅',
    failed: '❌',
    cancelled: '⏹️',
    paused: '⏸️'
  };
  return map[status] || '📋';
}

/* ─── New Mission View ─── */
function renderNewMission(projectName) {
  const container = $(`view-new-mission`);
  container.innerHTML = `
    <a class="back-link" onclick="${state.currentProject ? `navigate('project-detail',{projectName:'${esc(state.currentProject.name)}'})` : `navigate('projects')`}">← Back</a>

    <div class="steps">
      <div class="step active">
        <span class="step-number">1</span>
        <span>Define Goal</span>
      </div>
      <div class="step-line"></div>
      <div class="step">
        <span class="step-number">2</span>
        <span>Review Plan</span>
      </div>
      <div class="step-line"></div>
      <div class="step">
        <span class="step-number">3</span>
        <span>Approve</span>
      </div>
    </div>

    <div class="card" style="max-width:640px;margin:0 auto;">
      <div class="form-group">
        <label class="form-label" for="mission-goal">Describe what you want to build</label>
        <textarea id="mission-goal" class="form-textarea" rows="6" placeholder="e.g. Build a REST API for user management with CRUD operations, authentication, and role-based access control...">${state.missionGoals[projectName || ''] || ''}</textarea>
      </div>
      <details style="margin-top:12px;font-size:0.9em;">
        <summary style="cursor:pointer;color:var(--text-muted);">Advanced options (model/agent)</summary>
        <div style="display:flex;gap:8px;margin-top:8px;">
          <div style="flex:1;">
            <label class="form-label" for="mission-model">Model</label>
            <input id="mission-model" class="form-input" type="text" placeholder="e.g. openai/gpt-4o" value="${state.missionModel || ''}">
          </div>
          <div style="flex:1;">
            <label class="form-label" for="mission-agent">Agent</label>
            <input id="mission-agent" class="form-input" type="text" placeholder="e.g. dev" value="${state.missionAgent || ''}">
          </div>
        </div>
      </details>
      <div class="form-actions" style="justify-content:center;">
        <button class="btn btn-ghost" onclick="navigate('${state.currentProject ? `project-detail',{projectName:'${esc(state.currentProject.name)}'}` : 'projects'}">Cancel</button>
        <button class="btn btn-primary" id="generate-plan-btn" onclick="generatePlan('${esc(projectName || '')}')">
          Generate Plan with AI
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="2" x2="12" y2="6"/><line x1="12" y1="18" x2="12" y2="22"/><line x1="4.93" y1="4.93" x2="7.76" y2="7.76"/><line x1="16.24" y1="16.24" x2="19.07" y2="19.07"/><line x1="2" y1="12" x2="6" y2="12"/><line x1="18" y1="12" x2="22" y2="12"/><line x1="4.93" y1="19.07" x2="7.76" y2="16.24"/><line x1="16.24" y1="7.76" x2="19.07" y2="4.93"/></svg>
        </button>
      </div>
      <div id="plan-error" class="error-state hidden"></div>
    </div>
  `;
}

async function generatePlan(projectName) {
  const goal = $('mission-goal')?.value.trim();
  if (!goal) { toast('Please describe what you want to build', 'error'); return; }

  state.missionGoals[projectName || ''] = goal;
  const btn = $('generate-plan-btn');
  if (btn) { btn.disabled = true; btn.innerHTML = '<span class="spinner"></span> Generating...'; }

  const errEl = $('plan-error');
  if (errEl) errEl.classList.add('hidden');

  const model = $('mission-model')?.value.trim() || '';
  const agent = $('mission-agent')?.value.trim() || '';
  state.missionModel = model;
  state.missionAgent = agent;

  try {
    const body = { goal };
    if (projectName) body.project = projectName;
    if (model) body.model = model;
    if (agent) body.agent = agent;

    const data = await apiFetch('/api/missions', {
      method: 'POST',
      body: JSON.stringify(body)
    });

    state.proposedPlan = data.plan || data;
    sessionStorage.setItem('proposedPlan', JSON.stringify(state.proposedPlan));
    navigate('plan-review', { projectName });
  } catch (e) {
    if (errEl) {
      errEl.innerHTML = `<div class="error-icon">⚠</div><p>${esc(e.message)}</p>`;
      errEl.classList.remove('hidden');
    }
    toast('Failed to generate plan: ' + e.message, 'error');
  } finally {
    if (btn) { btn.disabled = false; btn.innerHTML = 'Generate Plan with AI'; }
  }
}

/* ─── Plan Review View ─── */
function renderPlanReview() {
  const plan = state.proposedPlan;
  const container = $(`view-plan-review`);

  if (!plan) {
    navigate('new-mission', { projectName: state.currentProject?.name || '' });
    return;
  }

  const milestones = plan.milestones || plan.phases || [];
  const features = plan.features || [];

  container.innerHTML = `
    <a class="back-link" onclick="navigate('new-mission',{projectName:'${esc(state.currentProject?.name || '')}'})">← Back to Goal</a>

    <div class="steps">
      <div class="step completed">
        <span class="step-number">✓</span>
        <span>Define Goal</span>
      </div>
      <div class="step-line completed"></div>
      <div class="step active">
        <span class="step-number">2</span>
        <span>Review Plan</span>
      </div>
      <div class="step-line"></div>
      <div class="step">
        <span class="step-number">3</span>
        <span>Approve</span>
      </div>
    </div>

    <div class="plan-name">${esc(plan.name || plan.mission_name || 'Mission Plan')}</div>
    ${plan.description ? `<p class="plan-description" style="margin-top:8px;margin-bottom:24px;">${esc(plan.description)}</p>` : ''}

    ${milestones.length ? `
    <div class="plan-section">
      <h3>Milestones</h3>
      ${milestones.map((ms, i) => `
        <div class="plan-milestone">
          <h4>${esc(ms.name || `Milestone ${i + 1}`)}</h4>
          ${ms.description ? `<div class="plan-milestone-desc">${esc(ms.description)}</div>` : ''}
          ${ms.tasks || ms.features ? (ms.tasks || ms.features).map(f => `
            <div class="plan-feature">
              <div class="plan-feature-header">
                <span class="plan-feature-id">${esc(f.id || f.name || '')}</span>
                <span class="plan-feature-desc">${esc(f.description || f.title || '')}</span>
              </div>
              ${(f.preconditions || f.dependencies || []).length ? `
                <div class="plan-feature-meta">Depends on: ${(f.preconditions || f.dependencies || []).join(', ')}</div>
              ` : ''}
              ${f.expectedBehavior || f.assertions || f.validation ? `
                <div class="plan-feature-meta">Expected: ${esc(f.expectedBehavior || f.assertions || f.validation || '')}</div>
              ` : ''}
            </div>`).join('') : ''}
        </div>`).join('')}
    </div>` : ''}

    ${features.length ? `
    <div class="plan-section">
      <h3>Features</h3>
      ${features.map(f => `
        <div class="plan-feature">
          <div class="plan-feature-header">
            <span class="plan-feature-id">${esc(f.id || '')}</span>
            <span class="plan-feature-desc">${esc(f.description || f.title || '')}</span>
          </div>
        </div>`).join('')}
    </div>` : ''}

    <div class="plan-actions">
      <button class="btn btn-ghost" onclick="navigate('new-mission',{projectName:'${esc(state.currentProject?.name || '')}'})">Reject</button>
      <button class="btn btn-success" onclick="approvePlan()">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20,6 9,17 4,12"/></svg>
        Approve Plan
      </button>
    </div>
    <div id="approve-error" class="error-state hidden"></div>
  `;
}

async function approvePlan() {
  const plan = state.proposedPlan;
  if (!plan) {
    toast('No plan to approve', 'error');
    return;
  }

  const btn = document.querySelector('.plan-actions .btn-success');
  if (btn) { btn.disabled = true; btn.innerHTML = '<span class="spinner"></span> Approving...'; }

  try {
    const data = await apiFetch(`/api/missions/approve`, {
      method: 'POST',
      body: JSON.stringify({ plan })
    });
    const mission = data.mission || data;
    toast('Plan approved!', 'success');
    sessionStorage.removeItem('proposedPlan');
    state.currentMission = mission;
    navigate('mission-detail', { missionId: mission.id });
  } catch (e) {
    toast('Failed to approve plan: ' + e.message, 'error');
    const errEl = $('approve-error');
    if (errEl) {
      errEl.innerHTML = `<div class="error-icon">⚠</div><p>${esc(e.message)}</p>`;
      errEl.classList.remove('hidden');
    }
  } finally {
    if (btn) { btn.disabled = false; btn.innerHTML = 'Approve Plan'; }
  }
}

/* ─── Mission Detail View ─── */
async function renderMissionDetail(missionId) {
  const container = $(`view-mission-detail`);
  container.innerHTML = `<div class="loading-state"><div class="spinner"></div><p>Loading mission...</p></div>`;

  try {
    await loadMission(missionId, false);
  } catch { /* handled in loadMission */ }

  const mission = state.currentMission;
  if (!mission) {
    container.innerHTML = `<div class="error-state"><div class="error-icon">⚠</div><p>Mission not found</p><button class="btn" onclick="navigate('missions')">Back to Missions</button></div>`;
    return;
  }

  const st = (mission.status || 'pending').toLowerCase().replace(/\s+/g, '_');
  const features = state.currentMissionFeatures || [];
  const milestones = mission.milestones || [];

  // Calculate milestone progress
  const totalFeatures = features.length;
  const completedFeatures = features.filter(f => (f.status || '').toLowerCase() === 'completed').length;
  const pct = totalFeatures ? Math.round((completedFeatures / totalFeatures) * 100) : 0;

  // Determine which buttons to show
  const showRun = st === 'pending' || st === 'cancelled' || st === 'active' || st === 'planning';
  const showPause = st === 'running' || st === 'in_progress';
  const showResume = st === 'paused';
  const showCancel = st === 'running' || st === 'in_progress' || st === 'paused';

  container.innerHTML = `
    <a class="back-link" onclick="navigate('missions')">← All Missions</a>
    <div class="detail-header">
      <div class="detail-header-top">
        <h1>${esc(mission.name || 'Mission')}</h1>
        <span class="status-badge status-${st}">
          <span class="status-dot"></span>
          ${st.replace(/_/g, ' ')}
        </span>
        ${mission.project ? `<span class="project-badge">${esc(mission.project)}</span>` : ''}
      </div>
      <div class="detail-meta">
        <span class="detail-meta-item">ID: <code style="color:var(--accent-purple);font-family:var(--font-mono);font-size:12px;">${esc(mission.id)}</code></span>
        <span class="detail-meta-item">${features.length} features</span>
        <span class="detail-meta-item">${formatDate(mission.createdAt || mission.created_at)}</span>
      </div>
      <div class="detail-actions">
        ${showRun ? `<button class="btn btn-success" onclick="runMission('${esc(mission.id)}')"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5,3 19,12 5,21"/></svg> Run</button>` : ''}
        ${showPause ? `<button class="btn" onclick="pauseMission('${esc(mission.id)}')"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg> Pause</button>` : ''}
        ${showResume ? `<button class="btn btn-success" onclick="resumeMission('${esc(mission.id)}')"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5,3 19,12 5,21"/></svg> Resume</button>` : ''}
        ${showCancel ? `<button class="btn btn-ghost btn-danger" onclick="cancelMission('${esc(mission.id)}')"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> Cancel</button>` : ''}
      </div>
    </div>

    ${milestones.length ? `
    <div class="milestones-section">
      <h3>Milestones <span style="font-size:13px;font-weight:400;color:var(--text-muted);">${completedFeatures}/${totalFeatures} features</span></h3>
      <div class="milestone-progress">
        <span>${pct}%</span>
        <div class="milestone-bar">
          <div class="milestone-bar-fill" style="width:${pct}%"></div>
        </div>
      </div>
    </div>` : ''}

    <div class="features-section">
      <h3>Features</h3>
      <div class="features-table" id="features-table">
        ${features.length ? features.map((f, idx) => renderFeatureRow(f, idx)).join('') : `<div class="empty-state" style="padding:40px"><p>No features defined for this mission.</p></div>`}
      </div>
    </div>
  `;
}

function renderFeatureRow(f, idx) {
  const fst = (f.status || 'pending').toLowerCase().replace(/\s+/g, '_');
  const canRetry = fst === 'failed';
  const hasDetails = f.preconditions?.length || f.expectedBehavior || f.handoff;
  return `
    <div class="feature-row" data-feature-id="${esc(f.id || f.feature_id || idx)}">
      <div class="feature-row-header">
        <span class="feature-id">${esc(f.id || f.feature_id || `F${idx + 1}`)}</span>
        <span class="feature-desc">${esc(f.description || f.title || '')}</span>
        <span class="feature-status status-${fst}">
          <span class="status-dot"></span>
          ${fst.replace(/_/g, ' ')}
        </span>
        ${canRetry ? `<button class="btn btn-sm" onclick="event.stopPropagation();retryFeature('${esc(f.id || f.feature_id)}')">Retry</button>` : ''}
        <span class="feature-chevron">▶</span>
      </div>
      <div class="feature-row-body">
        ${hasDetails ? `
        <div class="feature-details-grid">
          ${f.preconditions?.length ? `<div class="feature-detail-item"><strong>Preconditions</strong>${f.preconditions.map(p => `<div>${esc(p)}</div>`).join('')}</div>` : ''}
          ${f.expectedBehavior ? `<div class="feature-detail-item"><strong>Expected Behavior</strong><div>${esc(f.expectedBehavior)}</div></div>` : ''}
          ${f.handoff ? `<div class="feature-detail-item full-width"><strong>Handoff</strong><div>${esc(f.handoff)}</div></div>` : ''}
          ${f.validation ? `<div class="feature-detail-item full-width"><strong>Validation</strong><div>${esc(f.validation)}</div></div>` : ''}
        </div>` : ''}
        <div class="logs-section">
          <div class="logs-header">
            <h4>Logs</h4>
            <div class="logs-filter" id="logs-filter-${esc(f.id || f.feature_id || idx)}">
              <button class="log-filter-btn active" data-level="all" onclick="filterLogs(this,'${esc(f.id || f.feature_id || idx)}')">All</button>
              <button class="log-filter-btn" data-level="info" onclick="filterLogs(this,'${esc(f.id || f.feature_id || idx)}')">Info</button>
              <button class="log-filter-btn" data-level="warn" onclick="filterLogs(this,'${esc(f.id || f.feature_id || idx)}')">Warn</button>
              <button class="log-filter-btn" data-level="error" onclick="filterLogs(this,'${esc(f.id || f.feature_id || idx)}')">Error</button>
            </div>
          </div>
          <div class="logs-container" id="logs-${esc(f.id || f.feature_id || idx)}" data-feature-id="${esc(f.id || f.feature_id || idx)}">
            <div class="logs-empty">Loading logs...</div>
          </div>
        </div>
      </div>
    </div>`;
}

/* ─── Feature interactions ─── */
function toggleFeature(header) {
  const row = header.closest('.feature-row');
  if (!row) return;
  const wasExpanded = row.classList.contains('expanded');

  // Close others
  $$('.feature-row.expanded').forEach(r => {
    if (r !== row) r.classList.remove('expanded');
  });

  if (!wasExpanded) {
    row.classList.add('expanded');
    // Load logs for this feature
    const fid = row.dataset.featureId;
    const missionId = state.currentMission?.id;
    if (missionId && fid) {
      const logsContainer = document.getElementById(`logs-${fid}`);
      if (logsContainer) {
        loadFeatureLogs(missionId, fid, logsContainer);
      }
    }
  } else {
    row.classList.remove('expanded');
  }
}

async function loadFeatureLogs(missionId, featureId, container) {
  try {
    const data = await apiFetch(`/api/missions/${missionId}/logs?featureId=${featureId}`);
    const logs = data.logs || data || [];
    if (!Array.isArray(logs) || !logs.length) {
      container.innerHTML = '<div class="logs-empty">No logs available</div>';
      return;
    }
    state.currentMissionLogs = logs;
    container.innerHTML = logs.map(l => {
      const ts = l.timestamp || l.time || l.ts || '';
      const level = (l.level || 'info').toLowerCase();
      const msg = l.message || l.msg || l.content || '';
      return `<div class="log-entry"><span class="log-ts">${esc(ts)}</span><span class="log-level ${level}">[${level}]</span><span class="log-msg">${esc(msg)}</span></div>`;
    }).join('');
  } catch {
    container.innerHTML = '<div class="logs-empty">Failed to load logs</div>';
  }
}

let logFilterState = {};

function filterLogs(btn, featureId) {
  const parent = btn.closest('.logs-filter');
  parent.querySelectorAll('.log-filter-btn').forEach(b => b.classList.remove('active'));
  btn.classList.add('active');
  const level = btn.dataset.level;

  const container = document.getElementById(`logs-${featureId}`);
  if (!container) return;

  const logs = state.currentMissionLogs || [];
  const filtered = level === 'all' ? logs : logs.filter(l => (l.level || 'info').toLowerCase() === level);

  if (!filtered.length) {
    container.innerHTML = '<div class="logs-empty">No logs for this level</div>';
    return;
  }

  container.innerHTML = filtered.map(l => {
    const ts = l.timestamp || l.time || l.ts || '';
    const lvl = (l.level || 'info').toLowerCase();
    const msg = l.message || l.msg || l.content || '';
    return `<div class="log-entry"><span class="log-ts">${esc(ts)}</span><span class="log-level ${lvl}">[${lvl}]</span><span class="log-msg">${esc(msg)}</span></div>`;
  }).join('');
}

/* ─── Mission Actions ─── */
async function runMission(id) {
  try {
    await apiFetch(`/api/missions/${id}/run`, { method: 'POST' });
    toast('Mission started!', 'success');
    loadMission(id, true);
  } catch (e) { toast('Failed to run: ' + e.message, 'error'); }
}

async function pauseMission(id) {
  try {
    await apiFetch(`/api/missions/${id}/pause`, { method: 'POST' });
    toast('Mission paused', 'info');
    loadMission(id, true);
  } catch (e) { toast('Failed to pause: ' + e.message, 'error'); }
}

async function resumeMission(id) {
  try {
    await apiFetch(`/api/missions/${id}/resume`, { method: 'POST' });
    toast('Mission resumed!', 'success');
    loadMission(id, true);
  } catch (e) { toast('Failed to resume: ' + e.message, 'error'); }
}

async function cancelMission(id) {
  if (!confirm('Cancel this mission?')) return;
  try {
    await apiFetch(`/api/missions/${id}/cancel`, { method: 'POST' });
    toast('Mission cancelled', 'info');
    loadMission(id, true);
  } catch (e) { toast('Failed to cancel: ' + e.message, 'error'); }
}

function updateMissionActions(missionId, status) {
  const actionsContainer = document.querySelector('.detail-actions');
  if (!actionsContainer) return;
  const st = (status || '').toLowerCase().replace(/\s+/g, '_');
  const showRun = st === 'pending' || st === 'draft';
  const showPause = st === 'running' || st === 'in_progress';
  const showResume = st === 'paused';
  const showCancel = st === 'running' || st === 'in_progress' || st === 'paused';
  actionsContainer.innerHTML = `
        ${showRun ? `<button class="btn btn-success" onclick="runMission('${esc(missionId)}')"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5,3 19,12 5,21"/></svg> Run</button>` : ''}
        ${showPause ? `<button class="btn btn-ghost" onclick="pauseMission('${esc(missionId)}')"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg> Pause</button>` : ''}
        ${showResume ? `<button class="btn btn-success" onclick="resumeMission('${esc(missionId)}')"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5,3 19,12 5,21"/></svg> Resume</button>` : ''}
        ${showCancel ? `<button class="btn btn-ghost btn-danger" onclick="cancelMission('${esc(missionId)}')"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg> Cancel</button>` : ''}
      `;
}

async function retryFeature(featureId) {
  const missionId = state.currentMission?.id;
  if (!missionId || !featureId) return;
  try {
    const data = await apiFetch(`/api/missions/${missionId}/features/${featureId}/retry`, { method: 'POST' });
    toast('Feature retry initiated', 'info');
    loadMission(missionId, true);
  } catch (e) { toast('Failed to retry: ' + e.message, 'error'); }
}

/* ─── Missions Dashboard ─── */
function renderMissionsDashboard() {
  const container = $(`view-missions`);
  container.innerHTML = `
    <div class="page-header">
      <h1>Missions</h1>
      <div class="header-actions">
        <div class="filter-bar">
          <select class="filter-dropdown" id="mission-status-filter" onchange="applyMissionFilter()">
            <option value="all">All Status</option>
            <option value="pending">Pending</option>
            <option value="in_progress">In Progress</option>
            <option value="running">Running</option>
            <option value="paused">Paused</option>
            <option value="completed">Completed</option>
            <option value="failed">Failed</option>
            <option value="cancelled">Cancelled</option>
          </select>
        </div>
      </div>
    </div>
    <div id="missions-dashboard-content"></div>
  `;

  renderMissionsContent();
}

function applyMissionFilter() {
  const sel = $('mission-status-filter');
  state.filters.status = sel?.value || 'all';
  renderMissionsContent();
}

function renderMissionsContent() {
  const content = $('missions-dashboard-content');
  if (!content) return;

  const filter = state.filters.status;
  let missions = state.missions || [];

  if (filter !== 'all') {
    missions = missions.filter(m => (m.status || '').toLowerCase().replace(/\s+/g, '_') === filter);
  }

  if (!missions.length) {
    content.innerHTML = `
      <div class="empty-state">
        <div class="empty-state-icon">🚀</div>
        <h3>No missions found</h3>
        <p>${filter !== 'all' ? 'No missions match the current filter.' : 'Create a project and start a mission to see it here.'}</p>
      </div>`;
    return;
  }

  // Group by project
  const grouped = {};
  for (const m of missions) {
    const proj = m.project || 'Uncategorized';
    if (!grouped[proj]) grouped[proj] = [];
    grouped[proj].push(m);
  }

  // Sort each group by date (newest first)
  for (const proj of Object.keys(grouped)) {
    grouped[proj].sort((a, b) => {
      const da = new Date(a.createdAt || a.created_at || 0);
      const db = new Date(b.createdAt || b.created_at || 0);
      return db - da;
    });
  }

  // Sort project groups by name
  const sortedProjects = Object.keys(grouped).sort();

  content.innerHTML = sortedProjects.map(proj => `
    <div class="mission-group">
      <div class="mission-group-title">
        <a href="#" onclick="navigate('project-detail',{projectName:'${esc(proj)}'})">${esc(proj)}</a>
        <span style="font-weight:400;color:var(--text-muted);font-size:13px;"> — ${grouped[proj].length} mission${grouped[proj].length !== 1 ? 's' : ''}</span>
      </div>
      ${grouped[proj].map(m => renderMissionItem(m)).join('')}
    </div>`).join('');

  content.innerHTML += `<div class="sort-info">Sorted by created date (newest first)</div>`;
}

/* ─── Utility ─── */
function formatDate(dateStr) {
  if (!dateStr) return '';
  try {
    const d = new Date(dateStr);
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit' });
  } catch { return dateStr; }
}

/* ─── Init ─── */
function init() {
  connectWebSocket();

  // Restore view from URL hash BEFORE loading data
  let initialView = 'projects';
  let initialParams = {};
  if (window.location.hash) {
    const hash = window.location.hash.slice(1);
    const parts = hash.split('/').filter(Boolean);
    if (parts[0]) {
      initialView = parts[0];
      if (parts[1]) initialParams.projectName = decodeURIComponent(parts[1]);
      if (parts[2]) initialParams.missionId = decodeURIComponent(parts[2]);
    }
  }

  // Restore proposedPlan from sessionStorage (survives page reload)
  const savedPlan = sessionStorage.getItem('proposedPlan');
  if (savedPlan) {
    try { state.proposedPlan = JSON.parse(savedPlan); } catch(e) { /* ignore */ }
  }

  // Load all data first, then navigate to restored view
  Promise.all([
    apiFetch('/api/projects').then(data => {
      state.projects = data.projects || [];
    }).catch(() => {}),
    apiFetch('/api/missions').then(data => {
      state.missions = data.missions || [];
    }).catch(() => {})
  ]).then(() => {
    navigate(initialView, initialParams);
  });

  // Handle browser back/forward
  window.addEventListener('popstate', (e) => {
    if (e.state && e.state.view) {
      navigate(e.state.view, e.state.params || {});
    }
  });

  // Keyboard shortcuts
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      if (!$('modal-overlay').classList.contains('hidden')) {
        hideModal();
        return;
      }
      if (state.view === 'project-detail' || state.view === 'mission-detail') {
        navigate('projects');
        return;
      }
    }
    if (e.key === 'n' && !e.ctrlKey && !e.metaKey) {
      const tag = document.activeElement?.tagName || '';
      if (tag !== 'INPUT' && tag !== 'TEXTAREA' && tag !== 'SELECT') {
        if (state.view === 'projects') {
          showNewProjectModal();
        }
      }
    }
  });

  // Add nav click handlers
  document.querySelectorAll('[data-nav]').forEach(el => {
    el.addEventListener('click', (e) => {
      e.preventDefault();
      const view = el.dataset.nav;
      if (view === 'projects') {
        navigate('projects');
      } else if (view === 'missions') {
        navigate('missions');
      }
    });
  });

  // Feature toggle via event delegation (handles dynamically rendered content)
  document.addEventListener('click', (e) => {
    const header = e.target.closest('.feature-row-header');
    if (header) {
      toggleFeature(header);
    }
  });
}

// Start
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}
