// ywai Mission Control — Vanilla JS
(() => {
  'use strict';

  /* ─── State ─── */
  const API_BASE = '/api';
  let missions = [];
  let currentMissionId = null;
  let ws = null;
  let wsReconnectTimer = null;
  let reconnectDelay = 1000;
  const MAX_RECONNECT_DELAY = 30000;
  let logLevelFilter = 'all';
  let timerInterval = null;
  const STATUS_ICONS = {
    planning: '📋',
    active: '▶',
    paused: '⏸',
    completed: '✅',
    failed: '❌',
    cancelled: '🚫',
    validating: '🔍',
    pending: '⏳',
    in_progress: '🔄',
  };

  /* ─── DOM References ─── */
  const $ = (id) => document.getElementById(id);
  const loadingState = $('loading-state');
  const errorState = $('error-state');
  const errorMessage = $('error-message');
  const emptyState = $('empty-state');
  const missionsList = $('missions-list');
  const missionsGrid = $('missions-grid');
  const missionDetail = $('mission-detail');
  const detailContent = $('detail-content');
  const notFoundState = $('not-found-state');
  const notFoundMessage = $('not-found-message');
  const connectionStatus = $('connection-status');
  const toastContainer = $('toast-container');

  /* ─── Toast Notifications ─── */
  function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.textContent = message;
    toastContainer.appendChild(toast);
    setTimeout(() => { toast.remove(); }, 4000);
  }

  /* ─── API Helpers ─── */
  async function apiFetch(path, options = {}) {
    const url = `${API_BASE}${path}`;
    const res = await fetch(url, {
      headers: { 'Content-Type': 'application/json', ...options.headers },
      ...options,
    });
    const data = await res.json().catch(() => null);
    if (!res.ok) {
      const errMsg = data?.error || `HTTP ${res.status}`;
      const err = new Error(errMsg);
      err.status = res.status;
      err.data = data;
      throw err;
    }
    return data;
  }

  /* ─── View Switching ─── */
  function showView(view) {
    [loadingState, errorState, emptyState, missionsList, missionDetail, notFoundState].forEach(el => el.classList.add('hidden'));
    if (view) view.classList.remove('hidden');
  }

  /* ─── Load Missions ─── */
  async function loadMissions() {
    showView(loadingState);
    try {
      const data = await apiFetch('/missions');
      missions = Array.isArray(data) ? data : [];
      if (missions.length === 0) {
        showView(emptyState);
      } else {
        renderMissionsList();
        showView(missionsList);
      }
    } catch (err) {
      errorMessage.textContent = err.message || 'Failed to load missions.';
      showView(errorState);
    }
  }

  /* ─── Render Missions List ─── */
  function renderMissionsList() {
    missionsGrid.innerHTML = missions.map(m => {
      const statusClass = m.status || 'pending';
      const created = new Date(m.createdAt).toLocaleDateString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
      const icon = STATUS_ICONS[m.status] || '📋';
      return `
        <div class="mission-card" onclick="window.app.showMission('${m.id}')">
          <div class="mission-card-left">
            <div class="mission-card-name">${escapeHtml(m.name || m.id)}</div>
            <div class="mission-card-meta">
              <span>${created}</span>
              <span>${m.featureCount || 0} features</span>
              <span>${m.milestoneCount || 0} milestones</span>
            </div>
          </div>
          <div class="mission-card-right">
            <span class="status-badge ${statusClass}">${icon} ${m.status}</span>
            <span class="feature-status ${statusClass}">${m.featureCount || 0}</span>
          </div>
        </div>
      `;
    }).join('');
  }

  /* ─── Show Mission Detail ─── */
  async function showMission(id) {
    if (!id) return;
    currentMissionId = id;
    showView(loadingState);
    try {
      const mission = await apiFetch(`/missions/${encodeURIComponent(id)}`);
      // Also fetch features
      let features = [];
      try {
        features = await apiFetch(`/missions/${encodeURIComponent(id)}/features`);
      } catch (e) { /* ignore */ }
      renderMissionDetail(mission, features);
      showView(missionDetail);
    } catch (err) {
      if (err.status === 404) {
        notFoundMessage.textContent = `Mission "${escapeHtml(id)}" was not found.`;
        showView(notFoundState);
      } else {
        showToast(`Failed to load mission: ${err.message}`, 'error');
        showMissionsList();
      }
    }
  }

  /* ─── Render Mission Detail ─── */
  function renderMissionDetail(mission, features) {
    const statusClass = mission.status || 'pending';
    const icon = STATUS_ICONS[mission.status] || '📋';
    const created = new Date(mission.createdAt).toLocaleString();
    const updated = new Date(mission.updatedAt).toLocaleString();
    const completed = mission.completedAt ? new Date(mission.completedAt).toLocaleString() : null;

    // Group features by milestone
    const milestones = {};
    (mission.milestones || []).forEach(m => { milestones[m.name] = { ...m, features: [] }; });
    (features || []).forEach(f => {
      const ms = f.milestone || 'default';
      if (!milestones[ms]) {
        milestones[ms] = { name: ms, description: '', features: [] };
      }
      milestones[ms].features.push(f);
    });
    // Also add features for 'default' if not already captured
    if (!mission.milestones || mission.milestones.length === 0) {
      (features || []).forEach(f => {
        const ms = f.milestone || 'default';
        if (!milestones[ms]) milestones[ms] = { name: ms, description: '', features: [] };
        milestones[ms].features.push(f);
      });
    }

    // Milestone sections HTML
    let milestonesHTML = '';
    Object.values(milestones).forEach(ms => {
      const featList = ms.features || [];
      const total = featList.length;
      const completedCount = featList.filter(f => f.status === 'completed').length;
      const pct = total > 0 ? Math.round((completedCount / total) * 100) : 0;
      const progressClass = pct === 100 ? 'completed' : (pct > 0 ? 'partial' : 'none');

      let featuresHTML = '';
      if (featList.length === 0) {
        featuresHTML = '<div class="empty-features">No features</div>';
      } else {
        featuresHTML = featList.map(f => {
          const fIcon = STATUS_ICONS[f.status] || '⏳';
          let featureActions = '';
          if (f.status === 'failed') {
            featureActions = `<button class="btn btn-sm btn-danger" onclick="event.stopPropagation();window.app.retryFeature('${mission.id}', '${f.id}')">↻ Retry</button>`;
          }
          return `
            <div class="feature-item">
              <div class="feature-item-left">
                <span class="status-icon ${f.status}">${fIcon}</span>
                <span class="feature-item-name">${escapeHtml(f.description || f.id)}</span>
              </div>
              <div class="feature-item-right">
                <span class="feature-status ${f.status}">${f.status}</span>
                <span class="feature-item-meta">${f.skillName || ''}</span>
                ${featureActions}
              </div>
            </div>
          `;
        }).join('');
      }

      milestonesHTML += `
        <div class="milestone-section">
          <div class="milestone-header">
            <span class="milestone-name">🏁 ${escapeHtml(ms.name)}</span>
            <div class="milestone-progress">
              <span>${completedCount}/${total}</span>
              <div class="progress-bar">
                <div class="progress-fill ${progressClass}" style="width:${pct}%"></div>
              </div>
              <span>${pct}%</span>
            </div>
          </div>
          <div class="feature-list">${featuresHTML}</div>
        </div>
      `;
    });

    // Determine action buttons based on status
    let actionsHTML = '';
    if (mission.status === 'active') {
      actionsHTML += `<button class="btn btn-danger" onclick="window.app.pauseMission('${mission.id}')">⏸ Pause</button>`;
      actionsHTML += `<button class="btn btn-danger" onclick="window.app.cancelMission('${mission.id}')">✕ Cancel</button>`;
    } else if (mission.status === 'paused') {
      actionsHTML += `<button class="btn btn-success" onclick="window.app.resumeMission('${mission.id}')">▶ Resume</button>`;
      actionsHTML += `<button class="btn btn-danger" onclick="window.app.cancelMission('${mission.id}')">✕ Cancel</button>`;
    }

    function renderValidation(vs) {
      if (!vs || vs.status === 'not_started') {
        return '<div class="validation-section"><h3>🔍 Validation</h3><p class="validation-none">Not yet started</p></div>';
      }
      const assertions = vs.assertions || [];
      const passed = assertions.filter(a => a.status === 'passed').length;
      const failed = assertions.filter(a => a.status === 'failed').length;
      const total = assertions.length;
      let rows = assertions.map(a => `
        <div class="assertion-item ${a.status}">
          <span class="assertion-icon">${a.status === 'passed' ? '✅' : a.status === 'failed' ? '❌' : '⏳'}</span>
          <span class="assertion-id">${escapeHtml(a.id)}</span>
          <span class="assertion-surface">${escapeHtml(a.surface || '')}</span>
          ${a.error ? `<span class="assertion-error">${escapeHtml(a.error)}</span>` : ''}
        </div>
      `).join('');
      return `
        <div class="validation-section">
          <h3>🔍 Validation</h3>
          <div class="validation-summary">
            <span class="validation-pass">✅ ${passed} passed</span>
            <span class="validation-fail">❌ ${failed} failed</span>
            <span class="validation-total">📊 ${total} total</span>
          </div>
          <div class="assertion-list">${rows}</div>
        </div>
      `;
    }

    // Start elapsed timer
    if (timerInterval) clearInterval(timerInterval);
    timerInterval = setInterval(() => updateElapsedTimer(mission), 1000);

    // Helper to get elapsed time string
    function getElapsed(startDate) {
      const diff = Date.now() - new Date(startDate).getTime();
      if (diff < 0) return '0m 0s';
      const seconds = Math.floor(diff / 1000);
      const minutes = Math.floor(seconds / 60);
      const hours = Math.floor(minutes / 60);
      const days = Math.floor(hours / 24);
      if (days > 0) return `${days}d ${hours % 24}h`;
      if (hours > 0) return `${hours}h ${minutes % 60}m`;
      return `${minutes}m ${seconds % 60}s`;
    }

    const elapsedStart = mission.createdAt;
    const elapsedStr = getElapsed(elapsedStart);

    // Render logs with filter
    function renderLogs(logsArray, filterLevel) {
      if (!logsArray || logsArray.length === 0) {
        return '<div class="logs-empty">No logs available</div>';
      }
      const filtered = filterLevel === 'all' ? logsArray : logsArray.filter(l => {
        const level = (l.level || 'info').toLowerCase();
        return level === filterLevel;
      });
      if (filtered.length === 0) {
        return `<div class="logs-empty">No ${filterLevel} logs</div>`;
      }
      return filtered.map(l => {
        const ts = l.timestamp ? new Date(l.timestamp).toLocaleTimeString() : '';
        const level = (l.level || 'info').toLowerCase();
        return `<div class="log-line log-${level}"><span class="log-ts">${ts}</span><span class="log-level">[${level}]</span><span class="log-msg">${escapeHtml(l.message || l.content || '')}</span></div>`;
      }).join('');
    }

    const logFilterButtons = `
      <div class="log-filters">
        <span class="log-filters-label">Log Level:</span>
        <button class="log-filter-btn ${logLevelFilter === 'all' ? 'active' : ''}" data-level="all" onclick="window.app.setLogFilter('all')">All</button>
        <button class="log-filter-btn ${logLevelFilter === 'info' ? 'active' : ''}" data-level="info" onclick="window.app.setLogFilter('info')">Info</button>
        <button class="log-filter-btn ${logLevelFilter === 'warn' ? 'active' : ''}" data-level="warn" onclick="window.app.setLogFilter('warn')">Warn</button>
        <button class="log-filter-btn ${logLevelFilter === 'error' ? 'active' : ''}" data-level="error" onclick="window.app.setLogFilter('error')">Error</button>
      </div>
    `;

    detailContent.innerHTML = `
      <div class="detail-header">
        <div class="detail-title">
          <span>${icon}</span>
          <span>${escapeHtml(mission.name || mission.id)}</span>
          <span class="status-badge ${statusClass}">${mission.status}</span>
        </div>
        <div class="detail-meta">
          <div class="detail-meta-item"><span class="detail-meta-label">ID:</span> ${escapeHtml(mission.id)}</div>
          <div class="detail-meta-item"><span class="detail-meta-label">Created:</span> ${created}</div>
          <div class="detail-meta-item"><span class="detail-meta-label">Updated:</span> ${updated}</div>
          ${completed ? `<div class="detail-meta-item"><span class="detail-meta-label">Completed:</span> ${completed}</div>` : ''}
          <div class="detail-meta-item"><span class="detail-meta-label">Features:</span> ${(features || []).length}</div>
          <div class="detail-meta-item"><span class="detail-meta-label timer-label">⏱ Elapsed:</span> <span id="elapsed-timer">${elapsedStr}</span></div>
        </div>
        ${actionsHTML ? `<div class="detail-actions">${actionsHTML}</div>` : ''}
      </div>
      <div id="milestones-container">${milestonesHTML}</div>
      <div id="validation-container"></div>
      <div id="logs-section">
        <h3 style="margin:16px 0 8px;font-size:15px;font-weight:600;">📋 Logs</h3>
        ${logFilterButtons}
        <div id="logs-container"><div class="logs-empty">Loading logs...</div></div>
      </div>
    `;

    // Load validation state asynchronously
    loadValidation(mission.id).then(vs => {
      const vc = document.getElementById('validation-container');
      if (vc && vs) {
        vc.innerHTML = renderValidation(vs);
      }
    });

    // Load and render logs for all features
    loadFeatureLogsForMission(mission.id, features);
  }

  // Timer update helper
  function updateElapsedTimer(mission) {
    const el = document.getElementById('elapsed-timer');
    if (!el) return;
    const diff = Date.now() - new Date(mission.createdAt).getTime();
    if (diff < 0) { el.textContent = '0m 0s'; return; }
    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);
    if (days > 0) el.textContent = `${days}d ${hours % 24}h`;
    else if (hours > 0) el.textContent = `${hours}h ${minutes % 60}m`;
    else el.textContent = `${minutes}m ${seconds % 60}s`;
  }

  // Load logs for all features in a mission
  async function loadFeatureLogsForMission(missionId, features) {
    if (!features || features.length === 0) {
      const lc = document.getElementById('logs-container');
      if (lc) lc.innerHTML = '<div class="logs-empty">No logs available (no features)</div>';
      return;
    }
    try {
      const allLogs = [];
      for (const f of features) {
        try {
          const data = await loadFeatureLogs(missionId, f.id);
          if (data && data.content) {
            // Parse log content - split by newlines and determine level
            const lines = data.content.split('\n').filter(l => l.trim());
            lines.forEach(line => {
              let level = 'info';
              const lower = line.toLowerCase();
              if (lower.includes('[error]') || lower.includes('error:')) level = 'error';
              else if (lower.includes('[warn]') || lower.includes('warn:') || lower.includes('warning:')) level = 'warn';
              allLogs.push({
                featureId: f.id,
                level: level,
                message: line,
                timestamp: data.timestamp || null,
              });
            });
          }
        } catch (e) { /* skip features without logs */ }
      }
      const lc = document.getElementById('logs-container');
      if (lc) {
        lc.innerHTML = renderLogs(allLogs, logLevelFilter);
      }
    } catch (e) {
      const lc = document.getElementById('logs-container');
      if (lc) lc.innerHTML = '<div class="logs-empty">Failed to load logs</div>';
    }
  }

  // Set log level filter
  function setLogFilter(level) {
    logLevelFilter = level;
    // Update active button styling
    document.querySelectorAll('.log-filter-btn').forEach(btn => {
      btn.classList.toggle('active', btn.dataset.level === level);
    });
    // Re-render logs with new filter
    const mission = missions.find(m => m.id === currentMissionId);
    if (mission) {
      apiFetch(`/missions/${encodeURIComponent(currentMissionId)}/features`).then(features => {
        loadFeatureLogsForMission(currentMissionId, features || []);
      }).catch(() => {});
    }
  }

  // setLogFilter is exposed via init()

  /* ─── Pause / Resume ─── */
  async function pauseMission(id) {
    try {
      const result = await apiFetch(`/missions/${encodeURIComponent(id)}/pause`, { method: 'POST' });
      showToast('Mission paused', 'success');
      // Re-render detail with updated mission
      if (result) {
        const features = await apiFetch(`/missions/${encodeURIComponent(id)}/features`).catch(() => []);
        renderMissionDetail(result, features);
      }
    } catch (err) {
      showToast(`Failed to pause: ${err.message}`, 'error');
    }
  }

  async function resumeMission(id) {
    try {
      const result = await apiFetch(`/missions/${encodeURIComponent(id)}/resume`, { method: 'POST' });
      showToast('Mission resumed', 'success');
      if (result) {
        const features = await apiFetch(`/missions/${encodeURIComponent(id)}/features`).catch(() => []);
        renderMissionDetail(result, features);
      }
    } catch (err) {
      showToast(`Failed to resume: ${err.message}`, 'error');
    }
  }

  /* ─── Cancel Mission ─── */
  async function cancelMission(id) {
    if (id && !confirm('Are you sure you want to cancel this mission?')) return;
    try {
      const result = await apiFetch(`/missions/${encodeURIComponent(id)}/cancel`, { method: 'POST' });
      showToast('Mission cancelled', 'success');
      if (currentMissionId === id && result) {
        const features = await apiFetch(`/missions/${encodeURIComponent(id)}/features`).catch(() => []);
        renderMissionDetail(result, features);
      } else {
        loadMissions();
      }
    } catch (err) {
      showToast(`Failed to cancel: ${err.message}`, 'error');
    }
  }

  /* ─── Retry Feature ─── */
  async function retryFeature(missionId, featureId) {
    if (!confirm(`Retry feature "${featureId}"?`)) return;
    try {
      const result = await apiFetch(`/missions/${encodeURIComponent(missionId)}/features/${encodeURIComponent(featureId)}/retry`, { method: 'POST' });
      showToast('Feature queued for retry', 'success');
      if (currentMissionId === missionId && result) {
        const features = await apiFetch(`/missions/${encodeURIComponent(missionId)}/features`).catch(() => []);
        renderMissionDetail(result, features);
      }
    } catch (err) {
      showToast(`Failed to retry: ${err.message}`, 'error');
    }
  }

  /* ─── Validation & Logs ─── */
  async function loadValidation(missionId) {
    try {
      return await apiFetch(`/missions/${encodeURIComponent(missionId)}/validation`);
    } catch (e) { return null; }
  }

  async function loadFeatureLogs(missionId, featureId) {
    try {
      return await apiFetch(`/missions/${encodeURIComponent(missionId)}/features/${encodeURIComponent(featureId)}/logs`);
    } catch (e) { return null; }
  }

  /* ─── Navigation ─── */
  function showMissionsList() {
    currentMissionId = null;
    loadMissions();
  }

  /* ─── Keyboard Help ─── */
  function showKeyboardHelp() {
    $('keyboard-help').classList.remove('hidden');
  }

  /* ─── WebSocket ─── */
  function connectWebSocket() {
    if (ws && ws.readyState === WebSocket.OPEN) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    try {
      ws = new WebSocket(wsUrl);
    } catch (err) {
      updateConnectionStatus(false);
      scheduleReconnect();
      return;
    }

    ws.onopen = () => {
      updateConnectionStatus(true);
      reconnectDelay = 1000;
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        handleWebSocketMessage(msg);
      } catch (e) { /* ignore parse errors */ }
    };

    ws.onclose = () => {
      updateConnectionStatus(false);
      scheduleReconnect();
    };

    ws.onerror = () => {
      // onclose will fire after this
    };
  }

  function handleWebSocketMessage(msg) {
    if (!msg || !msg.type) return;

    switch (msg.type) {
      case 'initial_state':
        // Refresh missions list with initial state
        break;
      case 'mission_status_changed':
        showToast(`Mission status changed: ${msg.payload?.status}`, 'info');
        // Refresh current view
        if (currentMissionId) {
          showMission(currentMissionId);
        } else {
          loadMissions();
        }
        break;
      case 'feature_status_changed':
        showToast(`Feature ${msg.payload?.featureId} → ${msg.payload?.status}`, 'info');
        if (currentMissionId === msg.payload?.missionId) {
          showMission(currentMissionId);
        }
        break;
      case 'log_update':
        // Log updates - refresh if viewing a mission
        if (currentMissionId) {
          showMission(currentMissionId);
        }
        break;
      default:
        break;
    }
  }

  function updateConnectionStatus(connected) {
    if (connected) {
      connectionStatus.textContent = '● Connected';
      connectionStatus.className = 'connection-status connected';
    } else {
      connectionStatus.textContent = '○ Disconnected';
      connectionStatus.className = 'connection-status disconnected';
    }
  }

  function scheduleReconnect() {
    if (wsReconnectTimer) clearTimeout(wsReconnectTimer);
    wsReconnectTimer = setTimeout(() => {
      connectWebSocket();
      reconnectDelay = Math.min(reconnectDelay * 1.5, MAX_RECONNECT_DELAY);
    }, reconnectDelay);
  }

  /* ─── Helpers ─── */
  function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }

  /* ─── Init ─── */
  function init() {
    // Expose functions globally for inline onclick handlers
    window.app = { showMission, showMissionsList, pauseMission, resumeMission, cancelMission, retryFeature, loadValidation, loadFeatureLogs, showKeyboardHelp, setLogFilter, loadFeatureLogsForMission, updateElapsedTimer };

    loadMissions();
    connectWebSocket();

    // Keyboard shortcuts
    document.addEventListener('keydown', (e) => {
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;
      switch (e.key) {
        case '?':
          showKeyboardHelp();
          break;
        case 'Escape':
          if (!$('keyboard-help').classList.contains('hidden')) {
            $('keyboard-help').classList.add('hidden');
          } else if (currentMissionId) {
            showMissionsList();
          }
          break;
        case 'r':
          if (currentMissionId) {
            showMission(currentMissionId);
          } else {
            loadMissions();
          }
          break;
      }
    });
  }

  // Start when DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
