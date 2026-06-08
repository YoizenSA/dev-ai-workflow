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
          return `
            <div class="feature-item">
              <div class="feature-item-left">
                <span class="status-icon ${f.status}">${fIcon}</span>
                <span class="feature-item-name">${escapeHtml(f.description || f.id)}</span>
              </div>
              <div>
                <span class="feature-status ${f.status}">${f.status}</span>
                <span class="feature-item-meta">${f.skillName || ''}</span>
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
      actionsHTML = `<button class="btn btn-danger" onclick="window.app.pauseMission('${mission.id}')">⏸ Pause</button>`;
    } else if (mission.status === 'paused') {
      actionsHTML = `<button class="btn btn-success" onclick="window.app.resumeMission('${mission.id}')">▶ Resume</button>`;
    }

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
        </div>
        ${actionsHTML ? `<div class="detail-actions">${actionsHTML}</div>` : ''}
      </div>
      <div id="milestones-container">${milestonesHTML}</div>
    `;
  }

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
    window.app = { showMission, showMissionsList, pauseMission, resumeMission, showKeyboardHelp };

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
