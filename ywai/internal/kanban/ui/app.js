// ywai Kanban — Vanilla JS
(() => {
	/* ─── Agent Config ─── */
	const AGENTS = {
		dev: { icon: "🚧", label: "Dev", color: "#2ed573" },
		qa: { icon: "🧪", label: "QA", color: "#ffa502" },
		architect: { icon: "📋", label: "Architect", color: "#1e90ff" },
		reviewer: { icon: "🔍", label: "Reviewer", color: "#9b59b6" },
		devops: { icon: "🚀", label: "DevOps", color: "#ff4757" },
	};

	const COLUMNS = ["backlog", "ready", "in_progress", "review", "done"];
	const COLUMN_LABELS = {
		backlog: "Backlog",
		ready: "Ready",
		in_progress: "In Progress",
		review: "Review",
		done: "Done",
	};

	/* ─── State ─── */
	let sessions = [];
	let currentSessionId = null;
	let boardData = null; // { session, columns: { backlog: [], ready: [], ... } }
	let ws = null;
	let wsReconnectTimer = null;
	let reconnectDelay = 1000;
	const MAX_RECONNECT_DELAY = 30000;

	// Expose functions to global scope for inline event handlers
	window.kanbanApp = {};

	/* ─── DOM refs ─── */
	const $ = (sel) => document.querySelector(sel);

	const sessionSelect = $("#session-select");
	const newSessionBtn = $("#new-session-btn");
	const closeSessionBtn = $("#close-session-btn");
	const deleteSessionBtn = $("#delete-session-btn");
	const wsStatus = $("#ws-status");

	const sessionModal = $("#session-modal");
	const sessionForm = $("#session-form");
	const sessionProject = $("#session-project");
	const sessionGoal = $("#session-goal");
	const sessionModalCancel = $("#session-modal-cancel");
	const sessionModalCloseIcon = $("#session-modal-close-icon");

	const delegationModal = $("#delegation-modal");
	const delegationForm = $("#delegation-form");
	const delegationAgent = $("#delegation-agent");
	const delegationTask = $("#delegation-task");
	const delegationDeps = $("#delegation-deps");
	const delegationModalCancel = $("#delegation-modal-cancel");
	const delegationModalCloseIcon = $("#delegation-modal-close-icon");

	// Details Modal refs
	const detailsModal = $("#details-modal");
	const detailsForm = $("#details-form");
	const detailsId = $("#details-id");
	const detailsAgent = $("#details-agent");
	const detailsColumn = $("#details-column");
	const detailsStatus = $("#details-status");
	const detailsTaskSummary = $("#details-task-summary");
	const detailsBlockedCheckbox = $("#details-blocked-checkbox");
	const detailsBlockerContainer = $("#details-blocker-container");
	const detailsBlockerInput = $("#details-blocker-input");
	const detailsHandoffInput = $("#details-handoff-input");
	const detailsModalCancel = $("#details-modal-cancel");
	const detailsModalCloseIcon = $("#details-modal-close-icon");

	/* ─── Helpers ─── */
	function apiBase() {
		const loc = window.location;
		return `${loc.protocol}//${loc.host}`;
	}

	function wsBase() {
		const loc = window.location;
		const proto = loc.protocol === "https:" ? "wss:" : "ws:";
		return `${proto}//${loc.host}`;
	}

	async function apiFetch(path, opts = {}) {
		const url = `${apiBase()}${path}`;
		const headers = { "Content-Type": "application/json", ...opts.headers };
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

	function showToast(msg) {
		let toast = document.querySelector(".toast");
		if (!toast) {
			toast = document.createElement("div");
			toast.className = "toast";
			document.body.appendChild(toast);
		}
		toast.innerHTML = `🔔 &nbsp; ${msg}`;
		toast.classList.add("show");
		clearTimeout(toast._hide);
		toast._hide = setTimeout(() => toast.classList.remove("show"), 3500);
	}

	function formatTimeAgo(dateStr) {
		const diff = Date.now() - new Date(dateStr).getTime();
		const mins = Math.floor(diff / 60000);
		if (mins < 1) return "just now";
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
			wsStatus.textContent = "● connected";
			wsStatus.className = "ws-status connected";
			if (wsReconnectTimer) {
				clearTimeout(wsReconnectTimer);
				wsReconnectTimer = null;
			}
		};

		ws.onclose = () => {
			wsStatus.textContent = "● disconnected";
			wsStatus.className = "ws-status disconnected";
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
				console.error("WS parse error:", e);
			}
		};
	}

	function handleBoardUpdate(update) {
		const { type, payload } = update;

		switch (type) {
			case "session.created":
				sessions.push(payload);
				renderSessionSelect();
				showToast(`Session created: <strong>${payload.goal}</strong>`);
				break;

			case "session.updated":
			case "session.status_changed": {
				const idx = sessions.findIndex((s) => s.id === payload.id);
				if (idx !== -1) sessions[idx] = payload;
				renderSessionSelect();
				updateCloseSessionButtonVisibility();
				if (currentSessionId === payload.id && payload.status === "closed") {
					showToast("Session closed successfully");
					refreshBoard(); // Full refresh when session closes (to disable all cards)
				}
				break;
			}

			case "delegation.created":
				if (currentSessionId && payload.session_id === currentSessionId) {
					addCardToBoard(payload);
				}
				break;

			case "delegation.status_changed":
				if (currentSessionId && payload.session_id === currentSessionId) {
					moveCardOnBoard(payload);
				}
				break;

			case "session.deleted":
				if (currentSessionId && payload.session_id === currentSessionId) {
					currentSessionId = null;
					boardData = null;
					loadSessions();
					renderBoard();
					showToast("Session was deleted");
				} else {
					loadSessions();
				}
				break;

			case "activity.created":
				handleActivityCreated(payload);
				break;

			case "activity.resolved":
				handleActivityResolved(payload);
				break;

			case "decision.pending":
				addToDecisionFeed(payload);
				break;

			case "delegation.auto_unblocked":
				showToast(
					`✅ Auto-unblocked: <strong>${payload.task_summary}</strong>`,
				);
				refreshGraphIfVisible();
				break;
		}
	}

	function handleActivityCreated(payload) {
		const card = findCard(payload.delegation_id);
		if (!card) return;

		// Update the action bar
		const actionBar = card.querySelector(".card-action-bar");
		const latestMsg = card.querySelector(".action-latest-msg");
		if (actionBar) {
			actionBar.style.display = "flex";
		}
		if (latestMsg && payload.content) {
			latestMsg.textContent = payload.content;
		}

		// Append to the activity stream if visible
		const activityStream = card.querySelector(".card-activity");
		if (activityStream && activityStream.style.display !== "none") {
			const entry = createActivityEntry(payload);
			activityStream.appendChild(entry);
			activityStream.scrollTop = activityStream.scrollHeight;
		}
	}

	function handleActivityResolved(payload) {
		const card = findCard(payload.delegation_id);
		if (!card) return;

		// Hide the action bar
		const actionBar = card.querySelector(".card-action-bar");
		if (actionBar) {
			actionBar.style.display = "none";
		}

		// Mark the activity as resolved in the stream
		const activityStream = card.querySelector(".card-activity");
		if (activityStream) {
			const entry = activityStream.querySelector(
				`[data-activity-id="${payload.activity_id}"]`,
			);
			if (entry) {
				entry.classList.add("resolved");
				const resolutionEl = entry.querySelector(".activity-resolution");
				if (resolutionEl) {
					resolutionEl.textContent = `✓ Resolved: ${escapeHtml(payload.resolution || "done")}`;
				}
			}
		}
	}

	function addToDecisionFeed(payload) {
		const feed = document.getElementById("decision-feed");
		if (!feed) return;
		const list = feed.querySelector(".feed-list");
		if (!list) return;

		const item = document.createElement("div");
		item.className = "feed-item";
		item.setAttribute("data-decision-id", payload.id);
		item.innerHTML = `
			<div class="feed-item-agent">🤖 ${escapeHtml(payload.agent || "agent")}</div>
			<div class="feed-item-content">${escapeHtml(payload.content || payload.question || "")}</div>
			<div class="feed-item-time">${formatTimeAgo(payload.created_at || new Date().toISOString())}</div>
		`;
		list.prepend(item);
		updateFeedBadge();
	}

	function createActivityEntry(activity) {
		const entry = document.createElement("div");
		const actType = activity.type || "progress";
		entry.className = `activity-event type-${actType}`;
		entry.setAttribute("data-activity-id", activity.id);
		entry.innerHTML = `
			<div class="activity-header">
				<span class="activity-type type-${activity.type || "progress"}">${activity.type || "progress"}</span>
				<span class="activity-time">${formatTimeAgo(activity.created_at)}</span>
			</div>
			<div class="activity-content">${escapeHtml(activity.content || "")}</div>
			<div class="activity-resolution"></div>
		`;
		return entry;
	}

	/* ─── Activity & Decision API Helpers ─── */
	async function fetchActivities(delegationId) {
		return apiFetch(`/api/delegations/${delegationId}/activities`);
	}

	async function resolveActivity(delegationId, activityId, resolution) {
		return apiFetch(
			`/api/delegations/${delegationId}/activities/${activityId}`,
			{
				method: "PATCH",
				body: JSON.stringify({ resolution }),
			},
		);
	}

	async function getPendingDecisions(sessionId) {
		return apiFetch(`/api/sessions/${sessionId}/decisions`);
	}

	async function renderDecisionFeed() {
		if (!currentSessionId) {
			const feed = document.getElementById("decision-feed");
			if (feed) feed.style.display = "none";
			return;
		}

		try {
			const decisions = await getPendingDecisions(currentSessionId);
			const feed = document.getElementById("decision-feed");
			if (!feed) return;

			feed.style.display = decisions && decisions.length > 0 ? "block" : "none";
			const list = feed.querySelector(".feed-list");
			if (!list) return;

			list.innerHTML = "";
			const items = decisions || [];
			if (items.length === 0) {
				list.innerHTML = `
					<div class="feed-empty">
						<span class="feed-empty-icon">🎉</span>
						<span>No pending decisions</span>
					</div>
				`;
			} else {
				items.forEach((d) => {
					const item = document.createElement("div");
					item.className = "feed-item";
					item.setAttribute("data-decision-id", d.id);
					item.innerHTML = `
						<div class="feed-item-agent">🤖 ${escapeHtml(d.agent || "agent")}</div>
						<div class="feed-item-content">${escapeHtml(d.content || d.question || "")}</div>
						<div class="feed-item-time">${formatTimeAgo(d.created_at || new Date().toISOString())}</div>
					`;
					list.appendChild(item);
				});
			}
		} catch (e) {
			console.error("Failed to load decisions:", e);
		}
	}

	function updateFeedBadge() {
		const feed = document.getElementById("decision-feed");
		if (!feed) return;
		const list = feed.querySelector(".feed-list");
		if (!list) return;
		const badge = feed.querySelector(".feed-badge");
		if (badge) {
			const count = list.children.length;
			badge.textContent = count;
			badge.style.display = count > 0 ? "inline" : "none";
		}
	}

	/* ─── Sessions ─── */
	async function loadSessions() {
		try {
			const data = await apiFetch("/api/sessions");
			sessions = Array.isArray(data) ? data : [];
			renderSessionSelect();

			// Auto-select first active session
			const active = sessions.find((s) => s.status === "active");
			if (active && !currentSessionId) {
				selectSession(active.id);
			} else {
				updateCloseSessionButtonVisibility();
			}
		} catch (e) {
			console.error("Failed to load sessions:", e);
		}
	}

	function renderSessionSelect() {
		if (!Array.isArray(sessions)) return;
		const active = sessions.filter((s) => s.status === "active");
		const closed = sessions.filter((s) => s.status === "closed");

		sessionSelect.innerHTML =
			'<option value="">-- Select a session --</option>';

		if (active.length > 0) {
			const optgroup = document.createElement("optgroup");
			optgroup.label = "Active Sessions";
			active.forEach((s) => {
				const opt = document.createElement("option");
				opt.value = s.id;
				const projectLabel = s.project ? `[${s.project}] ` : "";
				opt.textContent = projectLabel + s.goal;
				if (s.id === currentSessionId) opt.selected = true;
				optgroup.appendChild(opt);
			});
			sessionSelect.appendChild(optgroup);
		}

		if (closed.length > 0) {
			const optgroup = document.createElement("optgroup");
			optgroup.label = "Closed Sessions";
			closed.forEach((s) => {
				const opt = document.createElement("option");
				opt.value = s.id;
				const projectLabel = s.project ? `[${s.project}] ` : "";
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
		renderDecisionFeed();
	}

	function updateCloseSessionButtonVisibility() {
		const current = sessions.find((s) => s.id === currentSessionId);
		if (current && current.status === "active") {
			closeSessionBtn.style.display = "inline-flex";
		} else {
			closeSessionBtn.style.display = "none";
		}
		if (current) {
			deleteSessionBtn.style.display = "inline-flex";
		} else {
			deleteSessionBtn.style.display = "none";
		}
	}

	/* ─── Board ─── */
	async function refreshBoard() {
		if (!currentSessionId) {
			boardData = null;
			renderBoard();
			renderDecisionFeed();
			return;
		}

		try {
			const data = await apiFetch(`/api/sessions/${currentSessionId}/board`);
			boardData = data;
			renderBoard();
			renderDecisionFeed();
			refreshGraphIfVisible();
		} catch (e) {
			console.error("Failed to load board:", e);
			boardData = null;
			renderBoard();
			renderDecisionFeed();
		}
	}

	function renderBoard() {
		const sessionClosed =
			sessions.find((s) => s.id === currentSessionId)?.status === "closed";

		COLUMNS.forEach((col) => {
			const container = document.getElementById(`col-${col}`);
			const countEl = document.getElementById(`count-${col}`);
			const delegations = boardData?.columns?.[col] || [];

			container.innerHTML = "";

			// In backlog column, show "Add delegation" button at top
			if (col === "backlog" && currentSessionId) {
				if (!sessionClosed) {
					const addBtn = document.createElement("button");
					addBtn.className = "add-delegation-btn";
					addBtn.innerHTML = "➕ Add Delegation";
					addBtn.addEventListener("click", () => openDelegationModal());
					container.appendChild(addBtn);
				}
			}

			if (delegations.length === 0) {
				const empty = document.createElement("div");
				empty.className = "empty-state";
				empty.innerHTML = `
                    <span style="font-size: 1.5rem; margin-bottom: 4px; display:block;">✨</span>
                    No tasks in ${COLUMN_LABELS[col]}
                `;
				container.appendChild(empty);
			} else {
				delegations.forEach((d) => {
					const card = createCard(d, sessionClosed);
					container.appendChild(card);
				});
			}

			countEl.textContent = delegations.length;
		});
	}

	function escapeHtml(str) {
		const div = document.createElement("div");
		div.textContent = str;
		return div.innerHTML;
	}

	function createCard(d, sessionClosed) {
		const agent = AGENTS[d.agent] || {
			icon: "❓",
			label: d.agent,
			color: "#888",
		};

		const statusClass = d.status || "pending";
		const statusLabel = (d.status || "pending")
			.replace(/_/g, " ")
			.replace(/\b\w/g, (c) => c.toUpperCase());
		const elapsed = formatTimeAgo(d.started_at || d.created_at);
		const pendingAction = d.pending_action ?? false;
		const latestMsg = d.latest_activity || "";

		const card = document.createElement("div");
		card.className = `card card-agent-${d.agent}`;

		if (!sessionClosed) {
			card.draggable = true;
		}

		card.dataset.delegationId = d.id;
		card.dataset.id = d.id;

		// Agent header with colored circle
		const agentHeader = document.createElement("div");
		agentHeader.className = "card-agent";
		agentHeader.innerHTML = `
			<span class="agent-circle" style="--agent-color:${agent.color}">${agent.icon}</span>
			<span class="agent-label">${agent.label}</span>
			<span class="card-id">#${d.id.substring(0, 8)}</span>
		`;
		card.appendChild(agentHeader);

		// Title (bold task summary)
		const title = document.createElement("div");
		title.className = "card-title";
		title.textContent = d.task_summary;
		card.appendChild(title);

		// Status row: pill + elapsed time
		const statusRow = document.createElement("div");
		statusRow.className = "card-status-row";
		statusRow.innerHTML = `
			<span class="status-pill status-${statusClass}">${statusLabel}</span>
			<span class="status-elapsed">${elapsed}</span>
		`;
		card.appendChild(statusRow);

		// Pending Action Bar
		const actionBar = document.createElement("div");
		actionBar.className = "card-action-bar";
		actionBar.style.display = pendingAction ? "flex" : "none";
		actionBar.innerHTML = `
			<span class="action-badge">⚠️ Needs your input</span>
			<div class="action-latest-msg"></div>
			<div class="action-buttons">
				<button class="action-approve" data-action="approve">✅ Approve</button>
				<button class="action-reject" data-action="reject">❌ Reject</button>
				<button class="action-modify" data-action="modify">✏️ Modify</button>
			</div>
		`;
		actionBar.querySelector(".action-latest-msg").textContent = latestMsg;
		card.appendChild(actionBar);

		// Activity Toggle
		const activityToggle = document.createElement("button");
		activityToggle.className = "card-activity-toggle";
		activityToggle.setAttribute("data-action", "toggle-activity");
		activityToggle.textContent = "📋 Activity (0)";
		card.appendChild(activityToggle);

		// Hidden Activity Stream
		const activityStream = document.createElement("div");
		activityStream.className = "card-activity";
		activityStream.style.display = "none";
		activityStream.setAttribute("data-delegation-id", d.id);
		card.appendChild(activityStream);

		// Meta: time + badges
		const meta = document.createElement("div");
		meta.className = "card-meta";

		const timeSpan = document.createElement("span");
		timeSpan.className = "card-time";
		timeSpan.textContent = `Created ${formatTimeAgo(d.created_at)}`;
		meta.appendChild(timeSpan);

		const badgesContainer = document.createElement("div");
		badgesContainer.className = "card-badges";

		if (d.dependencies && d.dependencies.length > 0) {
			const depBadge = document.createElement("span");
			depBadge.className = "card-badge";
			depBadge.innerHTML = `🔗 ${d.dependencies.length}`;
			depBadge.title = `Depends on: ${d.dependencies.join(", ")}`;
			badgesContainer.appendChild(depBadge);
		}

		if (d.blocker) {
			const blockBadge = document.createElement("span");
			blockBadge.className = "card-badge blocked";
			blockBadge.innerHTML = `🚫 Blocked`;
			blockBadge.title = d.blocker;
			badgesContainer.appendChild(blockBadge);
		}

		meta.appendChild(badgesContainer);
		card.appendChild(meta);

		// Drag events
		if (!sessionClosed) {
			card.addEventListener("dragstart", (e) => {
				e.dataTransfer.setData("text/plain", d.id);
				card.classList.add("dragging");
			});

			card.addEventListener("dragend", () => {
				card.classList.remove("dragging");
			});
		}

		// Click event -> open detail modal
		card.addEventListener("click", (e) => {
			if (
				e.target.closest(".card-badge") ||
				e.target.closest(".card-action-bar button") ||
				e.target.closest(".card-activity-toggle") ||
				e.target.closest(".card-activity") ||
				card.classList.contains("dragging")
			) {
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
		const sessionClosed =
			sessions.find((s) => s.id === currentSessionId)?.status === "closed";
		const card = createCard(d, sessionClosed);
		const container = document.querySelector(
			`.column[data-column="${d.column}"] .column-cards`,
		);
		if (container) {
			container.appendChild(card);
			const countEl = document.querySelector(
				`.column[data-column="${d.column}"] .column-count`,
			);
			if (countEl) {
				countEl.textContent = container.children.length;
			}
		}
	}

	// @ts-expect-error - used by external integrations
	function _removeCardFromBoard(delegationId) {
		const card = findCard(delegationId);
		if (card) {
			const column = card.closest(".column");
			card.remove();
			if (column) {
				const countEl = column.querySelector(".column-count");
				const cardsContainer = column.querySelector(".column-cards");
				if (countEl && cardsContainer) {
					countEl.textContent = cardsContainer.children.length;
				}
			}
		}
	}

	function moveCardOnBoard(d) {
		const existingCard = findCard(d.id);
		if (existingCard) {
			const currentColumn = existingCard.closest(".column");
			const currentColumnName = currentColumn?.dataset.column;

			if (currentColumnName !== d.column) {
				// Different column: remove from old, add to new
				const oldContainer = currentColumn.querySelector(".column-cards");
				const oldCount = currentColumn.querySelector(".column-count");

				existingCard.remove();

				const sessionClosed =
					sessions.find((s) => s.id === currentSessionId)?.status === "closed";
				const newCard = createCard(d, sessionClosed);
				const newContainer = document.querySelector(
					`.column[data-column="${d.column}"] .column-cards`,
				);
				if (newContainer) {
					newContainer.appendChild(newCard);
				}

				if (oldCount && oldContainer)
					oldCount.textContent = oldContainer.children.length;
				const newColumn = document.querySelector(
					`.column[data-column="${d.column}"]`,
				);
				const newCount = newColumn?.querySelector(".column-count");
				const newCardsContainer = newColumn?.querySelector(".column-cards");
				if (newCount && newCardsContainer)
					newCount.textContent = newCardsContainer.children.length;
			} else {
				// Same column: just update the card in place
				const sessionClosed =
					sessions.find((s) => s.id === currentSessionId)?.status === "closed";
				const newCard = createCard(d, sessionClosed);
				existingCard.replaceWith(newCard);
			}
		} else {
			addCardToBoard(d);
		}
	}

	/* ─── Drag & Drop ─── */
	function setupDragDrop() {
		COLUMNS.forEach((col) => {
			const columnEl = document.querySelector(`[data-column="${col}"]`);
			if (!columnEl) return;

			columnEl.addEventListener("dragover", (e) => {
				const sessionClosed =
					sessions.find((s) => s.id === currentSessionId)?.status === "closed";
				if (sessionClosed) return;

				e.preventDefault();
				const cards = columnEl.querySelector(".column-cards");
				if (cards) cards.classList.add("drag-over");
			});

			columnEl.addEventListener("dragleave", () => {
				const cards = columnEl.querySelector(".column-cards");
				if (cards) cards.classList.remove("drag-over");
			});

			columnEl.addEventListener("drop", async (e) => {
				const sessionClosed =
					sessions.find((s) => s.id === currentSessionId)?.status === "closed";
				if (sessionClosed) return;

				e.preventDefault();
				const cards = columnEl.querySelector(".column-cards");
				if (cards) cards.classList.remove("drag-over");

				const delegationId = e.dataTransfer.getData("text/plain");
				if (!delegationId) return;

				const targetColumn = columnEl.dataset.column;

				try {
					await apiFetch(`/api/delegations/${delegationId}`, {
						method: "PATCH",
						body: JSON.stringify({ column: targetColumn }),
					});
					showToast(
						`Moved delegation to <strong>${COLUMN_LABELS[targetColumn]}</strong>`,
					);
					// Board will be refreshed via WebSocket update
				} catch (e) {
					showToast(`Error: ${e.message}`);
				}
			});
		});
	}

	/* ─── Modals ─── */
	function openSessionModal() {
		sessionModal.style.display = "flex";
		sessionGoal.value = "";
		sessionGoal.focus();
	}

	function closeSessionModal() {
		sessionModal.style.display = "none";
	}

	function openDelegationModal() {
		if (!currentSessionId) {
			showToast("Please select a session first");
			return;
		}
		delegationModal.style.display = "flex";
		delegationTask.value = "";
		delegationDeps.value = "";
		delegationTask.focus();
	}

	function closeDelegationModal() {
		delegationModal.style.display = "none";
	}

	/* ─── Details Modal ─── */
	function openDetailsModal(d) {
		detailsId.textContent = d.id;

		const agent = AGENTS[d.agent] || { icon: "❓", label: d.agent };
		detailsAgent.innerHTML = `<span class="card-agent-badge">${agent.icon} &nbsp;${agent.label}</span>`;

		detailsColumn.className = `meta-value badge ${d.column}`;
		detailsColumn.textContent = COLUMN_LABELS[d.column] || d.column;

		detailsStatus.className = `meta-value badge ${d.column}`;
		detailsStatus.textContent = d.status;

		detailsTaskSummary.textContent = d.task_summary;
		$("#details-delegation-id").value = d.id;

		// Check blocker state
		if (d.blocker) {
			detailsBlockedCheckbox.checked = true;
			detailsBlockerContainer.style.display = "block";
			detailsBlockerInput.value = d.blocker;
		} else {
			detailsBlockedCheckbox.checked = false;
			detailsBlockerContainer.style.display = "none";
			detailsBlockerInput.value = "";
		}

		// Handoff preview
		detailsHandoffInput.value = d.handoff_preview || "";

		// Handle disabled inputs if session is closed
		const sessionClosed =
			sessions.find((s) => s.id === currentSessionId)?.status === "closed";
		const submitBtn = detailsForm.querySelector('button[type="submit"]');
		if (sessionClosed) {
			detailsBlockedCheckbox.disabled = true;
			detailsBlockerInput.disabled = true;
			detailsHandoffInput.disabled = true;
			if (submitBtn) submitBtn.style.display = "none";
		} else {
			detailsBlockedCheckbox.disabled = false;
			detailsBlockerInput.disabled = false;
			detailsHandoffInput.disabled = false;
			if (submitBtn) submitBtn.style.display = "inline-flex";
		}

		detailsModal.style.display = "flex";
	}

	function closeDetailsModal() {
		detailsModal.style.display = "none";
	}

	// Toggle Blocker Reason input visibility
	detailsBlockedCheckbox.addEventListener("change", () => {
		if (detailsBlockedCheckbox.checked) {
			detailsBlockerContainer.style.display = "block";
			detailsBlockerInput.focus();
		} else {
			detailsBlockerContainer.style.display = "none";
			detailsBlockerInput.value = "";
		}
	});

	/* ─── Event Listeners ─── */
	// Session select change
	sessionSelect.addEventListener("change", () => {
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
	newSessionBtn.addEventListener("click", openSessionModal);

	// Close session button
	closeSessionBtn.addEventListener("click", async () => {
		if (!currentSessionId) return;
		if (
			!confirm(
				"Are you sure you want to close this session? Closed sessions cannot add new delegations or drag tasks.",
			)
		) {
			return;
		}

		try {
			await apiFetch(`/api/sessions/${currentSessionId}`, {
				method: "PATCH",
				body: JSON.stringify({ status: "closed" }),
			});
			showToast("Session closed successfully!");
		} catch (e) {
			showToast(`Error: ${e.message}`);
		}
	});

	deleteSessionBtn.addEventListener("click", async () => {
		if (!currentSessionId) return;
		if (
			!confirm(
				"⚠️ Are you sure you want to DELETE this session? This will permanently remove the session and all its delegations. This action cannot be undone.",
			)
		) {
			return;
		}

		try {
			await apiFetch(`/api/sessions/${currentSessionId}`, {
				method: "DELETE",
			});
			currentSessionId = null;
			boardData = null;
			await loadSessions();
			renderBoard();
			showToast("Session deleted successfully!");
		} catch (e) {
			showToast(`Error: ${e.message}`);
		}
	});

	// Session form submit
	sessionForm.addEventListener("submit", async (e) => {
		e.preventDefault();
		const project = sessionProject.value.trim();
		const goal = sessionGoal.value.trim();
		if (!project || !goal) return;

		try {
			const newSession = await apiFetch("/api/sessions", {
				method: "POST",
				body: JSON.stringify({ project, goal }),
			});
			closeSessionModal();
			selectSession(newSession.id);
		} catch (e) {
			showToast(`Error: ${e.message}`);
		}
	});

	sessionModalCancel.addEventListener("click", closeSessionModal);
	sessionModalCloseIcon.addEventListener("click", closeSessionModal);

	// Delegation form submit
	delegationForm.addEventListener("submit", async (e) => {
		e.preventDefault();
		const agent = delegationAgent.value;
		const task = delegationTask.value.trim();
		const deps = delegationDeps.value.trim()
			? delegationDeps.value
					.split(",")
					.map((s) => s.trim())
					.filter(Boolean)
			: [];

		if (!agent || !task) return;

		try {
			await apiFetch("/api/delegations", {
				method: "POST",
				body: JSON.stringify({
					session_id: currentSessionId,
					agent,
					task_summary: task,
					dependencies: deps,
				}),
			});
			closeDelegationModal();
			showToast("Delegation created successfully!");
		} catch (e) {
			showToast(`Error: ${e.message}`);
		}
	});

	delegationModalCancel.addEventListener("click", closeDelegationModal);
	delegationModalCloseIcon.addEventListener("click", closeDelegationModal);

	// Details form submit
	detailsForm.addEventListener("submit", async (e) => {
		e.preventDefault();
		const id = $("#details-delegation-id").value;
		const handoff = detailsHandoffInput.value.trim();

		let blocker = "";
		if (detailsBlockedCheckbox.checked) {
			blocker = detailsBlockerInput.value.trim() || "Blocked";
		}

		try {
			await apiFetch(`/api/delegations/${id}`, {
				method: "PATCH",
				body: JSON.stringify({
					blocker: blocker,
					handoff_preview: handoff,
				}),
			});
			closeDetailsModal();
			showToast("Delegation updated successfully!");
		} catch (e) {
			showToast(`Error: ${e.message}`);
		}
	});

	detailsModalCancel.addEventListener("click", closeDetailsModal);
	detailsModalCloseIcon.addEventListener("click", closeDetailsModal);

	// Close modals on Escape
	document.addEventListener("keydown", (e) => {
		if (e.key === "Escape") {
			closeSessionModal();
			closeDelegationModal();
			closeDetailsModal();
		}
	});

	// Close modals on backdrop click
	[sessionModal, delegationModal, detailsModal].forEach((modal) => {
		if (modal) {
			modal.addEventListener("click", (e) => {
				if (e.target === modal) {
					modal.style.display = "none";
				}
			});
		}
	});

	// Close session modal on X button and Cancel button
	const closeModalBtn = $("#close-modal-btn");
	const cancelSessionBtn = $("#cancel-session-btn");
	if (closeModalBtn)
		closeModalBtn.addEventListener(
			"click",
			() => (sessionModal.style.display = "none"),
		);
	if (cancelSessionBtn)
		cancelSessionBtn.addEventListener(
			"click",
			() => (sessionModal.style.display = "none"),
		);

	/* ─── Dependency Graph ─── */
	let graphVisible = false;
	let graphData = null; // stored for node click lookup

	const GRAPH_COLORS = {
		done: "#2ed573",
		running: "#5e60f0",
		in_progress: "#5e60f0",
		review: "#ffa502",
		changes: "#ffa502",
		blocked: "#ff4757",
		pending: "#555",
	};

	function setupGraphToggle() {
		const toggleBtn = document.getElementById("graph-toggle-btn");
		const closeBtn = document.getElementById("graph-close-btn");

		if (toggleBtn) {
			toggleBtn.addEventListener("click", () => {
				graphVisible = !graphVisible;
				const panel = document.getElementById("graph-panel");
				if (graphVisible) {
					panel.style.display = "block";
					toggleBtn.classList.add("active");
					refreshGraph();
				} else {
					panel.style.display = "none";
					toggleBtn.classList.remove("active");
				}
			});
		}

		if (closeBtn) {
			closeBtn.addEventListener("click", () => {
				graphVisible = false;
				document.getElementById("graph-panel").style.display = "none";
				if (toggleBtn) toggleBtn.classList.remove("active");
			});
		}
	}

	async function refreshGraph() {
		if (!currentSessionId || !graphVisible) return;

		try {
			const data = await apiFetch(`/api/sessions/${currentSessionId}/graph`);
			graphData = data;
			renderGraph(data);
		} catch (e) {
			console.error("Failed to load graph:", e);
		}
	}

	function refreshGraphIfVisible() {
		if (graphVisible) refreshGraph();
	}

	function renderGraph(data) {
		const svg = document.getElementById("graph-svg");
		const statsEl = document.getElementById("graph-stats");
		if (!svg || !data) return;

		const nodes = data.nodes || [];
		const edges = data.edges || [];

		// Update stats
		if (statsEl) {
			const blocked = nodes.filter((n) => n.status === "blocked").length;
			statsEl.textContent = `${nodes.length} tasks · ${edges.length} dependencies${blocked > 0 ? ` · ${blocked} blocked` : ""}`;
		}

		if (nodes.length === 0) {
			svg.innerHTML = `<text x="50%" y="50%" text-anchor="middle" fill="#888" font-size="14">No delegations to display</text>`;
			return;
		}

		// ─── Layout ───
		// Group nodes by column for a left-to-right swimlane layout
		const columnOrder = ["backlog", "ready", "in_progress", "review", "done"];
		const columnGroups = {};
		columnOrder.forEach((c) => (columnGroups[c] = []));
		nodes.forEach((n) => {
			const col = columnGroups[n.column] ? n.column : "backlog";
			columnGroups[col].push(n);
		});

		const NODE_W = 160;
		const NODE_H = 56;
		const COL_GAP = 60;
		const ROW_GAP = 20;
		const PADDING = 30;

		// Calculate positions
		const positions = {};
		let maxX = 0;
		let maxY = 0;
		let colX = PADDING;

		columnOrder.forEach((colName) => {
			const colNodes = columnGroups[colName];
			if (colNodes.length === 0) {
				colX += NODE_W + COL_GAP;
				return;
			}
			let y = PADDING;
			colNodes.forEach((node) => {
				positions[node.id] = { x: colX, y: y };
				if (colX + NODE_W > maxX) maxX = colX + NODE_W;
				if (y + NODE_H > maxY) maxY = y + NODE_H;
				y += NODE_H + ROW_GAP;
			});
			colX += NODE_W + COL_GAP;
		});

		const svgW = Math.max(maxX + PADDING, 600);
		const svgH = Math.max(maxY + PADDING, 380);

		// Build SVG content
		let svgContent = "";

		// Arrow marker
		svgContent += `
			<defs>
				<marker id="arrowhead" markerWidth="10" markerHeight="7" refX="10" refY="3.5" orient="auto">
					<polygon points="0 0, 10 3.5, 0 7" fill="#666" />
				</marker>
			</defs>`;

		// Column labels (swimlane headers)
		const colLabels = {
			backlog: "Backlog",
			ready: "Ready",
			in_progress: "In Progress",
			review: "Review",
			done: "Done",
		};
		let labelX = PADDING;
		columnOrder.forEach((colName) => {
			const colNodes = columnGroups[colName];
			const colWidth = colNodes.length > 0 ? NODE_W : NODE_W;
			svgContent += `<text x="${labelX + colWidth / 2}" y="18" text-anchor="middle" fill="#666" font-size="10" font-weight="700" text-transform="uppercase" letter-spacing="1">${colLabels[colName] || colName}</text>`;
			labelX += NODE_W + COL_GAP;
		});

		// Draw edges first (behind nodes)
		const nodeMap = {};
		nodes.forEach((n) => (nodeMap[n.id] = n));

		edges.forEach((edge) => {
			const from = positions[edge.from];
			const to = positions[edge.to];
			if (!from || !to) return;

			const x1 = from.x + NODE_W;
			const y1 = from.y + NODE_H / 2;
			const x2 = to.x;
			const y2 = to.y + NODE_H / 2;

			// Bezier curve for smooth edges
			const dx = Math.abs(x2 - x1) * 0.4;
			const cp1x = x1 + dx;
			const cp2x = x2 - dx;

			const fromNode = nodeMap[edge.from];
			const edgeColor = fromNode
				? GRAPH_COLORS[fromNode.status] || "#555"
				: "#555";

			svgContent += `<path class="graph-edge" d="M${x1},${y1} C${cp1x},${y1} ${cp2x},${y2} ${x2},${y2}" stroke="${edgeColor}" stroke-opacity="0.4" />`;
		});

		// Draw nodes
		nodes.forEach((node) => {
			const pos = positions[node.id];
			if (!pos) return;

			const color = GRAPH_COLORS[node.status] || "#555";
			const agent = AGENTS[node.agent] || { icon: "❓", label: node.agent };
			const shortId = node.id.substring(0, 8);

			// Truncate task summary
			const maxLen = 22;
			const summary =
				node.task_summary.length > maxLen
					? node.task_summary.substring(0, maxLen) + "..."
					: node.task_summary;

			svgContent += `
				<g class="graph-node" data-id="${node.id}">
					<rect class="graph-node-rect" x="${pos.x}" y="${pos.y}" width="${NODE_W}" height="${NODE_H}"
						fill="${color}22" stroke="${color}" stroke-width="2" />
					<circle cx="${pos.x + 14}" cy="${pos.y + 16}" r="9" fill="${color}33" />
					<text class="graph-node-agent" x="${pos.x + 14}" y="${pos.y + 19}" text-anchor="middle">${agent.icon}</text>
					<text class="graph-node-label" x="${pos.x + 28}" y="${pos.y + 19}">${escapeHtml(summary)}</text>
					<text class="graph-node-id" x="${pos.x + NODE_W - 6}" y="${pos.y + 14}" text-anchor="end">#${shortId}</text>
					<text class="graph-node-status" x="${pos.x + 8}" y="${pos.y + NODE_H - 8}" fill="${color}">${node.status.toUpperCase()}</text>
				</g>`;
		});

		svg.setAttribute("viewBox", `0 0 ${svgW} ${svgH}`);
		svg.setAttribute("width", svgW);
		svg.setAttribute("height", svgH);
		svg.innerHTML = svgContent;

		// Click on node to open detail modal
		svg.querySelectorAll(".graph-node").forEach((nodeEl) => {
			nodeEl.addEventListener("click", () => {
				const id = nodeEl.dataset.id;
				// Find the delegation in boardData
				if (boardData && boardData.columns) {
					for (const col of Object.values(boardData.columns)) {
						const d = col.find((d) => d.id === id);
						if (d) {
							openDetailsModal(d);
							return;
						}
					}
				}
			});
		});
	}

	/* ─── Init ─── */
	async function init() {
		connectWS();
		setupDragDrop();
		setupTabs();
		setupConfig();
		setupGraphToggle();
		createDecisionFeedSidebar();
		setupBoardEventDelegation();
		await loadSessions();

		// Periodic refresh every 30s as fallback
		setInterval(() => {
			if (currentSessionId) {
				refreshBoard();
				renderDecisionFeed();
			}
		}, 30000);
	}

	// --- Decision Feed Sidebar ---
	function createDecisionFeedSidebar() {
		// Avoid duplicates
		if (document.getElementById("decision-feed")) return;

		const feed = document.createElement("div");
		feed.id = "decision-feed";
		feed.style.display = "none";
		feed.innerHTML = `
			<div class="feed-header">
				<h3>📋 Pending Decisions</h3>
				<span class="feed-badge" style="display:none">0</span>
			</div>
			<div class="feed-list"></div>
		`;
		document.body.appendChild(feed);
	}

	// --- Board Event Delegation ---
	function setupBoardEventDelegation() {
		document.getElementById("board")?.addEventListener("click", (e) => {
			const toggle = e.target.closest("[data-action='toggle-activity']");
			if (toggle) {
				const card = toggle.closest(".card");
				if (!card) return;
				const stream = card.querySelector(".card-activity");
				if (!stream) return;

				if (stream.style.display === "none") {
					// Load activities
					const delegationId = card.dataset.delegationId;
					fetchActivities(delegationId)
						.then((activities) => {
							if (!Array.isArray(activities)) return;
							stream.innerHTML = "";
							const MAX_VISIBLE = 3;
							activities.forEach((a, i) => {
								const entry = createActivityEntry(a);
								if (i >= MAX_VISIBLE) entry.classList.add("hidden-event");
								stream.appendChild(entry);
							});
							// "Show all N events" button
							if (activities.length > MAX_VISIBLE) {
								const showAll = document.createElement("button");
								showAll.className = "activity-show-all";
								showAll.textContent = `Show all ${activities.length} events...`;
								showAll.addEventListener("click", (ev) => {
									ev.stopPropagation();
									stream
										.querySelectorAll(".hidden-event")
										.forEach((el) => el.classList.add("visible"));
									showAll.remove();
									stream.style.maxHeight = "360px";
								});
								stream.appendChild(showAll);
							}
							stream.style.display = "block";
							toggle.textContent = `📋 Hide Activity (${activities.length})`;
						})
						.catch((err) => {
							console.error("Failed to load activities:", err);
						});
				} else {
					stream.style.display = "none";
					stream.style.maxHeight = "240px";
					// Count only actual activity events (not the show-all button)
					const count = stream.querySelectorAll(".activity-event").length;
					toggle.textContent = `📋 Activity (${count})`;
				}
				return;
			}

			const actionBtn = e.target.closest(
				".action-approve, .action-reject, .action-modify",
			);
			if (actionBtn) {
				const card = actionBtn.closest(".card");
				if (!card) return;
				const delegationId = card.dataset.delegationId;
				const action = actionBtn.dataset.action;

				const resolutionMap = {
					approve: "Approved via UI",
					reject: "Rejected via UI",
					modify: "Needs modification",
				};
				const resolution = resolutionMap[action] || action;

				// Get the pending activity ID from the action bar's parent context
				// For simplicity, resolve the latest pending activity
				fetchActivities(delegationId)
					.then((activities) => {
						if (!Array.isArray(activities) || activities.length === 0) return;
						// Find the latest unresolved activity
						const pending = activities.find((a) => !a.resolved_at);
						if (!pending) return;
						return resolveActivity(delegationId, pending.id, resolution);
					})
					.then(() => {
						// Hide the action bar
						const actionBar = card.querySelector(".card-action-bar");
						if (actionBar) actionBar.style.display = "none";
						showToast(`Action "${action}" sent`);
					})
					.catch((err) => {
						console.error("Failed to resolve activity:", err);
					});
				return;
			}
		});
	}

	// --- Tab System ---
	function setupTabs() {
		document.querySelectorAll(".tab-btn").forEach((btn) => {
			btn.addEventListener("click", () => {
				const tab = btn.dataset.tab;

				// Update buttons
				document
					.querySelectorAll(".tab-btn")
					.forEach((b) => b.classList.remove("active"));
				btn.classList.add("active");

				// Update content
				document
					.querySelectorAll(".tab-content")
					.forEach((c) => c.classList.remove("active"));
				document.getElementById(`tab-${tab}`).classList.add("active");

				// Load config data when switching to config tab
				if (tab === "config") {
					loadConfigData();
				}
			});
		});
	}

	// --- Config System ---
	const configData = {
		mcp: [],
		agents: [],
		skills: [],
		json: null,
		providers: {},
	};
	let currentAgent = null;

	function setupConfig() {
		// Config nav
		document.querySelectorAll(".config-nav-btn").forEach((btn) => {
			btn.addEventListener("click", () => {
				const section = btn.dataset.section;
				document
					.querySelectorAll(".config-nav-btn")
					.forEach((b) => b.classList.remove("active"));
				btn.classList.add("active");
				document
					.querySelectorAll(".config-section")
					.forEach((s) => s.classList.remove("active"));
				document.getElementById(`section-${section}`).classList.add("active");
				if (section === "providers") loadProviders();
				if (section === "permissions") loadPermissions();
			});
		});

		// Agent editor
		document
			.getElementById("agent-cancel-btn")
			.addEventListener("click", () => {
				document.getElementById("agent-editor").style.display = "none";
				currentAgent = null;
			});

		document
			.getElementById("agent-save-btn")
			.addEventListener("click", saveAgent);

		document
			.getElementById("agent-delete-btn")
			.addEventListener("click", () => {
				if (currentAgent) {
					deleteAgent(currentAgent);
				}
			});

		// New agent
		document.getElementById("new-agent-btn").addEventListener("click", () => {
			const name = prompt("Agent name:");
			if (name) {
				currentAgent = name;
				document.getElementById("agent-editor-title").textContent =
					`New: ${name}`;
				document.getElementById("agent-content").value =
					`# ${name}\n\nYou are a ${name} agent.\n\n## Role\nDescribe your role here.\n`;
				document.getElementById("agent-delete-btn").style.display = "none";
				document.getElementById("agent-editor").style.display = "flex";
			}
		});

		// JSON save
		document
			.getElementById("save-json-btn")
			.addEventListener("click", saveJson);
	}

	async function loadConfigData() {
		await Promise.all([
			loadMCP(),
			loadAgents(),
			loadSkills(),
			loadJson(),
			loadProviders(),
		]);
	}

	// --- MCP ---
	async function loadMCP() {
		try {
			const res = await fetch("/api/config/mcp");
			configData.mcp = await res.json();
			renderMCP();
		} catch (e) {
			console.error("Failed to load MCP:", e);
		}
	}

	function renderMCP() {
		const list = document.getElementById("mcp-list");
		list.innerHTML = configData.mcp
			.map((mcp) => {
				let type = "unknown";
				let detailsHtml = "";
				try {
					const cfg = JSON.parse(
						typeof mcp.config === "string"
							? mcp.config
							: JSON.stringify(mcp.config),
					);
					type = cfg.type || "local";

					const details = [];
					if (cfg.url) {
						details.push(
							`<strong>URL:</strong> <span class="code-font">${cfg.url}</span>`,
						);
					}
					if (cfg.command && cfg.command.length > 0) {
						const cmdStr = Array.isArray(cfg.command)
							? cfg.command.join(" ")
							: cfg.command;
						details.push(
							`<strong>Command:</strong> <span class="code-font">${cmdStr}</span>`,
						);
					}
					if (cfg.args && cfg.args.length > 0) {
						details.push(
							`<strong>Args:</strong> <span class="code-font">${cfg.args.join(" ")}</span>`,
						);
					}
					if (cfg.env && Object.keys(cfg.env).length > 0) {
						const envStrings = Object.entries(cfg.env).map(
							([k, v]) => `${k}=${v}`,
						);
						details.push(
							`<strong>Env:</strong> <span class="code-font">${envStrings.join(", ")}</span>`,
						);
					}

					for (const [key, val] of Object.entries(cfg)) {
						if (
							[
								"type",
								"url",
								"command",
								"args",
								"env",
								"enabled",
								"disabled",
							].includes(key)
						)
							continue;

						if (typeof val === "object" && val !== null) {
							details.push(
								`<strong>${key}:</strong> <span class="code-font">${JSON.stringify(val)}</span>`,
							);
						} else {
							let displayVal = val;
							if (
								key.toLowerCase().includes("key") ||
								key.toLowerCase().includes("secret") ||
								key.toLowerCase().includes("token")
							) {
								if (typeof val === "string" && val.length > 8) {
									displayVal =
										val.substring(0, 4) + "..." + val.substring(val.length - 4);
								}
							}
							details.push(
								`<strong>${key}:</strong> <span class="code-font">${displayVal}</span>`,
							);
						}
					}

					if (details.length > 0) {
						detailsHtml = `<div class="mcp-details">${details.join("<br>")}</div>`;
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
                            <input type="checkbox" ${mcp.enabled ? "checked" : ""} onchange="window.kanbanApp.toggleMCP('${mcp.name}', this.checked)">
                            <span class="toggle-slider"></span>
                        </label>
                    </div>
                    ${detailsHtml}
                </div>
            `;
			})
			.join("");
	}

	async function toggleMCP(name, enabled) {
		try {
			await fetch(`/api/config/mcp/${name}`, {
				method: "PUT",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ enabled }),
			});
			showToast(
				`${name} ${enabled ? "enabled" : "disabled"}. Restart opencode to apply.`,
			);
		} catch (e) {
			console.error("Failed to toggle MCP:", e);
			showToast("Failed to toggle MCP", "error");
		}
	}

	// Expose toggleMCP to global scope
	window.kanbanApp.toggleMCP = toggleMCP;

	// --- Providers ---
	async function loadProviders() {
		try {
			const res = await fetch("/api/config/providers");
			configData.providers = await res.json();
			renderProviders();
		} catch (e) {
			console.error("Failed to load providers:", e);
		}
	}

	function renderProviders() {
		const container = document.getElementById("providers-list");
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
			// Handle nested options structure from opencode.json
			const options = config.options || {};
			const hasKey = !!(config.apiKey || options.apiKey);
			const baseUrl = config.baseUrl || options.baseURL || "";
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
                    <div class="provider-card-url">${baseUrl || "No base URL configured"}</div>
                </div>`;
		}
		html += "</div>";
		container.innerHTML = html;
	}

	async function addProvider() {
		document.getElementById("provider-editor-title").textContent =
			"Add Provider";
		document.getElementById("provider-name-input").value = "";
		document.getElementById("provider-name-input").disabled = false;
		document.getElementById("provider-baseurl-input").value = "";
		document.getElementById("provider-apikey-input").value = "";
		document.getElementById("provider-models-input").value = "";
		document.getElementById("provider-delete-btn").style.display = "none";
		showProviderEditor(false);
	}

	async function editProvider(name) {
		try {
			const res = await fetch("/api/config/providers");
			const providers = await res.json();
			const p = providers[name];
			if (!p) return;
			// Handle nested options structure from opencode.json
			const options = p.options || {};
			document.getElementById("provider-editor-title").textContent =
				`Edit: ${name}`;
			document.getElementById("provider-name-input").value = name;
			document.getElementById("provider-name-input").disabled = true;
			document.getElementById("provider-baseurl-input").value =
				p.baseUrl || options.baseURL || "";
			document.getElementById("provider-apikey-input").value =
				p.apiKey || options.apiKey || "";
			document.getElementById("provider-models-input").value = JSON.stringify(
				p.models || {},
				null,
				2,
			);
			document.getElementById("provider-delete-btn").style.display =
				"inline-flex";
			showProviderEditor(true);
		} catch (e) {
			console.error("Failed to load provider for edit:", e);
		}
	}

	function showProviderEditor(isEdit = false) {
		// Reset eye toggle
		const apiInput = document.getElementById("provider-apikey-input");
		const eyeOn = document.getElementById("apikey-eye-icon");
		const eyeOff = document.getElementById("apikey-eye-off-icon");
		if (apiInput) apiInput.type = "password";
		if (eyeOn) eyeOn.style.display = "";
		if (eyeOff) eyeOff.style.display = "none";
		// Reset JSON status
		validateModelsJson(
			document.getElementById("provider-models-input")?.value || "",
		);

		// Remove any success overlay
		const existingOverlay = document.querySelector(".save-success-overlay");
		if (existingOverlay) existingOverlay.remove();

		// Show restart banner for edits
		const restartBanner = document.getElementById("restart-banner");
		if (restartBanner) {
			restartBanner.style.display = isEdit ? "flex" : "none";
		}

		const overlay = document.getElementById("provider-modal-overlay");
		overlay.classList.add("active");
		setTimeout(() => {
			const firstInput = document.getElementById("provider-name-input");
			if (firstInput && !firstInput.disabled) firstInput.focus();
		}, 100);
	}

	async function saveProvider() {
		const nameInput = document.getElementById("provider-name-input");
		const name = nameInput.value.trim();
		if (!name) {
			showToast("Provider name is required", "error");
			return;
		}

		let models = {};
		const modelsText = document
			.getElementById("provider-models-input")
			.value.trim();
		if (modelsText) {
			try {
				models = JSON.parse(modelsText);
			} catch {
				showToast("Invalid models JSON", "error");
				return;
			}
		}

		const baseUrl = document
			.getElementById("provider-baseurl-input")
			.value.trim();
		const apiKey = document
			.getElementById("provider-apikey-input")
			.value.trim();

		// Fetch existing provider to preserve structure (name, npm, options)
		let existingProvider = {};
		try {
			const existingRes = await fetch("/api/config/providers");
			const existingProviders = await existingRes.json();
			existingProvider = existingProviders[name] || {};
		} catch (e) {
			console.error("Failed to fetch existing provider:", e);
		}

		// Preserve existing structure and update UI-managed fields
		const provider = {
			...existingProvider,
			models: models,
			options: {
				...(existingProvider.options || {}),
				apiKey: apiKey,
				baseURL: baseUrl,
			},
		};

		const saveBtn = document.getElementById("provider-save-btn");
		const originalText = saveBtn.innerHTML;
		saveBtn.disabled = true;
		saveBtn.innerHTML =
			'<svg class="spinner" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><circle cx="12" cy="12" r="10" stroke-dasharray="31.416" stroke-dashoffset="10"></circle></svg> Saving...';

		try {
			const res = await fetch(
				`/api/config/providers/${encodeURIComponent(name)}`,
				{
					method: "PUT",
					headers: { "Content-Type": "application/json" },
					body: JSON.stringify(provider),
				},
			);

			if (res.ok) {
				// Show success feedback
				showSaveSuccess(name);
				loadProviders();
			} else {
				const err = await res.json();
				showToast(err.error || "Failed to save provider", "error");
				saveBtn.disabled = false;
				saveBtn.innerHTML = originalText;
			}
		} catch (e) {
			console.error("Failed to save provider:", e);
			showToast("Failed to save provider", "error");
			saveBtn.disabled = false;
			saveBtn.innerHTML = originalText;
		}
	}

	function showSaveSuccess() {
		const editor = document.getElementById("provider-editor");

		// Create success overlay
		const overlay = document.createElement("div");
		overlay.className = "save-success-overlay";
		overlay.innerHTML = `
            <div class="save-success-icon">
                <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="20 6 9 17 4 12"></polyline>
                </svg>
            </div>
            <div class="save-success-text">Provider saved successfully!</div>
            <div class="save-success-hint">Restart opencode to apply changes</div>
        `;

		editor.style.position = "relative";
		editor.appendChild(overlay);

		// Auto-close after 1.5s
		setTimeout(() => {
			cancelProviderEdit();
		}, 1500);
	}

	function cancelProviderEdit() {
		const overlay = document.getElementById("provider-modal-overlay");
		overlay.classList.remove("active");
		// Wait for animation to finish before hiding
		setTimeout(() => {
			overlay.style.display = "none";
		}, 300);
	}

	async function deleteProvider(name) {
		if (!name)
			name = document.getElementById("provider-name-input").value.trim();
		if (!name) return;
		if (!confirm(`Delete provider "${name}"?`)) return;

		try {
			const res = await fetch(
				`/api/config/providers/${encodeURIComponent(name)}`,
				{ method: "DELETE" },
			);
			if (res.ok) {
				showToast("Provider deleted", "success");
				cancelProviderEdit();
				loadProviders();
			} else {
				const err = await res.json();
				showToast(err.error || "Failed to delete provider", "error");
			}
		} catch (e) {
			console.error("Failed to delete provider:", e);
			showToast("Failed to delete provider", "error");
		}
	}

	function toggleApiKeyVisibility() {
		const input = document.getElementById("provider-apikey-input");
		const eyeOn = document.getElementById("apikey-eye-icon");
		const eyeOff = document.getElementById("apikey-eye-off-icon");
		const isHidden = input.type === "password";
		input.type = isHidden ? "text" : "password";
		eyeOn.style.display = isHidden ? "none" : "";
		eyeOff.style.display = isHidden ? "" : "none";
	}

	function validateModelsJson(value) {
		const wrapper = document.getElementById("models-textarea-wrapper");
		const status = document.getElementById("json-status");
		if (!value || !value.trim()) {
			wrapper.classList.remove("invalid");
			status.className = "json-status";
			status.textContent = "";
			return true;
		}
		try {
			JSON.parse(value);
			wrapper.classList.remove("invalid");
			status.className = "json-status valid";
			status.textContent = "✓ Valid JSON";
			return true;
		} catch {
			wrapper.classList.add("invalid");
			status.className = "json-status invalid";
			status.textContent = "✗ Invalid JSON";
			return false;
		}
	}

	function formatModelsJson() {
		const ta = document.getElementById("provider-models-input");
		const raw = ta.value.trim();
		if (!raw) return;
		try {
			ta.value = JSON.stringify(JSON.parse(raw), null, 2);
			validateModelsJson(ta.value);
		} catch {
			validateModelsJson(raw);
			showToast("Cannot format — JSON is invalid", "error");
		}
	}

	function clearModelsJson() {
		const ta = document.getElementById("provider-models-input");
		ta.value = "";
		validateModelsJson("");
	}

	// --- Agents ---
	async function loadAgents() {
		try {
			const res = await fetch("/api/config/agents");
			configData.agents = await res.json();
			renderAgents();
		} catch (e) {
			console.error("Failed to load agents:", e);
		}
	}

	function renderAgents() {
		const list = document.getElementById("agents-list");
		list.innerHTML = configData.agents
			.map(
				(agent) => `
            <div class="agent-item" onclick="window.kanbanApp.editAgent('${agent.name}')">
                <span class="agent-item-name">${agent.name}</span>
                <div style="display: flex; align-items: center; gap: 8px;">
                    <span class="agent-item-size">${formatSize(agent.size)}</span>
                    <button class="agent-delete-btn" onclick="event.stopPropagation(); window.kanbanApp.deleteAgent('${agent.name}')">🗑️</button>
                </div>
            </div>
        `,
			)
			.join("");
	}

	async function editAgent(name) {
		try {
			const res = await fetch(`/api/config/agents/${name}`);
			const data = await res.json();
			currentAgent = name;
			document.getElementById("agent-editor-title").textContent =
				`Edit: ${name}`;
			document.getElementById("agent-content").value = data.content;
			document.getElementById("agent-delete-btn").style.display = "inline-flex";
			document.getElementById("agent-editor").style.display = "flex";

			// Highlight active
			document
				.querySelectorAll(".agent-item")
				.forEach((item) => item.classList.remove("active"));
			const activeItem = Array.from(
				document.querySelectorAll(".agent-item"),
			).find((item) => item.textContent.includes(name));
			if (activeItem) activeItem.classList.add("active");
		} catch (e) {
			console.error("Failed to load agent:", e);
		}
	}

	async function saveAgent() {
		if (!currentAgent) return;

		const content = document.getElementById("agent-content").value;
		try {
			const res = await fetch(`/api/config/agents/${currentAgent}`, {
				method: "PUT",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ content }),
			});

			if (res.ok) {
				showToast(`Agent "${currentAgent}" saved`);
				document.getElementById("agent-editor").style.display = "none";
				loadAgents();
			} else {
				const err = await res.json();
				showToast(err.error || "Failed to save", "error");
			}
		} catch (e) {
			console.error("Failed to save agent:", e);
			showToast("Failed to save agent", "error");
		}
	}

	async function deleteAgent(name) {
		if (!confirm(`Are you sure you want to delete the agent "${name}"?`)) {
			return;
		}
		try {
			const res = await fetch(`/api/config/agents/${name}`, {
				method: "DELETE",
			});
			if (res.ok) {
				showToast(`Agent "${name}" deleted`);
				document.getElementById("agent-editor").style.display = "none";
				currentAgent = null;
				loadAgents();
			} else {
				const err = await res.json();
				showToast(err.error || "Failed to delete agent", "error");
			}
		} catch (e) {
			console.error("Failed to delete agent:", e);
			showToast("Failed to delete agent", "error");
		}
	}

	// --- Permissions ---
	async function loadPermissions() {
		const container = document.getElementById("permissions-container");
		const loading = document.getElementById("permissions-loading");
		container.innerHTML = "";
		loading.style.display = "flex";

		try {
			// Fetch available tools (built-in + MCP discovered + plugin discovered)
			const toolsRes = await fetch("/api/config/tools");
			const toolsData = await toolsRes.json();
			const allTools = toolsData.all || [];
			const mcpTools = toolsData.mcp_tools || {};
			const pluginTools = toolsData.plugin_tools || {};

			// Fetch agents from markdown files
			const agentsRes = await fetch("/api/config/agents");
			const mdAgents = await agentsRes.json();
			const mdAgentNames = mdAgents.map((a) => a.name);

			// Fetch current opencode config for agent permissions
			const configRes = await fetch("/api/config/opencode");
			const config = await configRes.json();
			const jsonAgents = config.agent || {};

			// Build lookup maps from markdown agents (now includes mode + permission)
			const mdAgentMap = {};
			for (const a of mdAgents) {
				mdAgentMap[a.name] = a;
			}

			// Merge: agents from markdown + agents from opencode.json
			const allAgentNames = [
				...new Set([...mdAgentNames, ...Object.keys(jsonAgents)]),
			].sort();

			if (allAgentNames.length === 0) {
				container.innerHTML =
					'<p class="empty-state">No agents found. Create agents in markdown files or opencode.json.</p>';
				loading.style.display = "none";
				return;
			}

			const grid = document.createElement("div");
			grid.className = "permissions-grid";

			// Build a set of MCP tool names for quick lookup
			const mcpToolSet = new Set();
			for (const [, tools] of Object.entries(mcpTools)) {
				tools.forEach((t) => mcpToolSet.add(t));
			}

			// Build a set of plugin tool names for quick lookup
			const pluginToolSet = new Set();
			for (const [, tools] of Object.entries(pluginTools)) {
				if (Array.isArray(tools)) tools.forEach((t) => pluginToolSet.add(t));
			}

			for (const name of allAgentNames) {
				const jsonAgent = jsonAgents[name] || {};
				const mdAgent = mdAgentMap[name] || {};
				// Prefer opencode.json permissions, fall back to markdown frontmatter
				const permission = jsonAgent.permission || mdAgent.permission || {};
				const mode = jsonAgent.mode || mdAgent.mode || "all";
				const isMdAgent = mdAgentNames.includes(name);

				const card = document.createElement("div");
				card.className = "permission-card";
				card.dataset.agent = name;

				let rowsHtml = "";
				let currentCategory = "";
				for (const key of allTools) {
					const val = permission[key] || "deny";
					const isMcp = mcpToolSet.has(key);
					const isPlugin = pluginToolSet.has(key);
					let badge = "";
					if (isMcp) badge = '<span class="mcp-badge">MCP</span>';
					else if (isPlugin) badge = '<span class="plugin-badge">Plugin</span>';

					// Determine category
					let category = "Built-in";
					if (isMcp) category = "MCP Tools";
					else if (isPlugin) category = "Plugin Tools";

					if (category !== currentCategory) {
						currentCategory = category;
						rowsHtml += `
										<div class="tool-category">
											<span class="tool-category-label">${category}</span>
										</div>
									`;
					}

					rowsHtml += `
									<div class="permission-row" data-tool="${key}">
										<span class="permission-key">${key}${badge}</span>
										<select class="permission-select ${val}" data-key="${key}" data-agent="${name}">
											<option value="allow" ${val === "allow" ? "selected" : ""}>allow</option>
											<option value="ask" ${val === "ask" ? "selected" : ""}>ask</option>
											<option value="deny" ${val === "deny" ? "selected" : ""}>deny</option>
										</select>
									</div>
								`;
				}

				const sourceBadge = isMdAgent
					? '<span class="skill-badge">MD</span>'
					: "";

				card.innerHTML = `
								<div class="permission-card-header">
									<div class="permission-card-header-left">
										<div class="permission-card-header-top">
											<span class="permission-card-name">${name}</span>
											<span class="permission-card-mode">${mode}</span>
											${sourceBadge}
										</div>
									</div>
									<div class="permission-actions">
										<div class="bulk-actions">
											<button class="btn btn-secondary btn-sm" onclick="window.kanbanApp.bulkSetPermission('${name}', 'allow')" title="Allow all tools">✓ All</button>
											<button class="btn btn-secondary btn-sm" onclick="window.kanbanApp.bulkSetPermission('${name}', 'deny')" title="Deny all tools">✗ All</button>
										</div>
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
			container.querySelectorAll(".permission-select").forEach((sel) => {
				sel.addEventListener("change", () => {
					sel.className = `permission-select ${sel.value}`;
				});
			});
		} catch (e) {
			console.error("Failed to load permissions:", e);
			container.innerHTML =
				'<p class="empty-state">Failed to load permissions.</p>';
		} finally {
			loading.style.display = "none";
		}
	}

	function addCustomTool(agentName) {
		const name = prompt("Tool name (e.g. my_custom_tool or MCP tool):");
		if (!name || !name.trim()) return;
		const key = name.trim();

		const card = document.querySelector(
			`.permission-card[data-agent="${agentName}"]`,
		);
		if (!card) return;
		const rowsContainer = card.querySelector(".permission-rows");
		if (!rowsContainer) return;

		// Check if already exists
		if (rowsContainer.querySelector(`[data-key="${key}"]`)) {
			showToast("Tool already exists", "error");
			return;
		}

		const row = document.createElement("div");
		row.className = "permission-row";
		row.setAttribute("data-tool", key);
		row.innerHTML = `
			<span class="permission-key">${key}</span>
			<select class="permission-select deny" data-key="${key}" data-agent="${agentName}">
				<option value="allow">allow</option>
				<option value="ask">ask</option>
				<option value="deny" selected>deny</option>
			</select>
		`;
		rowsContainer.appendChild(row);
		row
			.querySelector(".permission-select")
			.addEventListener("change", function () {
				this.className = `permission-select ${this.value}`;
			});
	}

	function bulkSetPermission(agentName, value) {
		const selects = document.querySelectorAll(
			`.permission-select[data-agent="${agentName}"]`,
		);
		selects.forEach((sel) => {
			sel.value = value;
			sel.className = `permission-select ${value}`;
		});
		showToast(`All tools set to ${value}`, "success");
	}

	function filterPermissions(query) {
		const normalizedQuery = query.toLowerCase().trim();
		const rows = document.querySelectorAll(".permission-row");
		const categories = document.querySelectorAll(".tool-category");

		if (!normalizedQuery) {
			rows.forEach((row) => (row.style.display = ""));
			categories.forEach((cat) => (cat.style.display = ""));
			return;
		}

		rows.forEach((row) => {
			const toolName = row.getAttribute("data-tool") || "";
			if (toolName.toLowerCase().includes(normalizedQuery)) {
				row.style.display = "";
			} else {
				row.style.display = "none";
			}
		});

		// Hide category headers if all their tools are hidden
		categories.forEach((cat) => {
			const nextRows = [];
			let sibling = cat.nextElementSibling;
			while (sibling && !sibling.classList.contains("tool-category")) {
				if (sibling.classList.contains("permission-row")) {
					nextRows.push(sibling);
				}
				sibling = sibling.nextElementSibling;
			}
			const hasVisible = nextRows.some((r) => r.style.display !== "none");
			cat.style.display = hasVisible ? "" : "none";
		});
	}

	async function savePermissions(agentName) {
		const selects = document.querySelectorAll(
			`.permission-select[data-agent="${agentName}"]`,
		);
		const permission = {};
		selects.forEach((sel) => {
			permission[sel.dataset.key] = sel.value;
		});

		try {
			const res = await fetch(`/api/config/agents/${agentName}/permissions`, {
				method: "PUT",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify(permission),
			});

			if (res.ok) {
				showToast(`Permissions for "${agentName}" saved`, "success");
			} else {
				const err = await res.json();
				showToast(err.error || "Failed to save permissions", "error");
			}
		} catch (e) {
			console.error("Failed to save permissions:", e);
			showToast("Failed to save permissions", "error");
		}
	}

	// --- Skills ---
	async function loadSkills() {
		try {
			const res = await fetch("/api/config/skills");
			configData.skills = await res.json();
			renderSkills();
		} catch (e) {
			console.error("Failed to load skills:", e);
		}
	}

	function renderSkills() {
		const list = document.getElementById("skills-list");
		list.innerHTML = configData.skills
			.map(
				(skill) => `
            <div class="skill-item">
                <div class="skill-header-row" style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 8px;">
                    <div class="skill-name" style="margin-bottom: 0;">${skill.name}</div>
                    <button class="agent-delete-btn" onclick="window.kanbanApp.deleteSkill('${skill.name}')" style="padding: 2px 6px;">🗑️</button>
                </div>
                <div class="skill-desc">${skill.description || "No description"}</div>
                ${skill.hasSkillMD ? '<span class="skill-badge">SKILL.md</span>' : ""}
            </div>
        `,
			)
			.join("");
	}

	async function deleteSkill(name) {
		if (!confirm(`Are you sure you want to delete the skill "${name}"?`)) {
			return;
		}
		try {
			const res = await fetch(`/api/config/skills/${name}`, {
				method: "DELETE",
			});
			if (res.ok) {
				showToast(`Skill "${name}" deleted`);
				loadSkills();
			} else {
				const err = await res.json();
				showToast(err.error || "Failed to delete skill", "error");
			}
		} catch (e) {
			console.error("Failed to delete skill:", e);
			showToast("Failed to delete skill", "error");
		}
	}

	// --- JSON Editor ---
	async function loadJson() {
		try {
			const res = await fetch("/api/config/opencode");
			const data = await res.json();
			configData.json = data;
			document.getElementById("json-editor").value = JSON.stringify(
				data,
				null,
				2,
			);
		} catch (e) {
			console.error("Failed to load JSON:", e);
		}
	}

	async function saveJson() {
		const content = document.getElementById("json-editor").value;
		try {
			// Validate JSON
			JSON.parse(content);

			const res = await fetch("/api/config/opencode", {
				method: "PUT",
				headers: { "Content-Type": "application/json" },
				body: content,
			});

			if (res.ok) {
				showToast("opencode.json saved. Restart opencode to apply.");
			} else {
				const err = await res.json();
				showToast(err.error || "Failed to save", "error");
			}
		} catch (e) {
			if (e instanceof SyntaxError) {
				showToast("Invalid JSON", "error");
			} else {
				console.error("Failed to save JSON:", e);
				showToast("Failed to save", "error");
			}
		}
	}

	function formatSize(bytes) {
		if (bytes < 1024) return bytes + " B";
		if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
		return (bytes / (1024 * 1024)).toFixed(1) + " MB";
	}

	function showToast(message, type = "success") {
		const toast = document.createElement("div");
		toast.className = `toast toast-${type}`;
		toast.textContent = message;
		toast.style.cssText = `
            position: fixed;
            bottom: 20px;
            right: 20px;
            background: ${type === "error" ? "var(--danger)" : "var(--accent)"};
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
	window.kanbanApp = {
		editAgent,
		deleteAgent,
		deleteSkill,
		toggleMCP,
		addProvider,
		editProvider,
		saveProvider,
		cancelProviderEdit,
		deleteProvider,
		toggleApiKeyVisibility,
		validateModelsJson,
		formatModelsJson,
		clearModelsJson,
		savePermissions,
		addCustomTool,
		bulkSetPermission,
		filterPermissions,
	};

	// Setup permissions search
	function setupPermissionsSearch() {
		const searchInput = document.getElementById("permissions-search-input");
		if (searchInput) {
			searchInput.addEventListener("input", (e) => {
				filterPermissions(e.target.value);
			});
		}
	}

	document.addEventListener("DOMContentLoaded", () => {
		init();
		setupPermissionsSearch();
	});
})();

// Close provider modal on Escape key
document.addEventListener("keydown", (e) => {
	if (e.key === "Escape") {
		const overlay = document.getElementById("provider-modal-overlay");
		if (overlay && overlay.style.display !== "none") {
			window.kanbanApp.cancelProviderEdit();
		}
	}
});

// --- Activity Stream & Decision Feed Styles ---
(() => {
	const style = document.createElement("style");
	style.textContent = `
/* ═══════════════════════════════════════════
   CSS Custom Properties (extends app.css :root)
   ═══════════════════════════════════════════ */
:root {
	--card-radius: 10px;
	--timeline-dot-size: 8px;
	--shadow-sm: 0 1px 3px rgba(0,0,0,0.12);
	--shadow-md: 0 4px 12px rgba(0,0,0,0.2);
	--shadow-lg: 0 8px 30px rgba(0,0,0,0.3);
	--glass-bg: rgba(14, 17, 30, 0.75);
}

/* ═══════════════════════════════════════════
   Card Agent Header
   ═══════════════════════════════════════════ */
.card-agent {
	display: flex;
	align-items: center;
	gap: 8px;
	margin-bottom: 6px;
}

.agent-circle {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 26px;
	height: 26px;
	border-radius: 50%;
	background: color-mix(in srgb, var(--agent-color, #888) 20%, transparent);
	font-size: 14px;
	flex-shrink: 0;
}

.agent-label {
	font-size: 0.7rem;
	font-weight: 700;
	color: var(--text-secondary);
	text-transform: uppercase;
	letter-spacing: 0.04em;
}

.card-id {
	font-size: 0.6rem;
	color: var(--text-muted);
	margin-left: auto;
	font-family: "SF Mono", "Fira Code", monospace;
}

/* ═══════════════════════════════════════════
   Status Pills & Row
   ═══════════════════════════════════════════ */
.card-status-row {
	display: flex;
	align-items: center;
	gap: 8px;
	margin: 6px 0 8px;
}

.status-pill {
	display: inline-flex;
	align-items: center;
	padding: 2px 10px;
	border-radius: 20px;
	font-size: 0.6rem;
	font-weight: 700;
	text-transform: uppercase;
	letter-spacing: 0.03em;
}
.status-pill.status-running,
.status-pill.status-in_progress {
	background: rgba(94, 96, 240, 0.15);
	color: #7b7dfb;
}
.status-pill.status-pending {
	background: rgba(255, 255, 255, 0.06);
	color: var(--text-muted);
}
.status-pill.status-blocked {
	background: rgba(255, 71, 87, 0.15);
	color: var(--danger);
	animation: pulse-red 2s infinite;
}
.status-pill.status-review,
.status-pill.status-changes {
	background: rgba(255, 165, 2, 0.15);
	color: #ffa502;
}
.status-pill.status-done {
	background: rgba(46, 213, 115, 0.15);
	color: var(--success);
}

.status-elapsed {
	font-size: 0.6rem;
	color: var(--text-muted);
	font-style: italic;
}

/* ═══════════════════════════════════════════
   Activity Toggle Button
   ═══════════════════════════════════════════ */
.card-activity-toggle {
	display: block;
	width: 100%;
	font-size: 0.65rem;
	font-weight: 600;
	color: var(--text-secondary);
	cursor: pointer;
	padding: 6px 10px;
	margin-top: 8px;
	border: 1px dashed var(--border);
	border-radius: 6px;
	background: transparent;
	text-align: center;
	transition: all 0.2s ease;
	user-select: none;
	font-family: inherit;
}
.card-activity-toggle:hover {
	background: var(--bg-secondary);
	color: var(--accent);
	border-color: var(--accent);
}

/* ═══════════════════════════════════════════
   Activity Stream — Timeline Style
   ═══════════════════════════════════════════ */
.card-activity {
	max-height: 240px;
	overflow: hidden;
	margin-top: 8px;
	padding: 8px 4px 8px 20px;
	background: var(--bg-secondary);
	border-radius: 6px;
	border: 1px solid var(--border);
	position: relative;
	transition: max-height 0.35s ease;
}
.card-activity::before {
	content: "";
	position: absolute;
	left: 11px;
	top: 12px;
	bottom: 12px;
	width: 1px;
	background: var(--border);
}

.activity-event {
	position: relative;
	padding: 5px 0 5px 6px;
	font-size: 0.65rem;
	border-bottom: none;
}
.activity-event:nth-child(even) {
	background: rgba(255,255,255,0.015);
}
.activity-event::before {
	content: "";
	position: absolute;
	left: -13px;
	top: 9px;
	width: var(--timeline-dot-size);
	height: var(--timeline-dot-size);
	border-radius: 50%;
	background: var(--accent);
	border: 2px solid var(--bg-secondary);
	z-index: 1;
}
.activity-event.type-decision::before { background: #ffa502; }
.activity-event.type-question::before { background: #9b59b6; }
.activity-event.type-blocked::before { background: var(--danger); }
.activity-event.type-progress::before { background: var(--accent); }

.activity-event.resolved {
	opacity: 0.5;
}
.activity-event.resolved .activity-content {
	text-decoration: line-through;
}
.activity-event:last-child {
	border-bottom: none;
}

.activity-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	margin-bottom: 2px;
}
.activity-type {
	font-size: 0.55rem;
	font-weight: 700;
	text-transform: uppercase;
	padding: 1px 6px;
	border-radius: 3px;
	letter-spacing: 0.04em;
}
.activity-type.type-decision { background: rgba(255,165,2,0.15); color: #ffa502; }
.activity-type.type-question { background: rgba(155,89,182,0.15); color: #b07cd8; }
.activity-type.type-blocked { background: rgba(255,71,87,0.15); color: var(--danger); }
.activity-type.type-progress { background: rgba(94,96,240,0.15); color: #7b7dfb; }

.activity-time {
	font-size: 0.55rem;
	color: var(--text-muted);
}
.activity-content {
	color: var(--text-primary);
	line-height: 1.35;
	font-size: 0.65rem;
}
.activity-resolution {
	font-size: 0.55rem;
	color: var(--accent);
	margin-top: 2px;
	font-style: italic;
}

/* Show-all button at bottom of activity */
.activity-show-all {
	display: block;
	width: 100%;
	padding: 5px 0;
	font-size: 0.6rem;
	font-weight: 600;
	color: var(--accent);
	text-align: center;
	cursor: pointer;
	background: none;
	border: none;
	border-top: 1px solid var(--border);
	margin-top: 4px;
	transition: color 0.2s;
	font-family: inherit;
}
.activity-show-all:hover {
	color: #7b7dfb;
}

/* Hidden events (beyond 3) */
.activity-event.hidden-event {
	display: none;
}
.activity-event.hidden-event.visible {
	display: block;
}

/* ═══════════════════════════════════════════
   Pending Action Bar — Prominent CTA
   ═══════════════════════════════════════════ */
.card-action-bar {
	display: flex;
	flex-direction: column;
	gap: 8px;
	padding: 10px 12px;
	margin-top: 10px;
	background: linear-gradient(135deg, rgba(255,243,205,0.12), rgba(255,243,205,0.06));
	border: 1px solid rgba(255,193,7,0.35);
	border-left: 4px solid #ffc107;
	border-radius: 8px;
	font-size: 0.7rem;
	animation: actionSlideIn 0.35s ease-out;
}

@keyframes actionSlideIn {
	from {
		opacity: 0;
		transform: translateY(-6px);
		max-height: 0;
	}
	to {
		opacity: 1;
		transform: translateY(0);
		max-height: 120px;
	}
}

.action-badge {
	display: inline-flex;
	align-items: center;
	gap: 5px;
	font-weight: 700;
	color: #b8860b;
	font-size: 0.65rem;
	text-transform: uppercase;
	letter-spacing: 0.05em;
}
.action-badge::before {
	content: "●";
	color: #ffc107;
	animation: pulse-badge 1.8s ease-in-out infinite;
}
@keyframes pulse-badge {
	0%, 100% { opacity: 1; transform: scale(1); }
	50% { opacity: 0.4; transform: scale(0.85); }
}

.action-latest-msg {
	color: rgba(241, 243, 249, 0.7);
	font-size: 0.65rem;
	background: rgba(0,0,0,0.15);
	padding: 5px 8px;
	border-radius: 4px;
	max-height: 44px;
	overflow: hidden;
	text-overflow: ellipsis;
	line-height: 1.35;
}

.action-buttons {
	display: flex;
	gap: 6px;
	flex-wrap: wrap;
}

.action-approve,
.action-reject,
.action-modify {
	font-size: 0.65rem;
	padding: 5px 12px;
	border: none;
	border-radius: 5px;
	cursor: pointer;
	font-weight: 700;
	transition: all 0.2s ease;
	font-family: inherit;
}
.action-approve:hover,
.action-reject:hover,
.action-modify:hover {
	transform: translateY(-2px);
	box-shadow: 0 4px 8px rgba(0,0,0,0.25);
}

.action-approve {
	background: linear-gradient(135deg, #28a745, #20c997);
	color: #fff;
}
.action-reject {
	background: linear-gradient(135deg, #dc3545, #e74c3c);
	color: #fff;
}
.action-modify {
	background: linear-gradient(135deg, #6c757d, #5a6268);
	color: #fff;
}

/* ═══════════════════════════════════════════
   Decision Feed Sidebar — Glass Morphism
   ═══════════════════════════════════════════ */
#decision-feed {
	position: fixed;
	right: 20px;
	top: 80px;
	width: 290px;
	max-height: calc(100vh - 120px);
	background: var(--glass-bg);
	backdrop-filter: blur(16px);
	-webkit-backdrop-filter: blur(16px);
	border: 1px solid rgba(255,255,255,0.08);
	border-radius: 12px;
	box-shadow: var(--shadow-lg);
	z-index: 50;
	overflow: hidden;
	display: flex;
	flex-direction: column;
	transition: opacity 0.3s ease, transform 0.3s ease;
}

.feed-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 14px 16px;
	background: rgba(255,255,255,0.03);
	border-bottom: 1px solid rgba(255,255,255,0.06);
}
.feed-header h3 {
	font-size: 0.85rem;
	font-weight: 700;
	color: var(--text-primary);
	margin: 0;
}
.feed-badge {
	background: var(--accent);
	color: #fff;
	font-size: 0.65rem;
	font-weight: 700;
	padding: 3px 8px;
	border-radius: 12px;
	min-width: 22px;
	text-align: center;
	transition: transform 0.2s ease;
}
.feed-badge.bump {
	animation: badgeBump 0.35s ease-out;
}
@keyframes badgeBump {
	0% { transform: scale(1); }
	40% { transform: scale(1.35); }
	100% { transform: scale(1); }
}

.feed-list {
	flex: 1;
	overflow-y: auto;
	padding: 8px 10px;
}

.feed-item {
	padding: 10px;
	margin-bottom: 6px;
	background: rgba(255,255,255,0.03);
	border-radius: 8px;
	border: 1px solid rgba(255,255,255,0.05);
	font-size: 0.7rem;
	cursor: pointer;
	transition: all 0.2s ease;
	border-left: 3px solid transparent;
}
.feed-item:hover {
	background: rgba(255,255,255,0.06);
	border-color: rgba(255,255,255,0.1);
	transform: translateX(2px);
}
.feed-item.type-decision { border-left-color: #5e60f0; }
.feed-item.type-question { border-left-color: #9b59b6; }
.feed-item.type-blocked { border-left-color: var(--danger); }

.feed-item:last-child {
	margin-bottom: 0;
}

.feed-item-agent {
	font-size: 0.6rem;
	font-weight: 700;
	text-transform: uppercase;
	letter-spacing: 0.04em;
	color: var(--accent);
	margin-bottom: 3px;
}
.feed-item-content {
	color: var(--text-primary);
	line-height: 1.35;
	margin-bottom: 4px;
}
.feed-item-time {
	font-size: 0.55rem;
	color: var(--text-muted);
}

/* Empty state for feed */
.feed-empty {
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	padding: 30px 16px;
	color: var(--text-muted);
	font-size: 0.75rem;
	text-align: center;
	gap: 8px;
	opacity: 0.7;
}
.feed-empty-icon {
	font-size: 2rem;
	margin-bottom: 4px;
}

/* ═══════════════════════════════════════════
   Card Gradients & Agent-specific styling
   ═══════════════════════════════════════════ */
.card.card-agent-dev { background: linear-gradient(160deg, rgba(46,213,115,0.06), transparent), var(--bg-card); }
.card.card-agent-qa { background: linear-gradient(160deg, rgba(255,165,2,0.06), transparent), var(--bg-card); }
.card.card-agent-architect { background: linear-gradient(160deg, rgba(30,144,255,0.06), transparent), var(--bg-card); }
.card.card-agent-reviewer { background: linear-gradient(160deg, rgba(155,89,182,0.06), transparent), var(--bg-card); }
.card.card-agent-devops { background: linear-gradient(160deg, rgba(255,71,87,0.06), transparent), var(--bg-card); }

.card.card-agent-dev:hover { background: linear-gradient(160deg, rgba(46,213,115,0.1), transparent), var(--bg-card-hover); }
.card.card-agent-qa:hover { background: linear-gradient(160deg, rgba(255,165,2,0.1), transparent), var(--bg-card-hover); }
.card.card-agent-architect:hover { background: linear-gradient(160deg, rgba(30,144,255,0.1), transparent), var(--bg-card-hover); }
.card.card-agent-reviewer:hover { background: linear-gradient(160deg, rgba(155,89,182,0.1), transparent), var(--bg-card-hover); }
.card.card-agent-devops:hover { background: linear-gradient(160deg, rgba(255,71,87,0.1), transparent), var(--bg-card-hover); }

/* ═══════════════════════════════════════════
   Animations
   ═══════════════════════════════════════════ */
@keyframes pulse {
	0%, 100% { opacity: 1; }
	50% { opacity: 0.65; }
}
@keyframes pulse-red {
	0% { box-shadow: 0 0 0 0 rgba(255,71,87,0.2); }
	70% { box-shadow: 0 0 0 5px rgba(255,71,87,0); }
	100% { box-shadow: 0 0 0 0 rgba(255,71,87,0); }
}
@keyframes fadeInUp {
	from { opacity: 0; transform: translateY(8px); }
	to { opacity: 1; transform: translateY(0); }
}
.card {
	animation: fadeInUp 0.3s ease-out;
}

/* ═══════════════════════════════════════════
   Card Hover Enhancement (extends app.css)
   ═══════════════════════════════════════════ */
.card:hover {
	transform: translateY(-2px);
	box-shadow: var(--shadow-md);
}

/* ═══════════════════════════════════════════
   Scrollbar styling for activity stream & feed
   ═══════════════════════════════════════════ */
.card-activity::-webkit-scrollbar,
.feed-list::-webkit-scrollbar {
	width: 4px;
}
.card-activity::-webkit-scrollbar-thumb,
.feed-list::-webkit-scrollbar-thumb {
	background: rgba(255,255,255,0.1);
	border-radius: 4px;
}
`;
	document.head.appendChild(style);
})();
