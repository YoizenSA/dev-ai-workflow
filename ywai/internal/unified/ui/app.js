// ywai Unified Dashboard - SPA
(() => {
  // State
  let currentView = 'kanban';
  let kanbanWS = null;
  let missionsWS = null;
  
  // API base
  const apiBase = () => `${window.location.protocol}//${window.location.host}`;
  
  // Router
  function navigate(view) {
    currentView = view;
    updateNav();
    renderView();
  }
  
  function updateNav() {
    document.querySelectorAll('.nav-item').forEach(item => {
      item.classList.remove('active');
      if (item.dataset.view === currentView) {
        item.classList.add('active');
      }
    });
  }
  
  function renderView() {
    const content = document.getElementById('main-content');
    
    if (currentView === 'kanban') {
      renderKanban(content);
    } else if (currentView === 'missions') {
      renderMissions(content);
    }
  }
  
  // Kanban View
  function renderKanban(container) {
    container.innerHTML = `
      <div class="page-header">
        <div class="page-heading">
          <span class="page-eyebrow">Delegations</span>
          <h1 class="page-title">Kanban Board</h1>
          <p class="page-subtitle">Track and manage AI agent delegations</p>
        </div>
        <div class="page-actions">
          <button class="btn btn-primary" onclick="window.unifiedApp.createDelegation()">+ New Delegation</button>
        </div>
      </div>
      
      <div class="board">
        <div class="column" data-column="backlog">
          <div class="column-header">
            <h2>Backlog</h2>
            <span class="column-count" id="count-backlog">0</span>
          </div>
          <div class="column-cards" id="col-backlog"></div>
        </div>
        <div class="column" data-column="ready">
          <div class="column-header">
            <h2>Ready</h2>
            <span class="column-count" id="count-ready">0</span>
          </div>
          <div class="column-cards" id="col-ready"></div>
        </div>
        <div class="column" data-column="in_progress">
          <div class="column-header">
            <h2>In Progress</h2>
            <span class="column-count" id="count-in_progress">0</span>
          </div>
          <div class="column-cards" id="col-in_progress"></div>
        </div>
        <div class="column" data-column="review">
          <div class="column-header">
            <h2>Review</h2>
            <span class="column-count" id="count-review">0</span>
          </div>
          <div class="column-cards" id="col-review"></div>
        </div>
        <div class="column" data-column="done">
          <div class="column-header">
            <h2>Done</h2>
            <span class="column-count" id="count-done">0</span>
          </div>
          <div class="column-cards" id="col-done"></div>
        </div>
      </div>
    `;
    
    connectKanbanWS();
  }
  
  // Missions View
  function renderMissions(container) {
    container.innerHTML = `
      <div class="page-header">
        <div class="page-heading">
          <span class="page-eyebrow">Missions</span>
          <h1 class="page-title">Mission Control</h1>
          <p class="page-subtitle">Manage projects and AI missions</p>
        </div>
        <div class="page-actions">
          <button class="btn btn-primary" onclick="window.unifiedApp.createMission()">+ New Mission</button>
        </div>
      </div>
      
      <div class="grid-2">
        <div class="card">
          <div class="card-header">
            <h3>Projects</h3>
          </div>
          <div id="projects-list" class="card-body">
            <p class="muted">Loading projects...</p>
          </div>
        </div>
        <div class="card">
          <div class="card-header">
            <h3>Active Missions</h3>
          </div>
          <div id="missions-list" class="card-body">
            <p class="muted">Loading missions...</p>
          </div>
        </div>
      </div>
    `;
    
    connectMissionsWS();
  }
  
  // WebSocket Connections
  function connectKanbanWS() {
    if (kanbanWS) {
      kanbanWS.close();
    }
    
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    kanbanWS = new WebSocket(`${protocol}//${window.location.host}/ws`);
    
    kanbanWS.onopen = () => {
      console.log('Kanban WebSocket connected');
    };
    
    kanbanWS.onmessage = (event) => {
      const data = JSON.parse(event.data);
      handleKanbanUpdate(data);
    };
    
    kanbanWS.onerror = (error) => {
      console.error('Kanban WebSocket error:', error);
    };
  }
  
  function connectMissionsWS() {
    if (missionsWS) {
      missionsWS.close();
    }
    
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    missionsWS = new WebSocket(`${protocol}//${window.location.host}/missions/ws`);
    
    missionsWS.onopen = () => {
      console.log('Missions WebSocket connected');
    };
    
    missionsWS.onmessage = (event) => {
      const data = JSON.parse(event.data);
      handleMissionsUpdate(data);
    };
    
    missionsWS.onerror = (error) => {
      console.error('Missions WebSocket error:', error);
    };
  }
  
  function handleKanbanUpdate(data) {
    // Handle kanban updates
    console.log('Kanban update:', data);
  }
  
  function handleMissionsUpdate(data) {
    // Handle missions updates
    console.log('Missions update:', data);
  }
  
  // Public API
  window.unifiedApp = {
    navigate,
    createDelegation: () => {
      alert('Create delegation - TODO');
    },
    createMission: () => {
      alert('Create mission - TODO');
    }
  };
  
  // Initialize
  document.addEventListener('DOMContentLoaded', () => {
    // Setup nav click handlers
    document.querySelectorAll('.nav-item').forEach(item => {
      item.addEventListener('click', (e) => {
        e.preventDefault();
        navigate(item.dataset.view);
      });
    });
    
    // Initial render
    renderView();
  });
})();
