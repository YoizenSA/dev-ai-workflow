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
    let reconnectDelay = 1000;
    const MAX_RECONNECT_DELAY = 30000;
    let apiToken = null;

    // Expose functions to global scope for inline event handlers
    window.kanbanApp = {};

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
        const headers = { 'Content-Type': 'application/json', ...opts.headers };
        if (apiToken && path.startsWith('/api/config')) {
            headers['Authorization'] = `Bearer ${apiToken}`;
        }
        const res = await fetch(url, {
            headers,
            ...opts,
        });
        if (!res.ok) {
            const err = await res.json().catch(() => ({ error: res.statusText }));
            throw new Error(err.error || `HTTP ${res.status}`);
        }
        return res.json();
    }

    async function loadApiToken() {
        try {
            const data = await apiFetch('/api/token');
            apiToken = data.token;
        } catch (e) {
            console.error('Failed to load API token:', e);
        }
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
            reconnectDelay = 1000; // reset on successful connection
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
            const delay = reconnectDelay + Math.random() * 1000; // jitter
            reconnectDelay = Math.min(reconnectDelay * 2, MAX_RECONNECT_DELAY);
            wsReconnectTimer = setTimeout(connectWS, delay);
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

            case 'session.updated':
            case 'session.status_changed': {
                const idx = sessions.findIndex(s => s.id === payload.id);
                if (idx !== -1) sessions[idx] = payload;
                renderSessionSelect();
                updateCloseSessionButtonVisibility();
                if (currentSessionId === payload.id && payload.status === 'closed') {
                    showToast('Session closed successfully');
                    refreshBoard(); // Full refresh when session closes (to disable all cards)
                }
                break;
            }

            case 'delegation.created':
                if (currentSessionId && payload.session_id === currentSessionId) {
                    addCardToBoard(payload);
                }
                break;

            case 'delegation.status_changed':
                if (currentSessionId && payload.session_id === currentSessionId) {
                    moveCardOnBoard(payload);
                }
                break;
        }
    }

    /* ─── Sessions ─── */
    async function loadSessions() {
        try {
            const data = await apiFetch('/api/sessions');
            sessions = Array.isArray(data) ? data : [];
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
        if (!Array.isArray(sessions)) return;
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
        const sessionClosed = sessions.find(s => s.id === currentSessionId)?.status === 'closed';

        COLUMNS.forEach(col => {
            const container = document.getElementById(`col-${col}`);
            const countEl = document.getElementById(`count-${col}`);
            const delegations = boardData?.columns?.[col] || [];

            container.innerHTML = '';

            // In backlog column, show "Add delegation" button at top
            if (col === 'backlog' && currentSessionId) {
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
                    const card = createCard(d, sessionClosed);
                    container.appendChild(card);
                });
            }

            countEl.textContent = delegations.length;
        });
    }

    function createCard(d, sessionClosed) {
        const agent = AGENTS[d.agent] || { icon: '❓', label: d.agent, color: '#888' };
        const card = document.createElement('div');
        card.className = `card card-agent-${d.agent}`;
        
        // Disable drag if session is closed
        if (!sessionClosed) {
            card.draggable = true;
        }
        
        card.dataset.delegationId = d.id;
        card.dataset.id = d.id;

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

    /* ─── Targeted DOM Updates ─── */
    function findCard(delegationId) {
        return document.querySelector(`.card[data-id="${delegationId}"]`);
    }

    function addCardToBoard(d) {
        const sessionClosed = sessions.find(s => s.id === currentSessionId)?.status === 'closed';
        const card = createCard(d, sessionClosed);
        const container = document.querySelector(`.column[data-column="${d.column}"] .column-cards`);
        if (container) {
            container.appendChild(card);
            const countEl = document.querySelector(`.column[data-column="${d.column}"] .column-count`);
            if (countEl) {
                countEl.textContent = container.children.length;
            }
        }
    }

    function removeCardFromBoard(delegationId) {
        const card = findCard(delegationId);
        if (card) {
            const container = card.closest('.column-cards');
            const column = card.closest('.column');
            card.remove();
            if (column) {
                const countEl = column.querySelector('.column-count');
                const cardsContainer = column.querySelector('.column-cards');
                if (countEl && cardsContainer) {
                    countEl.textContent = cardsContainer.children.length;
                }
            }
        }
    }

    function moveCardOnBoard(d) {
        const existingCard = findCard(d.id);
        if (existingCard) {
            const currentColumn = existingCard.closest('.column');
            const currentColumnName = currentColumn?.dataset.column;

            if (currentColumnName !== d.column) {
                // Different column: remove from old, add to new
                const oldContainer = currentColumn.querySelector('.column-cards');
                const oldCount = currentColumn.querySelector('.column-count');

                existingCard.remove();

                const sessionClosed = sessions.find(s => s.id === currentSessionId)?.status === 'closed';
                const newCard = createCard(d, sessionClosed);
                const newContainer = document.querySelector(`.column[data-column="${d.column}"] .column-cards`);
                if (newContainer) {
                    newContainer.appendChild(newCard);
                }

                if (oldCount && oldContainer) oldCount.textContent = oldContainer.children.length;
                const newColumn = document.querySelector(`.column[data-column="${d.column}"]`);
                const newCount = newColumn?.querySelector('.column-count');
                const newCardsContainer = newColumn?.querySelector('.column-cards');
                if (newCount && newCardsContainer) newCount.textContent = newCardsContainer.children.length;
            } else {
                // Same column: just update the card in place
                const sessionClosed = sessions.find(s => s.id === currentSessionId)?.status === 'closed';
                const newCard = createCard(d, sessionClosed);
                existingCard.replaceWith(newCard);
            }
        } else {
            addCardToBoard(d);
        }
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
            if (modal) {
                modal.addEventListener('click', (e) => {
                    if (e.target === modal) {
                        modal.style.display = 'none';
                    }
                });
            }
        });

        // Close session modal on X button and Cancel button
        const closeModalBtn = $('#close-modal-btn');
        const cancelSessionBtn = $('#cancel-session-btn');
        if (closeModalBtn) closeModalBtn.addEventListener('click', () => sessionModal.style.display = 'none');
        if (cancelSessionBtn) cancelSessionBtn.addEventListener('click', () => sessionModal.style.display = 'none');

    /* ─── Init ─── */
    async function init() {
        await loadApiToken();
        connectWS();
        setupDragDrop();
        setupTabs();
        setupConfig();
        await loadSessions();

        // Periodic refresh every 30s as fallback
        setInterval(() => {
            if (currentSessionId) refreshBoard();
        }, 30000);
    }

    // --- Tab System ---
    function setupTabs() {
        document.querySelectorAll('.tab-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                const tab = btn.dataset.tab;
                
                // Update buttons
                document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                
                // Update content
                document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
                document.getElementById(`tab-${tab}`).classList.add('active');
                
                // Load config data when switching to config tab
                if (tab === 'config') {
                    loadConfigData();
                }
            });
        });
    }

    // --- Config System ---
    let configData = { mcp: [], agents: [], skills: [], json: null, providers: {} };
    let currentAgent = null;

    function setupConfig() {
        // Config nav
        document.querySelectorAll('.config-nav-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                const section = btn.dataset.section;
                document.querySelectorAll('.config-nav-btn').forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                document.querySelectorAll('.config-section').forEach(s => s.classList.remove('active'));
                document.getElementById(`section-${section}`).classList.add('active');
                if (section === 'providers') loadProviders();
                if (section === 'permissions') loadPermissions();
            });
        });

        // Agent editor
        document.getElementById('agent-cancel-btn').addEventListener('click', () => {
            document.getElementById('agent-editor').style.display = 'none';
            currentAgent = null;
        });

        document.getElementById('agent-save-btn').addEventListener('click', saveAgent);

        document.getElementById('agent-delete-btn').addEventListener('click', () => {
            if (currentAgent) {
                deleteAgent(currentAgent);
            }
        });

        // New agent
        document.getElementById('new-agent-btn').addEventListener('click', () => {
            const name = prompt('Agent name:');
            if (name) {
                currentAgent = name;
                document.getElementById('agent-editor-title').textContent = `New: ${name}`;
                document.getElementById('agent-content').value = `# ${name}\n\nYou are a ${name} agent.\n\n## Role\nDescribe your role here.\n`;
                document.getElementById('agent-delete-btn').style.display = 'none';
                document.getElementById('agent-editor').style.display = 'flex';
            }
        });

        // JSON save
        document.getElementById('save-json-btn').addEventListener('click', saveJson);
    }

    async function loadConfigData() {
        await Promise.all([
            loadMCP(),
            loadAgents(),
            loadSkills(),
            loadJson(),
            loadProviders()
        ]);
    }

    // --- MCP ---
    async function loadMCP() {
        try {
            const res = await fetch('/api/config/mcp');
            configData.mcp = await res.json();
            renderMCP();
        } catch (e) {
            console.error('Failed to load MCP:', e);
        }
    }

    function renderMCP() {
        const list = document.getElementById('mcp-list');
        list.innerHTML = configData.mcp.map(mcp => {
            let type = 'unknown';
            let detailsHtml = '';
            try {
                const cfg = JSON.parse(typeof mcp.config === 'string' ? mcp.config : JSON.stringify(mcp.config));
                type = cfg.type || 'local';
                
                const details = [];
                if (cfg.url) {
                    details.push(`<strong>URL:</strong> <span class="code-font">${cfg.url}</span>`);
                }
                if (cfg.command && cfg.command.length > 0) {
                    const cmdStr = Array.isArray(cfg.command) ? cfg.command.join(' ') : cfg.command;
                    details.push(`<strong>Command:</strong> <span class="code-font">${cmdStr}</span>`);
                }
                if (cfg.args && cfg.args.length > 0) {
                    details.push(`<strong>Args:</strong> <span class="code-font">${cfg.args.join(' ')}</span>`);
                }
                if (cfg.env && Object.keys(cfg.env).length > 0) {
                    const envStrings = Object.entries(cfg.env).map(([k, v]) => `${k}=${v}`);
                    details.push(`<strong>Env:</strong> <span class="code-font">${envStrings.join(', ')}</span>`);
                }
                
                for (const [key, val] of Object.entries(cfg)) {
                    if (['type', 'url', 'command', 'args', 'env', 'enabled', 'disabled'].includes(key)) continue;
                    
                    if (typeof val === 'object' && val !== null) {
                        details.push(`<strong>${key}:</strong> <span class="code-font">${JSON.stringify(val)}</span>`);
                    } else {
                        let displayVal = val;
                        if (key.toLowerCase().includes('key') || key.toLowerCase().includes('secret') || key.toLowerCase().includes('token')) {
                            if (typeof val === 'string' && val.length > 8) {
                                displayVal = val.substring(0, 4) + '...' + val.substring(val.length - 4);
                            }
                        }
                        details.push(`<strong>${key}:</strong> <span class="code-font">${displayVal}</span>`);
                    }
                }
                
                if (details.length > 0) {
                    detailsHtml = `<div class="mcp-details">${details.join('<br>')}</div>`;
                }
            } catch (e) {
                console.error(e);
            }
            
            return `
                <div class="mcp-item">
                    <div class="mcp-header">
                        <div class="mcp-info">
                            <div class="mcp-name">${mcp.name}</div>
                            <div class="mcp-type">${type}</div>
                        </div>
                        <label class="toggle">
                            <input type="checkbox" ${mcp.enabled ? 'checked' : ''} onchange="window.kanbanApp.toggleMCP('${mcp.name}', this.checked)">
                            <span class="toggle-slider"></span>
                        </label>
                    </div>
                    ${detailsHtml}
                </div>
            `;
        }).join('');
    }

    async function toggleMCP(name, enabled) {
        try {
            await fetch(`/api/config/mcp/${name}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ enabled })
            });
            showToast(`${name} ${enabled ? 'enabled' : 'disabled'}. Restart opencode to apply.`);
        } catch (e) {
            console.error('Failed to toggle MCP:', e);
            showToast('Failed to toggle MCP', 'error');
        }
    }

    // Expose toggleMCP to global scope
    window.kanbanApp.toggleMCP = toggleMCP;

    // --- Providers ---
    async function loadProviders() {
        try {
            const res = await fetch('/api/config/providers');
            configData.providers = await res.json();
            renderProviders();
        } catch (e) {
            console.error('Failed to load providers:', e);
        }
    }

    function renderProviders() {
        const container = document.getElementById('providers-list');
        if (!container) return;

        const providers = configData.providers;
        if (!providers || Object.keys(providers).length === 0) {
            container.innerHTML = `
                <div class="providers-empty">
                    <svg class="providers-empty-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path><polyline points="3.27 6.96 12 12.01 20.73 6.96"></polyline><line x1="12" y1="22.08" x2="12" y2="12"></line></svg>
                    <span class="providers-empty-text">No providers configured</span>
                    <span class="providers-empty-hint">Add your first LLM provider to get started</span>
                </div>`;
            return;
        }

        let html = '<div class="providers-grid">';
        for (const [name, config] of Object.entries(providers)) {
            const modelCount = config.models ? Object.keys(config.models).length : 0;
            const hasKey = !!config.apiKey;
            const keyBadge = hasKey
                ? `<span class="provider-badge key-ok"><svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>Key set</span>`
                : `<span class="provider-badge key-missing"><svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>Missing key</span>`;
            html += `
                <div class="provider-card" onclick="window.kanbanApp.editProvider('${name.replace(/'/g, "\\'")}')">
                    <div class="provider-card-header">
                        <span class="provider-card-name">${name}</span>
                        <button class="provider-card-delete" onclick="event.stopPropagation(); window.kanbanApp.deleteProvider('${name.replace(/'/g, "\\'")}')" title="Delete provider">
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
                        </button>
                    </div>
                    <div class="provider-card-meta">
                        <span class="provider-badge models"><svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect><line x1="8" y1="21" x2="16" y2="21"></line><line x1="12" y1="17" x2="12" y2="21"></line></svg>${modelCount} models</span>
                        ${keyBadge}
                    </div>
                    <div class="provider-card-url">${config.baseUrl || 'No base URL configured'}</div>
                </div>`;
        }
        html += '</div>';
        container.innerHTML = html;
    }

    async function addProvider() {
        document.getElementById('provider-editor-title').textContent = 'Add Provider';
        document.getElementById('provider-name-input').value = '';
        document.getElementById('provider-name-input').disabled = false;
        document.getElementById('provider-baseurl-input').value = '';
        document.getElementById('provider-apikey-input').value = '';
        document.getElementById('provider-models-input').value = '';
        document.getElementById('provider-delete-btn').style.display = 'none';
        showProviderEditor();
    }

    async function editProvider(name) {
        try {
            const res = await fetch('/api/config/providers');
            const providers = await res.json();
            const p = providers[name];
            if (!p) return;
            document.getElementById('provider-editor-title').textContent = `Edit: ${name}`;
            document.getElementById('provider-name-input').value = name;
            document.getElementById('provider-name-input').disabled = true;
            document.getElementById('provider-baseurl-input').value = p.baseUrl || '';
            document.getElementById('provider-apikey-input').value = p.apiKey || '';
            document.getElementById('provider-models-input').value = JSON.stringify(p.models || {}, null, 2);
            document.getElementById('provider-delete-btn').style.display = 'inline-flex';
            showProviderEditor();
        } catch (e) {
            console.error('Failed to load provider for edit:', e);
        }
    }

    function showProviderEditor() {
        const overlay = document.getElementById('provider-modal-overlay');
        overlay.style.display = 'flex';
        setTimeout(() => {
            const firstInput = document.getElementById('provider-name-input');
            if (firstInput && !firstInput.disabled) firstInput.focus();
        }, 50);
    }

    async function saveProvider() {
        const nameInput = document.getElementById('provider-name-input');
        const name = nameInput.value.trim();
        if (!name) { showToast('Provider name is required', 'error'); return; }

        let models = {};
        const modelsText = document.getElementById('provider-models-input').value.trim();
        if (modelsText) {
            try { models = JSON.parse(modelsText); }
            catch { showToast('Invalid models JSON', 'error'); return; }
        }

        const provider = {
            baseUrl: document.getElementById('provider-baseurl-input').value.trim(),
            apiKey: document.getElementById('provider-apikey-input').value.trim(),
            models: models
        };

        try {
            const res = await fetch(`/api/config/providers/${encodeURIComponent(name)}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(provider)
            });

            if (res.ok) {
                showToast('Provider saved', 'success');
                document.getElementById('provider-modal-overlay').style.display = 'none';
                loadProviders();
            } else {
                const err = await res.json();
                showToast(err.error || 'Failed to save provider', 'error');
            }
        } catch (e) {
            console.error('Failed to save provider:', e);
            showToast('Failed to save provider', 'error');
        }
    }

    function cancelProviderEdit() {
        document.getElementById('provider-modal-overlay').style.display = 'none';
    }

    async function deleteProvider(name) {
        if (!name) name = document.getElementById('provider-name-input').value.trim();
        if (!name) return;
        if (!confirm(`Delete provider "${name}"?`)) return;

        try {
            const res = await fetch(`/api/config/providers/${encodeURIComponent(name)}`, { method: 'DELETE' });
            if (res.ok) {
                showToast('Provider deleted', 'success');
                document.getElementById('provider-modal-overlay').style.display = 'none';
                loadProviders();
            } else {
                const err = await res.json();
                showToast(err.error || 'Failed to delete provider', 'error');
            }
        } catch (e) {
            console.error('Failed to delete provider:', e);
            showToast('Failed to delete provider', 'error');
        }
    }

    // --- Agents ---
    async function loadAgents() {
        try {
            const res = await fetch('/api/config/agents');
            configData.agents = await res.json();
            renderAgents();
        } catch (e) {
            console.error('Failed to load agents:', e);
        }
    }

    function renderAgents() {
        const list = document.getElementById('agents-list');
        list.innerHTML = configData.agents.map(agent => `
            <div class="agent-item" onclick="window.kanbanApp.editAgent('${agent.name}')">
                <span class="agent-item-name">${agent.name}</span>
                <div style="display: flex; align-items: center; gap: 8px;">
                    <span class="agent-item-size">${formatSize(agent.size)}</span>
                    <button class="agent-delete-btn" onclick="event.stopPropagation(); window.kanbanApp.deleteAgent('${agent.name}')">🗑️</button>
                </div>
            </div>
        `).join('');
    }

    async function editAgent(name) {
        try {
            const res = await fetch(`/api/config/agents/${name}`);
            const data = await res.json();
            currentAgent = name;
            document.getElementById('agent-editor-title').textContent = `Edit: ${name}`;
            document.getElementById('agent-content').value = data.content;
            document.getElementById('agent-delete-btn').style.display = 'inline-flex';
            document.getElementById('agent-editor').style.display = 'flex';
            
            // Highlight active
            document.querySelectorAll('.agent-item').forEach(item => item.classList.remove('active'));
            const activeItem = Array.from(document.querySelectorAll('.agent-item')).find(item => item.textContent.includes(name));
            if (activeItem) activeItem.classList.add('active');
        } catch (e) {
            console.error('Failed to load agent:', e);
        }
    }

    async function saveAgent() {
        if (!currentAgent) return;
        
        const content = document.getElementById('agent-content').value;
        try {
            const res = await fetch(`/api/config/agents/${currentAgent}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ content })
            });
            
            if (res.ok) {
                showToast(`Agent "${currentAgent}" saved`);
                document.getElementById('agent-editor').style.display = 'none';
                loadAgents();
            } else {
                const err = await res.json();
                showToast(err.error || 'Failed to save', 'error');
            }
        } catch (e) {
            console.error('Failed to save agent:', e);
            showToast('Failed to save agent', 'error');
        }
    }

    async function deleteAgent(name) {
        if (!confirm(`Are you sure you want to delete the agent "${name}"?`)) {
            return;
        }
        try {
            const res = await fetch(`/api/config/agents/${name}`, {
                method: 'DELETE'
            });
            if (res.ok) {
                showToast(`Agent "${name}" deleted`);
                document.getElementById('agent-editor').style.display = 'none';
                currentAgent = null;
                loadAgents();
            } else {
                const err = await res.json();
                showToast(err.error || 'Failed to delete agent', 'error');
            }
        } catch (e) {
            console.error('Failed to delete agent:', e);
            showToast('Failed to delete agent', 'error');
        }
    }

    // --- Permissions ---
    async function loadPermissions() {
        const container = document.getElementById('permissions-container');
        const loading = document.getElementById('permissions-loading');
        container.innerHTML = '';
        loading.style.display = 'flex';

        try {
            // Fetch available tools (built-in + MCP discovered)
            const toolsRes = await fetch('/api/config/tools');
            const toolsData = await toolsRes.json();
            const allTools = toolsData.all || [];
            const mcpTools = toolsData.mcp_tools || {};

            // Fetch current opencode config for agent permissions
            const res = await fetch('/api/config/opencode');
            const config = await res.json();
            const agents = config.agent || {};

            const agentNames = Object.keys(agents).sort();
            if (agentNames.length === 0) {
                container.innerHTML = '<p class="empty-state">No agents found in opencode.json.</p>';
                loading.style.display = 'none';
                return;
            }

            const grid = document.createElement('div');
            grid.className = 'permissions-grid';

            // Build a set of MCP tool names for quick lookup
            const mcpToolSet = new Set();
            for (const [, tools] of Object.entries(mcpTools)) {
                tools.forEach(t => mcpToolSet.add(t));
            }

            for (const name of agentNames) {
                const agent = agents[name];
                const permission = agent.permission || {};
                const mode = agent.mode || 'all';

                const card = document.createElement('div');
                card.className = 'permission-card';
                card.dataset.agent = name;

                let rowsHtml = '';
                for (const key of allTools) {
                    const val = permission[key] || 'deny';
                    const isMcp = mcpToolSet.has(key);
                    const mcpBadge = isMcp ? '<span class="mcp-badge">MCP</span>' : '';
                    rowsHtml += `
                        <div class="permission-row">
                            <span class="permission-key">${key}${mcpBadge}</span>
                            <select class="permission-select ${val}" data-key="${key}" data-agent="${name}">
                                <option value="allow" ${val === 'allow' ? 'selected' : ''}>allow</option>
                                <option value="ask" ${val === 'ask' ? 'selected' : ''}>ask</option>
                                <option value="deny" ${val === 'deny' ? 'selected' : ''}>deny</option>
                            </select>
                        </div>
                    `;
                }

                card.innerHTML = `
                    <div class="permission-card-header">
                        <div>
                            <span class="permission-card-name">${name}</span>
                            <span class="permission-card-mode">${mode}</span>
                        </div>
                        <div class="permission-actions">
                            <button class="btn btn-secondary btn-sm" onclick="window.kanbanApp.addCustomTool('${name}')">+ Tool</button>
                            <button class="btn btn-primary btn-sm" onclick="window.kanbanApp.savePermissions('${name}')">Save</button>
                        </div>
                    </div>
                    <div class="permission-rows">
                        ${rowsHtml}
                    </div>
                `;
                grid.appendChild(card);
            }

            container.appendChild(grid);

            // Add change listeners to update select styling
            container.querySelectorAll('.permission-select').forEach(sel => {
                sel.addEventListener('change', () => {
                    sel.className = `permission-select ${sel.value}`;
                });
            });
        } catch (e) {
            console.error('Failed to load permissions:', e);
            container.innerHTML = '<p class="empty-state">Failed to load permissions.</p>';
        } finally {
            loading.style.display = 'none';
        }
    }

    function addCustomTool(agentName) {
        const name = prompt('Tool name (e.g. my_custom_tool or MCP tool):');
        if (!name || !name.trim()) return;
        const key = name.trim();

        const card = document.querySelector(`.permission-card[data-agent="${agentName}"]`);
        if (!card) return;
        const rowsContainer = card.querySelector('.permission-rows');
        if (!rowsContainer) return;

        // Check if already exists
        if (rowsContainer.querySelector(`[data-key="${key}"]`)) {
            showToast('Tool already exists', 'error');
            return;
        }

        const row = document.createElement('div');
        row.className = 'permission-row';
        row.innerHTML = `
            <span class="permission-key">${key}</span>
            <select class="permission-select deny" data-key="${key}" data-agent="${agentName}">
                <option value="allow">allow</option>
                <option value="ask">ask</option>
                <option value="deny" selected>deny</option>
            </select>
        `;
        rowsContainer.appendChild(row);
        row.querySelector('.permission-select').addEventListener('change', function() {
            this.className = `permission-select ${this.value}`;
        });
    }

    async function savePermissions(agentName) {
        const selects = document.querySelectorAll(`.permission-select[data-agent="${agentName}"]`);
        const permission = {};
        selects.forEach(sel => {
            permission[sel.dataset.key] = sel.value;
        });

        try {
            const res = await fetch(`/api/config/agents/${agentName}/permissions`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(permission)
            });

            if (res.ok) {
                showToast(`Permissions for "${agentName}" saved`, 'success');
            } else {
                const err = await res.json();
                showToast(err.error || 'Failed to save permissions', 'error');
            }
        } catch (e) {
            console.error('Failed to save permissions:', e);
            showToast('Failed to save permissions', 'error');
        }
    }

    // --- Skills ---
    async function loadSkills() {
        try {
            const res = await fetch('/api/config/skills');
            configData.skills = await res.json();
            renderSkills();
        } catch (e) {
            console.error('Failed to load skills:', e);
        }
    }

    function renderSkills() {
        const list = document.getElementById('skills-list');
        list.innerHTML = configData.skills.map(skill => `
            <div class="skill-item">
                <div class="skill-header-row" style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 8px;">
                    <div class="skill-name" style="margin-bottom: 0;">${skill.name}</div>
                    <button class="agent-delete-btn" onclick="window.kanbanApp.deleteSkill('${skill.name}')" style="padding: 2px 6px;">🗑️</button>
                </div>
                <div class="skill-desc">${skill.description || 'No description'}</div>
                ${skill.hasSkillMD ? '<span class="skill-badge">SKILL.md</span>' : ''}
            </div>
        `).join('');
    }

    async function deleteSkill(name) {
        if (!confirm(`Are you sure you want to delete the skill "${name}"?`)) {
            return;
        }
        try {
            const res = await fetch(`/api/config/skills/${name}`, {
                method: 'DELETE'
            });
            if (res.ok) {
                showToast(`Skill "${name}" deleted`);
                loadSkills();
            } else {
                const err = await res.json();
                showToast(err.error || 'Failed to delete skill', 'error');
            }
        } catch (e) {
            console.error('Failed to delete skill:', e);
            showToast('Failed to delete skill', 'error');
        }
    }

    // --- JSON Editor ---
    async function loadJson() {
        try {
            const res = await fetch('/api/config/opencode');
            const data = await res.json();
            configData.json = data;
            document.getElementById('json-editor').value = JSON.stringify(data, null, 2);
        } catch (e) {
            console.error('Failed to load JSON:', e);
        }
    }

    async function saveJson() {
        const content = document.getElementById('json-editor').value;
        try {
            // Validate JSON
            JSON.parse(content);
            
            const res = await fetch('/api/config/opencode', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: content
            });
            
            if (res.ok) {
                showToast('opencode.json saved. Restart opencode to apply.');
            } else {
                const err = await res.json();
                showToast(err.error || 'Failed to save', 'error');
            }
        } catch (e) {
            if (e instanceof SyntaxError) {
                showToast('Invalid JSON', 'error');
            } else {
                console.error('Failed to save JSON:', e);
                showToast('Failed to save', 'error');
            }
        }
    }

    function formatSize(bytes) {
        if (bytes < 1024) return bytes + ' B';
        if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
        return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
    }

    function showToast(message, type = 'success') {
        const toast = document.createElement('div');
        toast.className = `toast toast-${type}`;
        toast.textContent = message;
        toast.style.cssText = `
            position: fixed;
            bottom: 20px;
            right: 20px;
            background: ${type === 'error' ? 'var(--danger)' : 'var(--accent)'};
            color: white;
            padding: 12px 20px;
            border-radius: 8px;
            font-size: 14px;
            z-index: 1000;
            animation: slideIn 0.3s ease;
        `;
        document.body.appendChild(toast);
        setTimeout(() => toast.remove(), 3000);
    }

    // Expose functions for onclick handlers
    window.kanbanApp = { editAgent, deleteAgent, deleteSkill, toggleMCP, addProvider, editProvider, saveProvider, cancelProviderEdit, deleteProvider, savePermissions, addCustomTool };

    document.addEventListener('DOMContentLoaded', init);
})();

    // Close provider modal on Escape key
    document.addEventListener('keydown', function(e) {
        if (e.key === 'Escape') {
            const overlay = document.getElementById('provider-modal-overlay');
            if (overlay && overlay.style.display !== 'none') {
                window.kanbanApp.cancelProviderEdit();
            }
        }
    });

