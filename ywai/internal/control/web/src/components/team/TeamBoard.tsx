import { useState, useEffect, useCallback } from "react";

interface TeamMember {
  id: string;
  name: string;
  status: string;
  started_at: string;
  task_id?: string;
}

interface TeamTask {
  id: string;
  title: string;
  status: string;
  assignee?: string;
  result?: string;
  priority: string;
}

const API_BASE = "";

export default function TeamBoard() {
  const [members, setMembers] = useState<TeamMember[]>([]);
  const [tasks, setTasks] = useState<TeamTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [spawnProfile, setSpawnProfile] = useState("dev");
  const [spawnPrompt, setSpawnPrompt] = useState("");
  const [steerTarget, setSteerTarget] = useState("");
  const [steerMessage, setSteerMessage] = useState("");

  const fetchStatus = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/api/team/status`);
      if (!res.ok) throw new Error(res.statusText);
      const data = await res.json();
      setMembers(data.members || []);
      setTasks(data.tasks || []);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to fetch status");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, 3000);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  const handleSpawn = async () => {
    if (!spawnPrompt.trim()) return;
    try {
      const res = await fetch(`${API_BASE}/api/team/spawn`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          profile: spawnProfile,
          prompt: spawnPrompt,
          task: spawnPrompt.substring(0, 80),
        }),
      });
      if (!res.ok) throw new Error(res.statusText);
      setSpawnPrompt("");
      fetchStatus();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to spawn");
    }
  };

  const handleSteer = async () => {
    if (!steerTarget || !steerMessage.trim()) return;
    try {
      const res = await fetch(`${API_BASE}/api/team/steer`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ member_id: steerTarget, message: steerMessage }),
      });
      if (!res.ok) throw new Error(res.statusText);
      setSteerMessage("");
      fetchStatus();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to steer");
    }
  };

  const handleShutdown = async (memberId: string) => {
    try {
      await fetch(`${API_BASE}/api/team/shutdown`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ member_id: memberId }),
      });
      fetchStatus();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to shutdown");
    }
  };

  const getTaskCount = (status: string) =>
    tasks.filter((t) => t.status === status).length;

  if (loading) {
    return <div className="team-loading">Loading team status...</div>;
  }

  return (
    <div className="team-board">
      <div className="team-header">
        <h2>Team Mode</h2>
        <span className="team-stats">
          {members.length} members · {tasks.length} tasks
        </span>
      </div>

      {error && (
        <div className="team-error" onClick={() => setError(null)}>
          ⚠️ {error} (click to dismiss)
        </div>
      )}

      {/* Spawn controls */}
      <div className="team-controls">
        <div className="control-group">
          <select
            value={spawnProfile}
            onChange={(e) => setSpawnProfile(e.target.value)}
            className="team-select"
          >
            <option value="orchestrator">orchestrator</option>
            <option value="dev">dev</option>
            <option value="qa">qa</option>
            <option value="architect">architect</option>
            <option value="reviewer">reviewer</option>
            <option value="devops">devops</option>
            <option value="finder">finder</option>
            <option value="ask">ask</option>
          </select>
          <input
            type="text"
            value={spawnPrompt}
            onChange={(e) => setSpawnPrompt(e.target.value)}
            placeholder="Task prompt for teammate..."
            className="team-input"
            onKeyDown={(e) => e.key === "Enter" && handleSpawn()}
          />
          <button onClick={handleSpawn} className="team-btn primary" disabled={!spawnPrompt.trim()}>
            Spawn
          </button>
        </div>
      </div>

      {/* Kanban columns */}
      <div className="team-columns">
        {/* Members column */}
        <div className="team-column">
          <h3>Members ({members.length})</h3>
          <div className="team-cards">
            {members.map((m) => (
              <div key={m.id} className={`team-card status-${m.status}`}>
                <div className="card-header">
                  <span className={`status-dot ${m.status}`} />
                  <strong>{m.name}</strong>
                  <span className="member-id">{m.id}</span>
                </div>
                <div className="card-body">
                  <div className="card-field">
                    <label>Status</label>
                    <span>{m.status}</span>
                  </div>
                  <div className="card-field">
                    <label>Started</label>
                    <span>{new Date(m.started_at).toLocaleTimeString()}</span>
                  </div>
                  {m.task_id && (
                    <div className="card-field">
                      <label>Task</label>
                      <span>{m.task_id}</span>
                    </div>
                  )}
                </div>
                <div className="card-actions">
                  {m.status === "running" && (
                    <>
                      <button
                        onClick={() => setSteerTarget(m.id)}
                        className="team-btn small"
                      >
                        Steer
                      </button>
                      <button
                        onClick={() => handleShutdown(m.id)}
                        className="team-btn small danger"
                      >
                        Stop
                      </button>
                    </>
                  )}
                </div>
              </div>
            ))}
            {members.length === 0 && (
              <div className="team-empty">No active teammates</div>
            )}
          </div>
        </div>

        {/* Steer panel (shown when a member is selected) */}
        {steerTarget && (
          <div className="team-column">
            <h3>Steer: {steerTarget}</h3>
            <div className="steer-panel">
              <textarea
                value={steerMessage}
                onChange={(e) => setSteerMessage(e.target.value)}
                placeholder="Message to teammate..."
                rows={3}
                className="team-textarea"
              />
              <div className="steer-actions">
                <button onClick={handleSteer} className="team-btn primary" disabled={!steerMessage.trim()}>
                  Send
                </button>
                <button onClick={() => { setSteerTarget(""); setSteerMessage(""); }} className="team-btn">
                  Cancel
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Tasks column */}
        <div className="team-column">
          <h3>
            Tasks ({tasks.length})
            <span className="task-counts">
              {getTaskCount("running") > 0 && <> 🔄{getTaskCount("running")}</>}
              {getTaskCount("done") > 0 && <> ✅{getTaskCount("done")}</>}
              {getTaskCount("failed") > 0 && <> ❌{getTaskCount("failed")}</>}
            </span>
          </h3>
          <div className="team-cards">
            {tasks.map((t) => (
              <div key={t.id} className={`team-card task-card status-${t.status}`}>
                <div className="card-header">
                  <span className={`status-dot ${t.status}`} />
                  <strong>{t.title.substring(0, 60)}</strong>
                </div>
                <div className="card-body">
                  <div className="card-field">
                    <label>ID</label>
                    <span>{t.id}</span>
                  </div>
                  <div className="card-field">
                    <label>Status</label>
                    <span className={`badge badge-${t.status}`}>{t.status}</span>
                  </div>
                  <div className="card-field">
                    <label>Priority</label>
                    <span className={`badge badge-${t.priority}`}>{t.priority}</span>
                  </div>
                  {t.assignee && (
                    <div className="card-field">
                      <label>Assignee</label>
                      <span>{t.assignee}</span>
                    </div>
                  )}
                  {t.result && (
                    <div className="card-field result">
                      <label>Result</label>
                      <pre>{t.result.substring(0, 200)}</pre>
                    </div>
                  )}
                </div>
              </div>
            ))}
            {tasks.length === 0 && (
              <div className="team-empty">No tasks</div>
            )}
          </div>
        </div>
      </div>

      <style>{`
        .team-board { padding: 16px; color: #e0e0e0; background: #0d1117; min-height: 100vh; }
        .team-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
        .team-header h2 { margin: 0; font-size: 20px; color: #c9d1d9; }
        .team-stats { font-size: 13px; color: #8b949e; }
        .team-error { background: #da3633; color: #fff; padding: 8px 12px; border-radius: 6px; margin-bottom: 12px; cursor: pointer; font-size: 13px; }
        .team-controls { margin-bottom: 16px; }
        .control-group { display: flex; gap: 8px; align-items: center; }
        .team-select { padding: 6px 10px; background: #21262d; color: #c9d1d9; border: 1px solid #30363d; border-radius: 6px; font-size: 13px; }
        .team-input { flex: 1; padding: 6px 10px; background: #21262d; color: #c9d1d9; border: 1px solid #30363d; border-radius: 6px; font-size: 13px; }
        .team-textarea { width: 100%; padding: 6px 10px; background: #21262d; color: #c9d1d9; border: 1px solid #30363d; border-radius: 6px; font-size: 13px; resize: vertical; box-sizing: border-box; }
        .team-btn { padding: 6px 12px; background: #21262d; color: #c9d1d9; border: 1px solid #30363d; border-radius: 6px; cursor: pointer; font-size: 13px; }
        .team-btn:hover { background: #30363d; }
        .team-btn.primary { background: #238636; border-color: #238636; color: #fff; }
        .team-btn.primary:hover { background: #2ea043; }
        .team-btn.danger { background: #21262d; border-color: #da3633; color: #da3633; }
        .team-btn.danger:hover { background: #da3633; color: #fff; }
        .team-btn.small { padding: 3px 8px; font-size: 12px; }
        .team-btn:disabled { opacity: 0.5; cursor: not-allowed; }
        .team-columns { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
        .team-column { background: #161b22; border: 1px solid #21262d; border-radius: 8px; padding: 12px; }
        .team-column h3 { margin: 0 0 12px 0; font-size: 15px; color: #c9d1d9; display: flex; justify-content: space-between; }
        .team-cards { display: flex; flex-direction: column; gap: 8px; }
        .team-card { background: #0d1117; border: 1px solid #30363d; border-radius: 6px; padding: 10px; }
        .team-card.status-running { border-left: 3px solid #d29922; }
        .team-card.status-done { border-left: 3px solid #238636; opacity: 0.8; }
        .team-card.status-error, .team-card.status-failed { border-left: 3px solid #da3633; }
        .team-card.status-idle { border-left: 3px solid #8b949e; }
        .card-header { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
        .card-header strong { font-size: 14px; color: #c9d1d9; }
        .member-id { font-size: 11px; color: #8b949e; font-family: monospace; }
        .status-dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
        .status-dot.running { background: #d29922; }
        .status-dot.done { background: #238636; }
        .status-dot.error, .status-dot.failed { background: #da3633; }
        .status-dot.idle, .status-dot.pending { background: #8b949e; }
        .card-body { font-size: 12px; }
        .card-field { display: flex; justify-content: space-between; padding: 2px 0; }
        .card-field label { color: #8b949e; }
        .card-field.result pre { margin: 4px 0 0 0; padding: 6px; background: #161b22; border-radius: 4px; font-size: 11px; color: #c9d1d9; white-space: pre-wrap; word-break: break-all; }
        .card-actions { display: flex; gap: 6px; margin-top: 8px; }
        .badge { padding: 1px 6px; border-radius: 10px; font-size: 11px; }
        .badge-running { background: #d299221a; color: #d29922; }
        .badge-done { background: #2386361a; color: #238636; }
        .badge-failed { background: #da36331a; color: #da3633; }
        .badge-pending { background: #8b949e1a; color: #8b949e; }
        .badge-high { background: #da36331a; color: #da3633; }
        .badge-medium { background: #d299221a; color: #d29922; }
        .badge-low { background: #8b949e1a; color: #8b949e; }
        .team-empty { color: #8b949e; font-size: 13px; text-align: center; padding: 20px; }
        .steer-panel { display: flex; flex-direction: column; gap: 8px; }
        .steer-actions { display: flex; gap: 8px; }
        .task-counts { font-size: 12px; font-weight: normal; }
        .team-loading { color: #8b949e; text-align: center; padding: 40px; }
      `}</style>
    </div>
  );
}
