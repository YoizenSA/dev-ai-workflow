// ywai Kanban — Vanilla JS
(function () {
    'use strict';

    /* ─── Agent Config ─── */
    const AGENTS = {
        dev:       { icon: '🚧', label: 'Dev',       color: '#2ed573' },
        qa:        { icon: '🧪', label: 'QA',        color: '#ffa502' },
        architect: { icon: '📋', label: 'Architect', color: '#1e90ff' },
        reviewer:  { icon: '🔍', label: 'Reviewer',  color: '#9b59b6' },
        devops:    { icon: '🚀', label: 'DevOps',    color: '#ff4757' },
    };

    const COLUMNS = ['backlog', 'ready', 'in_progress', 'review', 'done'];
    const COLUMN_LABELS = {
        backlog: 'Backlog',
        ready: 'Ready',
        in_progress: 'In Progress',
        review: 'Review',
        done: 'Done',
    };

    /* ─── State ─── */
    let sessions = [];
    let currentSessionId = null;
    let boardData = null; // { session, columns: { backlog: [], ready: [], ... } }
    let ws = null;
    let wsReconnectTimer = null;

    /* ─── DOM refs ─── */
    const $ = (sel) => document.querySelector(sel);
    const $$ = (sel) => document.querySelectorAll(sel);

    const sessionSelect = $('#session-select');
    const newSessionBtn = $('#new-session-btn');
    const closeSessionBtn = $('#close-session-btn');
    const wsStatus = $('#ws-status');

    const sessionModal = $('#session-modal');
    const sessionForm = $('#session-form');
    const sessionProject = $('#session-project');
    const sessionGoal = $('#session-goal');
    const sessionModalCancel = $('#session-modal-cancel');
    const sessionModalCloseIcon = $('#session-modal-close-icon');

    const delegationModal = $('#delegation-modal');
    const delegationForm = $('#delegation-form');
    const delegationAgent = $('#delegation-agent');
    const delegationTask = $('#delegation-task');
    const delegationDeps = $('#delegation-deps');
    const delegationModalCancel = $('#delegation-modal-cancel');
    const delegationModalCloseIcon = $('#delegation-modal-close-icon');

    // Details Modal refs
    const detailsModal = $('#details-modal');
    const detailsForm = $('#details-form');
    const detailsId = $('#details-id');
    const detailsAgent = $('#details-agent');
    const detailsColumn = $('#details-column');
    const detailsStatus = $('#details-status');
    const detailsTaskSummary = $('#details-task-summary');
    const detailsBlockedCheckbox = $('#details-blocked-checkbox');
    const detailsBlockerContainer = $('#details-blocker-container');
    const detailsBlockerInput = $('#details-blocker-input');
    const detailsHandoffInput = $('#details-handoff-input');
    const detailsModalCancel = $('#details-modal-cancel');
    const detailsModalCloseIcon = $('#details-modal-close-icon');

    /* ─── Helpers ─── */
    function apiBase() {
        const loc = window.location;
        return `${loc.protocol}//${loc.host}`;
    }

    function wsBase() {
        const loc = window.location;
        const proto = loc.protocol === 'https:' ? 'wss:' : 'ws:';
        return `${proto}//${loc.host}`;
    }

    async function apiFetch(path, opts = {}) {
        const url = `${apiBase()}${path}`;
        const res = await fetch(url, {
            headers: { 'Content-Type': 'application/json', ...opts.headers },
            ...opts,
        });
        if (!res.ok) {
            const err = await res.json().catch(() => ({ error: res.statusText }));
            throw new Error(err.error || `HTTP ${res.status}`);
        }
        return res.json();
    }

    function showToast(msg) {
        let toast = document.querySelector('.toast');
        if (!toast) {
            toast = document.createElement('div');
            toast.className = 'toast';
            document.body.appendChild(toast);
        }
        toast.innerHTML = `🔔 &nbsp; ${msg}`;
        toast.classList.add('show');
        clearTimeout(toast._hide);
        toast._hide = setTimeout(() => toast.classList.remove('show'), 3500);
    }

    function formatTimeAgo(dateStr) {
        const diff = Date.now() - new Date(dateStr).getTime();
        const mins = Math.floor(diff / 60000);
        if (mins < 1) return 'just now';
        if (mins < 60) return `${mins}m ago`;
        const hours = Math.floor(mins / 60);
        if (hours < 24) return `${hours}h ago`;
        return `${Math.floor(hours / 24)}d ago`;
    }

    /* ─── WebSocket ─── */
    function connectWS() {
        if (ws && ws.readyState === WebSocket.OPEN) return;

        ws = new WebSocket(`${wsBase()}/api/events`);

        ws.onopen = () => {
            wsStatus.textContent = '● connected';
            wsStatus.className = 'ws-status connected';
            if (wsReconnectTimer) {
                clearTimeout(wsReconnectTimer);
                wsReconnectTimer = null;
            }
        };

        ws.onclose = () => {
            wsStatus.textContent = '● disconnected';
            wsStatus.className = 'ws-status disconnected';
            ws = null;
            // Reconnect after 3s
            wsReconnectTimer = setTimeout(connectWS, 3000);
        };

        ws.onerror = () => {
            ws.close();
        };

        ws.onmessage = (event) => {
            try {
                const update = JSON.parse(event.data);
                handleBoardUpdate(update);
            } catch (e) {
                console.error('WS parse error:', e);
            }
        };
    }

    function handleBoardUpdate(update) {
        const { type, payload } = update;

        switch (type) {
            case 'session.created':
                sessions.push(payload);
                renderSessionSelect();
                showToast(`Session created: <strong>${payload.goal}</strong>`);
                break;

            case 'session.status_changed':
                const idx = sessions.findIndex(s => s.id === payload.id);
                if (idx !== -1) sessions[idx] = payload;
                renderSessionSelect();
                updateCloseSessionButtonVisibility();
                if (currentSessionId === payload.id) {
                    refreshBoard();
                    if (payload.status === 'closed') {
                        showToast('Session closed successfully');
                    }
                }
                break;

            case 'delegation.created':
            case 'delegation.status_changed':
                if (currentSessionId && payload.session_id === currentSessionId) {
                    refreshBoard();
                }
                break;
        }
    }

    /* ─── Sessions ─── */
    async function loadSessions() {
        try {
            sessions = await apiFetch('/api/sessions');
            renderSessionSelect();

            // Auto-select first active session
            const active = sessions.find(s => s.status === 'active');
            if (active && !currentSessionId) {
                selectSession(active.id);
            } else {
                updateCloseSessionButtonVisibility();
            }
        } catch (e) {
            console.error('Failed to load sessions:', e);
        }
    }

    function renderSessionSelect() {
        const active = sessions.filter(s => s.status === 'active');
        const closed = sessions.filter(s => s.status === 'closed');

        sessionSelect.innerHTML = '<option value="">-- Select a session --</option>';

        if (active.length > 0) {
            const optgroup = document.createElement('optgroup');
            optgroup.label = 'Active Sessions';
            active.forEach(s => {
                const opt = document.createElement('option');
                opt.value = s.id;
                const projectLabel = s.project ? `[${s.project}] ` : '';
                opt.textContent = projectLabel + s.goal;
                if (s.id === currentSessionId) opt.selected = true;
                optgroup.appendChild(opt);
            });
            sessionSelect.appendChild(optgroup);
        }

        if (closed.length > 0) {
            const optgroup = document.createElement('optgroup');
            optgroup.label = 'Closed Sessions';
            closed.forEach(s => {
                const opt = document.createElement('option');
                opt.value = s.id;
                const projectLabel = s.project ? `[${s.project}] ` : '';
                opt.textContent = projectLabel + s.goal;
                if (s.id === currentSessionId) opt.selected = true;
                optgroup.appendChild(opt);
            });
            sessionSelect.appendChild(optgroup);
        }
    }

    function selectSession(id) {
        currentSessionId = id;
        sessionSelect.value = id;
        updateCloseSessionButtonVisibility();
        refreshBoard();
    }

    function updateCloseSessionButtonVisibility() {
        const current = sessions.find(s => s.id === currentSessionId);
        if (current && current.status === 'active') {
            closeSessionBtn.style.display = 'inline-flex';
        } else {
            closeSessionBtn.style.display = 'none';
        }
    }

    /* ─── Board ─── */
    async function refreshBoard() {
        if (!currentSessionId) {
            boardData = null;
            renderBoard();
            return;
        }

        try {
            const data = await apiFetch(`/api/sessions/${currentSessionId}/board`);
            boardData = data;
            renderBoard();
        } catch (e) {
            console.error('Failed to load board:', e);
            boardData = null;
            renderBoard();
        }
    }

    function renderBoard() {
        COLUMNS.forEach(col => {
            const container = document.getElementById(`col-${col}`);
            const countEl = document.getElementById(`count-${col}`);
            const delegations = boardData?.columns?.[col] || [];

            container.innerHTML = '';

            // In backlog column, show "Add delegation" button at top
            if (col === 'backlog' && currentSessionId) {
                const sessionClosed = sessions.find(s => s.id === currentSessionId)?.status === 'closed';
                if (!sessionClosed) {
                    const addBtn = document.createElement('button');
                    addBtn.className = 'add-delegation-btn';
                    addBtn.innerHTML = '➕ Add Delegation';
                    addBtn.addEventListener('click', () => openDelegationModal());
                    container.appendChild(addBtn);
                }
            }

            if (delegations.length === 0) {
                const empty = document.createElement('div');
                empty.className = 'empty-state';
                empty.innerHTML = `
                    <span style="font-size: 1.5rem; margin-bottom: 4px; display:block;">✨</span>
                    No tasks in ${COLUMN_LABELS[col]}
                `;
                container.appendChild(empty);
            } else {
                delegations.forEach(d => {
                    const card = createCard(d);
                    container.appendChild(card);
                });
            }

            countEl.textContent = delegations.length;
        });
    }

    function createCard(d) {
        const agent = AGENTS[d.agent] || { icon: '❓', label: d.agent, color: '#888' };
        const card = document.createElement('div');
        card.className = `card card-agent-${d.agent}`;
        
        // Disable drag if session is closed
        const sessionClosed = sessions.find(s => s.id === currentSessionId)?.status === 'closed';
        if (!sessionClosed) {
            card.draggable = true;
        }
        
        card.dataset.delegationId = d.id;

        // Header: agent badge + ID
        const header = document.createElement('div');
        header.className = 'card-header';
        header.innerHTML = `
            <span class="card-agent-badge">${agent.icon} &nbsp;${agent.label}</span>
            <span class="card-id">#${d.id.substring(0, 8)}</span>
        `;
        card.appendChild(header);

        // Title
        const title = document.createElement('div');
        title.className = 'card-title';
        title.textContent = d.task_summary;
        card.appendChild(title);

        // Meta
        const meta = document.createElement('div');
        meta.className = 'card-meta';

        // Time ago
        const timeSpan = document.createElement('span');
        timeSpan.className = 'card-time';
        timeSpan.innerHTML = `⏱️ ${formatTimeAgo(d.created_at)}`;
        meta.appendChild(timeSpan);

        // Badges container
        const badgesContainer = document.createElement('div');
        badgesContainer.className = 'card-badges';

        // Dependencies badge
        if (d.dependencies && d.dependencies.length > 0) {
            const depBadge = document.createElement('span');
            depBadge.className = 'card-badge';
            depBadge.innerHTML = `🔗 ${d.dependencies.length}`;
            depBadge.title = `Depends on: ${d.dependencies.join(', ')}`;
            badgesContainer.appendChild(depBadge);
        }

        // Blocker badge
        if (d.blocker) {
            const blockBadge = document.createElement('span');
            blockBadge.className = 'card-badge blocked';
            blockBadge.innerHTML = `🚫 Blocked`;
            blockBadge.title = d.blocker;
            badgesContainer.appendChild(blockBadge);
        }

        meta.appendChild(badgesContainer);
        card.appendChild(meta);

        // Drag events
        if (!sessionClosed) {
            card.addEventListener('dragstart', (e) => {
                e.dataTransfer.setData('text/plain', d.id);
                card.classList.add('dragging');
            });

            card.addEventListener('dragend', () => {
                card.classList.remove('dragging');
            });
        }

        // Click event -> open detail modal
        card.addEventListener('click', (e) => {
            // Ignore if clicking badges, or dragging
            if (e.target.closest('.card-badge') || card.classList.contains('dragging')) {
                return;
            }
            openDetailsModal(d);
        });

        return card;
    }

    /* ─── Drag & Drop ─── */
    function setupDragDrop() {
        COLUMNS.forEach(col => {
            const columnEl = document.querySelector(`[data-column="${col}"]`);
            if (!columnEl) return;

            columnEl.addEventListener('dragover', (e) => {
                const sessionClosed = sessions.find(s => s.id === currentSessionId)?.status === 'closed';
                if (sessionClosed) return;
                
                e.preventDefault();
                const cards = columnEl.querySelector('.column-cards');
                if (cards) cards.classList.add('drag-over');
            });

            columnEl.addEventListener('dragleave', (e) => {
                const cards = columnEl.querySelector('.column-cards');
                if (cards) cards.classList.remove('drag-over');
            });

            columnEl.addEventListener('drop', async (e) => {
                const sessionClosed = sessions.find(s => s.id === currentSessionId)?.status === 'closed';
                if (sessionClosed) return;

                e.preventDefault();
                const cards = columnEl.querySelector('.column-cards');
                if (cards) cards.classList.remove('drag-over');

                const delegationId = e.dataTransfer.getData('text/plain');
                if (!delegationId) return;

                const targetColumn = columnEl.dataset.column;

                try {
                    await apiFetch(`/api/delegations/${delegationId}`, {
                        method: 'PATCH',
                        body: JSON.stringify({ column: targetColumn }),
                    });
                    showToast(`Moved delegation to <strong>${COLUMN_LABELS[targetColumn]}</strong>`);
                    // Board will be refreshed via WebSocket update
                } catch (e) {
                    showToast(`Error: ${e.message}`);
                }
            });
        });
    }

    /* ─── Modals ─── */
    function openSessionModal() {
        sessionModal.style.display = 'flex';
        sessionGoal.value = '';
        sessionGoal.focus();
    }

    function closeSessionModal() {
        sessionModal.style.display = 'none';
    }

    function openDelegationModal() {
        if (!currentSessionId) {
            showToast('Please select a session first');
            return;
        }
        delegationModal.style.display = 'flex';
        delegationTask.value = '';
        delegationDeps.value = '';
        delegationTask.focus();
    }

    function closeDelegationModal() {
        delegationModal.style.display = 'none';
    }

    /* ─── Details Modal ─── */
    function openDetailsModal(d) {
        detailsId.textContent = d.id;
        
        const agent = AGENTS[d.agent] || { icon: '❓', label: d.agent };
        detailsAgent.innerHTML = `<span class="card-agent-badge">${agent.icon} &nbsp;${agent.label}</span>`;
        
        detailsColumn.className = `meta-value badge ${d.column}`;
        detailsColumn.textContent = COLUMN_LABELS[d.column] || d.column;
        
        detailsStatus.className = `meta-value badge ${d.column}`;
        detailsStatus.textContent = d.status;

        detailsTaskSummary.textContent = d.task_summary;
        $('#details-delegation-id').value = d.id;

        // Check blocker state
        if (d.blocker) {
            detailsBlockedCheckbox.checked = true;
            detailsBlockerContainer.style.display = 'block';
            detailsBlockerInput.value = d.blocker;
        } else {
            detailsBlockedCheckbox.checked = false;
            detailsBlockerContainer.style.display = 'none';
            detailsBlockerInput.value = '';
        }

        // Handoff preview
        detailsHandoffInput.value = d.handoff_preview || '';

        // Handle disabled inputs if session is closed
        const sessionClosed = sessions.find(s => s.id === currentSessionId)?.status === 'closed';
        const submitBtn = detailsForm.querySelector('button[type="submit"]');
        if (sessionClosed) {
            detailsBlockedCheckbox.disabled = true;
            detailsBlockerInput.disabled = true;
            detailsHandoffInput.disabled = true;
            if (submitBtn) submitBtn.style.display = 'none';
        } else {
            detailsBlockedCheckbox.disabled = false;
            detailsBlockerInput.disabled = false;
            detailsHandoffInput.disabled = false;
            if (submitBtn) submitBtn.style.display = 'inline-flex';
        }

        detailsModal.style.display = 'flex';
    }

    function closeDetailsModal() {
        detailsModal.style.display = 'none';
    }

    // Toggle Blocker Reason input visibility
    detailsBlockedCheckbox.addEventListener('change', () => {
        if (detailsBlockedCheckbox.checked) {
            detailsBlockerContainer.style.display = 'block';
            detailsBlockerInput.focus();
        } else {
            detailsBlockerContainer.style.display = 'none';
            detailsBlockerInput.value = '';
        }
    });

    /* ─── Event Listeners ─── */
    // Session select change
    sessionSelect.addEventListener('change', () => {
        const id = sessionSelect.value;
        if (id) {
            selectSession(id);
        } else {
            currentSessionId = null;
            boardData = null;
            updateCloseSessionButtonVisibility();
            renderBoard();
        }
    });

    // New session button
    newSessionBtn.addEventListener('click', openSessionModal);

    // Close session button
    closeSessionBtn.addEventListener('click', async () => {
        if (!currentSessionId) return;
        if (!confirm('Are you sure you want to close this session? Closed sessions cannot add new delegations or drag tasks.')) {
            return;
        }

        try {
            await apiFetch(`/api/sessions/${currentSessionId}`, {
                method: 'PATCH',
                body: JSON.stringify({ status: 'closed' }),
            });
            showToast('Session closed successfully!');
        } catch (e) {
            showToast(`Error: ${e.message}`);
        }
    });

    // Session form submit
    sessionForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const project = sessionProject.value.trim();
        const goal = sessionGoal.value.trim();
        if (!project || !goal) return;

        try {
            const newSession = await apiFetch('/api/sessions', {
                method: 'POST',
                body: JSON.stringify({ project, goal }),
            });
            closeSessionModal();
            selectSession(newSession.id);
        } catch (e) {
            showToast(`Error: ${e.message}`);
        }
    });

    sessionModalCancel.addEventListener('click', closeSessionModal);
    sessionModalCloseIcon.addEventListener('click', closeSessionModal);

    // Delegation form submit
    delegationForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const agent = delegationAgent.value;
        const task = delegationTask.value.trim();
        const deps = delegationDeps.value.trim()
            ? delegationDeps.value.split(',').map(s => s.trim()).filter(Boolean)
            : [];

        if (!agent || !task) return;

        try {
            await apiFetch('/api/delegations', {
                method: 'POST',
                body: JSON.stringify({
                    session_id: currentSessionId,
                    agent,
                    task_summary: task,
                    dependencies: deps,
                }),
            });
            closeDelegationModal();
            showToast('Delegation created successfully!');
        } catch (e) {
            showToast(`Error: ${e.message}`);
        }
    });

    delegationModalCancel.addEventListener('click', closeDelegationModal);
    delegationModalCloseIcon.addEventListener('click', closeDelegationModal);

    // Details form submit
    detailsForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const id = $('#details-delegation-id').value;
        const handoff = detailsHandoffInput.value.trim();
        
        let blocker = '';
        if (detailsBlockedCheckbox.checked) {
            blocker = detailsBlockerInput.value.trim() || 'Blocked';
        }

        try {
            await apiFetch(`/api/delegations/${id}`, {
                method: 'PATCH',
                body: JSON.stringify({
                    blocker: blocker,
                    handoff_preview: handoff
                }),
            });
            closeDetailsModal();
            showToast('Delegation updated successfully!');
        } catch (e) {
            showToast(`Error: ${e.message}`);
        }
    });

    detailsModalCancel.addEventListener('click', closeDetailsModal);
    detailsModalCloseIcon.addEventListener('click', closeDetailsModal);

    // Close modals on Escape
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            closeSessionModal();
            closeDelegationModal();
            closeDetailsModal();
        }
    });

    // Close modals on backdrop click
    [sessionModal, delegationModal, detailsModal].forEach(modal => {
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                modal.style.display = 'none';
            }
        });
    });

    /* ─── Init ─── */
    async function init() {
        connectWS();
        setupDragDrop();
        await loadSessions();

        // Periodic refresh every 30s as fallback
        setInterval(() => {
            if (currentSessionId) refreshBoard();
        }, 30000);
    }

    document.addEventListener('DOMContentLoaded', init);
})();
