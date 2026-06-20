import { useState, useEffect, useCallback } from "react";
import { useMissionsStore } from "../../stores/missionsStore";
import { missionsApi, configApi } from "../../api/client";
import type { Project, ModelInfo, PlanMission, FSEntry, GitInfo } from "../../api/types";
import Modal from "../shared/Modal";
import YdSelect from "../shared/YdSelect";
import ModelCombobox from "./ModelCombobox";

interface Props {
	open: boolean;
	onClose: () => void;
}

interface WizardState {
	step: number;

	// Step 1 — Project
	selectedProject: Project | null;
	projects: Project[];

	// File browser
	browseMode: boolean;
	browsePath: string;
	browseEntries: FSEntry[];
	browseLoading: boolean;
	browseError: string | null;
	newFolderName: string;
	creatingFolder: boolean;

	// Step 2 — Branch
	baseBranch: string;
	autoCheckout: boolean;
	createFeatureBranch: boolean;

	// Git state for the selected project (branch actual + branches + isGitRepo)
	gitInfo: GitInfo | null;
	gitLoading: boolean;
	initingGit: boolean;

	// Step 3 — Goal
	goal: string;
	refinedGoal: string;
	refining: boolean;
	chatMessages: { role: "user" | "ai"; text: string }[];
	goalAccepted: boolean;

	// Step 4 — Config
	goalRefineModel: string;
	planModel: string;
	planRefineModel: string;
	workerModel: string;
	maxParallel: number;
	plannerAgent: string;
	workerAgent: string;
	models: ModelInfo[];
	// Model ids/names recommended in dropdowns, derived from configured role
	// defaults (never hardcoded).
	recommendedModels: string[];
	agents: string[];
	modelsError: string | null;
	agentsError: string | null;
	plan: PlanMission | null;
	planMarkdown: string;
	planRefining: boolean;
	planChatMessages: { role: "user" | "ai"; text: string }[];
	planGenerating: boolean;
	submitting: boolean;
	error: string | null;
}

const STEPS = [
	{ num: 1, label: "Project" },
	{ num: 2, label: "Branch & Setup" },
	{ num: 3, label: "Goal" },
	{ num: 4, label: "Review" },
];

function slugify(text: string): string {
	return text
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, "-")
		.replace(/^-|-$/g, "");
}

function planToMarkdown(plan: PlanMission): string {
	let md = `# ${plan.name}\n\n`;
	if (plan.description) md += `${plan.description}\n\n`;
	if (plan.project) md += `**Project:** ${plan.project}\n\n`;
	for (const milestone of plan.milestones) {
		md += `## ${milestone.name}\n\n`;
		if (milestone.description) md += `${milestone.description}\n\n`;
		const features = plan.features.filter((f) => f.milestone === milestone.name);
		for (const f of features) {
			md += `- [ ] ${f.description}`;
			if (f.skillName) md += ` \`${f.skillName}\``;
			if (f.expectedBehavior && f.expectedBehavior.length > 0) md += ` (expectations: ${f.expectedBehavior.length})`;
			md += "\n";
		}
		md += "\n";
	}
	return md.trim();
}

export default function CreateMissionModal({ open, onClose }: Props) {
	const storeProjects = useMissionsStore((s) => s.projects);
	const storeGeneratePlan = useMissionsStore((s) => s.generatePlan);
	const storeApprovePlan = useMissionsStore((s) => s.approvePlan);
	const storeRunMission = useMissionsStore((s) => s.runMission);
	const storeAutoMission = useMissionsStore((s) => s.autoMission);
	const storeSelectMission = useMissionsStore((s) => s.selectMission);

	const [state, setState] = useState<WizardState>({
		step: 1,
		selectedProject: null,
		projects: [],
		browseMode: false,
		browsePath: "",
		browseEntries: [],
		browseLoading: false,
		browseError: null,
		newFolderName: "",
		creatingFolder: false,
		baseBranch: "main",
		autoCheckout: true,
		createFeatureBranch: true,
		gitInfo: null,
		gitLoading: false,
		initingGit: false,
		goal: "",
		refinedGoal: "",
		refining: false,
		chatMessages: [],
		goalAccepted: false,
		goalRefineModel: "",
		planModel: "",
		planRefineModel: "",
		workerModel: "",
		recommendedModels: [],
		maxParallel: 3,
		plannerAgent: "",
		workerAgent: "",
		models: [],
		agents: [],
		modelsError: null,
		agentsError: null,
		plan: null,
		planMarkdown: "",
		planRefining: false,
		planChatMessages: [],
		planGenerating: false,
		submitting: false,
		error: null,
	});

	const update = useCallback((patch: Partial<WizardState>) => {
		setState((prev) => ({ ...prev, ...patch }));
	}, []);

	// Load projects, models, agents and role-defaults on mount.
	// Role defaults from ~/.ywai/config.yaml drive the initial selections so
	// the user does not have to re-pick the same models every mission.
	useEffect(() => {
		if (open) {
			update({ projects: storeProjects, step: 1, error: null, modelsError: null, agentsError: null });
			// Fetch models, agents and user config in parallel.
			Promise.all([
				missionsApi.listModels().catch(() => null),
				missionsApi.listAgents().catch(() => null),
				configApi.getUserConfig().catch(() => null),
			]).then(([modelsRes, agentsRes, userCfg]) => {
				const roleDefaults = userCfg?.role_defaults ?? {};
				const planning = roleDefaults.planning ?? {};
				const dev = roleDefaults.dev ?? {};

				if (modelsRes) {
					const allModels = Object.values(modelsRes.modelsByProvider).flat();
					// Prefer the configured planning role's primary model; otherwise
					// fall back to the backend's default model. No hardcoded ids.
					const fallback = modelsRes.default || (allModels[0]?.id ?? "");
					const planningModel = planning.model || fallback;
					const workerModel = dev.model || fallback;
					// Recommended set = every model/fallback configured across roles.
					const recommendedModels = Array.from(
						new Set(
							Object.values(roleDefaults)
								.flatMap((rd) => [rd?.model, ...(rd?.fallbacks ?? [])])
								.filter((m): m is string => !!m),
						),
					);
					update({
						models: allModels,
						recommendedModels,
						goalRefineModel: planningModel,
						planModel: planningModel,
						planRefineModel: planningModel,
						workerModel: workerModel,
						modelsError: null,
					});
				} else {
					update({ modelsError: "Failed to load models" });
				}

				if (agentsRes) {
					update({
						agents: agentsRes.agents,
						plannerAgent: planning.agent || (agentsRes.agents[0] ?? ""),
						workerAgent: dev.agent || (agentsRes.agents[0] ?? ""),
						agentsError: null,
					});
				} else {
					update({ agentsError: "Failed to load agents" });
				}
			});
		}
	}, [open, storeProjects, update]);

	// Reset wizard state when modal opens
	useEffect(() => {
		if (open) {
			setState((prev) => ({
				...prev,
				step: 1,
				selectedProject: null,
				browseMode: false,
				browsePath: "",
				browseEntries: [],
				browseLoading: false,
				browseError: null,
				newFolderName: "",
				creatingFolder: false,
				baseBranch: "main",
				autoCheckout: true,
				createFeatureBranch: true,
				gitInfo: null,
				gitLoading: false,
				initingGit: false,
				goal: "",
				refinedGoal: "",
				refining: false,
				chatMessages: [],
				goalAccepted: false,
				plan: null,
				planGenerating: false,
				submitting: false,
				error: null,
			}));
		}
	}, [open]);

	// ─── Step 1: Project ──────────────────────────────────────────────

	const canProceedStep1 = Boolean(
		state.selectedProject && state.selectedProject.name,
	);

	const handleSelectProject = async (project: Project) => {
		update({
			selectedProject: project,
			baseBranch: project.branch ?? "main",
			gitInfo: null,
			gitLoading: true,
		});
		try {
			const info = await missionsApi.getProjectGitInfo(project.name);
			update({
				gitInfo: info,
				gitLoading: false,
				baseBranch: info.currentBranch || "main",
			});
		} catch {
			update({ gitInfo: null, gitLoading: false });
		}
	};

	// handleInitGit turns a non-git project into a repo so worktrees can branch.
	const handleInitGit = async () => {
		if (!state.selectedProject) return;
		update({ initingGit: true, error: null });
		try {
			const res = await missionsApi.initProjectGit(state.selectedProject.name);
			update({ initingGit: false, gitInfo: res.git });
		} catch (err) {
			update({ initingGit: false, error: String(err) });
		}
	};

	// ─── File Browser ─────────────────────────────────────────────────

	const handleOpenBrowser = async () => {
		update({ browseMode: true, browseLoading: true, browseError: null, browseEntries: [] });
		try {
			const res = await missionsApi.browseFS();
			update({ browsePath: res.path, browseEntries: res.entries, browseLoading: false });
		} catch (err) {
			update({ browseError: String(err), browseLoading: false });
		}
	};

	const handleBrowseTo = async (dirPath: string) => {
		update({ browseLoading: true, browseError: null, browseEntries: [] });
		try {
			const res = await missionsApi.browseFS(dirPath);
			update({ browsePath: res.path, browseEntries: res.entries, browseLoading: false });
		} catch (err) {
			update({ browseError: String(err), browseLoading: false });
		}
	};

	const handleBrowseUp = () => {
		const parent = state.browsePath.replace(/\/[^/]+\/?$/, "") || "/";
		handleBrowseTo(parent === state.browsePath ? "/" : parent);
	};

	const handleSelectFolderForPath = async (folderPath: string) => {
		const folderName = folderPath.split("/").filter(Boolean).pop() ?? folderPath;
		try {
			const project = await missionsApi.createProject(folderName, folderPath);
			update({
				projects: [...state.projects, project],
				selectedProject: project,
				browseMode: false,
				baseBranch: project.branch ?? "main",
			});
		} catch (err) {
			// If project already exists (409), find it in the list and select it
			const existing = state.projects.find((p) => p.name === folderName || p.path === folderPath);
			if (existing) {
				update({
					selectedProject: existing,
					browseMode: false,
					baseBranch: existing.branch ?? "main",
				});
				return;
			}
			// For other errors, try to refresh the projects list and find it
			try {
				const projects = await missionsApi.listProjects();
				const found = projects.find((p) => p.name === folderName || p.path === folderPath);
				if (found) {
					update({
						projects,
						selectedProject: found,
						browseMode: false,
						baseBranch: found.branch ?? "main",
					});
					return;
				}
			} catch {
				// ignore — fall through to error display
			}
			// If all else fails, show the error
			update({ browseError: String(err) });
		}
	};

	const handleSelectFolder = () => handleSelectFolderForPath(state.browsePath);

	const handleCreateFolder = async () => {
		const name = state.newFolderName.trim();
		if (!name) {
			update({ browseError: "Enter a folder name" });
			return;
		}
		update({ creatingFolder: true, browseError: null });
		try {
			const { path } = await missionsApi.createFolder(state.browsePath, name);
			// Clear the input, then reuse the existing select flow which registers
			// the new folder as a project and selects it for the mission.
			update({ newFolderName: "" });
			await handleSelectFolderForPath(path);
		} catch (err) {
			update({ browseError: String(err) });
		} finally {
			update({ creatingFolder: false });
		}
	};

	// ─── Step 2: Branch ────────────────────────────────────────────────

	const canProceedStep2 = true;

	const branchPreview = state.createFeatureBranch && state.goal
		? `mission/${slugify(state.goal)}`
		: state.baseBranch;

	// ─── Step 3: Goal ──────────────────────────────────────────────────

	const canProceedStep3 = state.goalAccepted && state.refinedGoal.trim().length > 0;

	const handleRefineWithAI = async () => {
		if (!state.goal.trim()) return;
		update({ refining: true, error: null });
		const userMsg = { role: "user" as const, text: state.goal };
		const thinkingMsg = { role: "ai" as const, text: "Thinking…" };
		update({ chatMessages: [...state.chatMessages, userMsg, thinkingMsg] });
		try {
			const refined = await missionsApi.refineGoal(state.goal, undefined, state.goalRefineModel);
			update({
				chatMessages: [...state.chatMessages, userMsg, { role: "ai" as const, text: "Here's a structured version of your goal:" }],
				refinedGoal: refined.refined,
				refining: false,
			});
		} catch (err) {
			update({
				chatMessages: [...state.chatMessages, userMsg, { role: "ai" as const, text: `Error: ${String(err)}` }],
				refining: false,
				error: `Refinement failed: ${String(err)}`,
			});
		}
	};

	const handleAcceptGoal = () => {
		if (!state.refinedGoal.trim()) return;
		update({ goalAccepted: true });
	};

	const handleSkipRefinement = () => {
		if (!state.goal.trim()) return;
		// Use the raw goal as-is, without AI refinement.
		update({ refinedGoal: state.goal.trim(), goalAccepted: true });
	};

	// handleAutoRun is the one-shot autonomous one-shot flow: take the goal as
	// written, plan + approve + run in the backend without the review step.
	const handleAutoRun = async () => {
		if (!state.goal.trim() || !state.selectedProject) return;
		update({ submitting: true, error: null });
		try {
			const missionId = await storeAutoMission({
				goal: state.goal.trim(),
				project: state.selectedProject.name,
				model: state.workerModel || undefined,
				agent: state.workerAgent || undefined,
				autoApprove: true,
			});
			storeSelectMission(missionId);
			onClose();
		} catch (err) {
			update({ error: String(err), submitting: false });
		}
	};

	// ─── Step 4: Config & Review ───────────────────────────────────────

	const canProceedStep4 = true;

	const handleGeneratePlan = async () => {
		update({ planGenerating: true, error: null, plan: null, planMarkdown: "" });
		try {
			const plan = await storeGeneratePlan(
				state.refinedGoal,
				state.selectedProject?.name,
				state.planModel,
				state.plannerAgent,
			);
			const md = planToMarkdown(plan);
			update({ plan, planMarkdown: md, planGenerating: false });
		} catch (err) {
			update({ error: String(err), planGenerating: false });
		}
	};

	const handleApprovePlan = async () => {
		if (!state.plan) return;
		update({ submitting: true, error: null });
		try {
			const planWithWorker = {
				...state.plan,
				model: state.workerModel || state.plan.model,
				agent: state.workerAgent || state.plan.agent,
			};
			const mission = await storeApprovePlan(planWithWorker);
			// Kick off execution immediately so the user does not have to press
			// a separate "Run" button after approving the plan.
			try {
				await storeRunMission(mission.id);
			} catch (runErr) {
				console.error("runMission failed after approval", runErr);
			}
			storeSelectMission(mission.id);
			onClose();
		} catch (err) {
			update({ error: String(err), submitting: false });
		}
	};

	const handleDiscardPlan = () => {
		update({ plan: null, planMarkdown: "", planChatMessages: [], error: null });
	};

	const handleRetryModelsAgents = () => {
		update({ modelsError: null, agentsError: null });
		missionsApi.listModels().then((res) => {
			const allModels = Object.values(res.modelsByProvider).flat();
			const chosen = res.default || (allModels[0]?.id ?? "");
			update({
				models: allModels,
				goalRefineModel: chosen,
				planModel: chosen,
				planRefineModel: chosen,
				workerModel: chosen,
				modelsError: null,
			});
		}).catch((err) => {
			update({ modelsError: `Failed to load models: ${err.message ?? err}` });
		});
		missionsApi.listAgents().then((res) => {
			update({
				agents: res.agents,
				plannerAgent: res.agents[0] ?? "",
				workerAgent: res.agents[0] ?? "",
				agentsError: null,
			});
		}).catch((err) => {
			update({ agentsError: `Failed to load agents: ${err.message ?? err}` });
		});
	};

	const [startingOpencode, setStartingOpencode] = useState(false);
	const handleStartOpencode = async () => {
		setStartingOpencode(true);
		try {
			await missionsApi.startOpencode();
			// Wait a bit for server to fully start, then retry
			setTimeout(() => {
				handleRetryModelsAgents();
				setStartingOpencode(false);
			}, 3000);
		} catch (err) {
			update({ error: `Failed to start opencode: ${err}` });
			setStartingOpencode(false);
		}
	};

	const handleRefinePlan = async (message: string) => {
		if (!message.trim() || !state.planMarkdown.trim()) return;
		const userMsg = { role: "user" as const, text: message };
		update({
			planChatMessages: [...state.planChatMessages, userMsg],
			planRefining: true,
			error: null,
		});
		try {
			const { refined } = await missionsApi.refineGoal(
				state.planMarkdown,
				`User wants to modify the plan: ${message}`,
				state.planRefineModel,
			);
			update({
				planMarkdown: refined,
				planChatMessages: [
					...state.planChatMessages,
					userMsg,
					{ role: "ai" as const, text: "Plan updated!" },
				],
				planRefining: false,
			});
		} catch (err) {
			update({
				planChatMessages: [
					...state.planChatMessages,
					userMsg,
					{ role: "ai" as const, text: `Error: ${String(err)}` },
				],
				planRefining: false,
			});
		}
	};

	// ─── Navigation ────────────────────────────────────────────────────

	const goNext = () => {
		if (state.step < 4) update({ step: state.step + 1 });
	};

	const goBack = () => {
		if (state.step > 1) update({ step: state.step - 1 });
	};

	const canGoNext = () => {
		switch (state.step) {
			case 1: return canProceedStep1;
			case 2: return canProceedStep2;
			case 3: return canProceedStep3;
			case 4: return canProceedStep4;
			default: return false;
		}
	};

	// ─── Render helpers ────────────────────────────────────────────────

	const renderStepIndicator = () => (
		<div className="wizard-steps">
			{STEPS.map((s, i) => {
				const isActive = s.num === state.step;
				const isDone = s.num < state.step;
				return (
					<div
						key={s.num}
						className={`wizard-step ${isActive ? "active" : ""} ${isDone ? "done" : ""}`}
					>
						<div className="wizard-step-circle">
							{isDone ? (
								<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
									<polyline points="20 6 9 17 4 12" />
								</svg>
							) : (
								s.num
							)}
						</div>
						<span className="wizard-step-label">{s.label}</span>
						{i < STEPS.length - 1 && <div className={`wizard-step-line ${isDone ? "done" : ""}`} />}
					</div>
				);
			})}
		</div>
	);

	const renderProjectStep = () => (
		<div className="wizard-step-content">
			<h3 className="wizard-step-title">Select Project</h3>
			<p className="wizard-step-desc">Choose which project this mission is for, or add a new one.</p>

			<div className="project-list">
				{state.projects.map((p) => {
					const isSelected = state.selectedProject?.name === p.name;
					// Show the real current branch when this card is the selected one
					// and we have git info; fall back to the static project.branch.
					const branch = isSelected && state.gitInfo?.currentBranch
						? state.gitInfo.currentBranch
						: p.branch;
					const isGit = isSelected ? state.gitInfo?.isGitRepo ?? true : true;
					return (
						<button
							key={p.name}
							type="button"
							className={`project-card ${isSelected ? "selected" : ""}`}
							onClick={() => handleSelectProject(p)}
						>
							<div className="project-card-head">
								<span className="project-card-name">{p.name}</span>
								{isSelected && (
									<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
										<polyline points="20 6 9 17 4 12" />
									</svg>
								)}
							</div>
							<span className="project-card-path" data-tip={p.path}>{p.path}</span>
							{branch && <span className="project-card-branch">Branch: {branch}</span>}
							{isSelected && !isGit && (
								<span className="project-card-branch" style={{ background: "var(--danger-soft)", color: "var(--danger)" }}>
									No git repo
								</span>
							)}
						</button>
					);
				})}
				{state.projects.length === 0 && (
					<p className="muted" style={{ gridColumn: "1 / -1", textAlign: "center", padding: "var(--space-4)" }}>
						No projects yet. Add one below.
					</p>
				)}
			</div>

			{/* Git init offer when the selected project is not a git repo */}
			{state.selectedProject && state.gitInfo && !state.gitInfo.isGitRepo && (
				<div className="alert alert-warning" style={{ marginTop: "var(--space-3)" }}>
					<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
						<circle cx="12" cy="12" r="10" />
						<line x1="12" y1="8" x2="12" y2="12" />
						<line x1="12" y1="16" x2="12.01" y2="16" />
					</svg>
					<span>
						This project is not a git repository. Worktree-based execution needs git.
					</span>
					<button
						type="button"
						className="btn btn-primary btn-sm"
						style={{ marginLeft: "var(--space-2)" }}
						disabled={state.initingGit}
						onClick={handleInitGit}
					>
						{state.initingGit ? "Initializing…" : "Initialize git"}
					</button>
				</div>
			)}
			{state.gitLoading && state.selectedProject && (
				<p className="muted" style={{ marginTop: "var(--space-2)", fontSize: "0.82rem" }}>
					Loading git info…
				</p>
			)}

			{!state.browseMode ? (
				<button
					type="button"
					className="btn btn-ghost"
					onClick={handleOpenBrowser}
					style={{ marginTop: "var(--space-3)" }}
				>
					+ Browse Folders
				</button>
			) : (
				<div className="fs-browser" style={{ marginTop: "var(--space-3)" }}>
					<div className="fs-breadcrumbs">
						{state.browsePath === "/" ? (
							<span className="fs-breadcrumb-current">/</span>
						) : (
							<>
								<button type="button" className="fs-breadcrumb" onClick={() => handleBrowseTo("/")}>
									/
								</button>
								{state.browsePath.split("/").filter(Boolean).map((segment, i, arr) => {
									const fullPath = "/" + arr.slice(0, i + 1).join("/");
									const isLast = i === arr.length - 1;
									return (
										<span key={fullPath} className="fs-breadcrumb-group">
											<span className="fs-breadcrumb-sep">/</span>
											{isLast ? (
												<span className="fs-breadcrumb-current">{segment}</span>
											) : (
												<button type="button" className="fs-breadcrumb" onClick={() => handleBrowseTo(fullPath)}>
													{segment}
												</button>
											)}
										</span>
									);
								})}
							</>
						)}
					</div>

					<div className="fs-list">
						{state.browseLoading && <p className="fs-empty">Loading…</p>}
						{state.browseError && <p className="fs-empty fs-error">{state.browseError}</p>}
						{!state.browseLoading && !state.browseError && state.browsePath !== "/" && (
							<button type="button" className="fs-entry fs-entry-up" onClick={handleBrowseUp}>
								<svg className="fs-entry-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
									<polyline points="15 18 9 12 15 6" />
								</svg>
								<span>..</span>
							</button>
						)}
						{!state.browseLoading && !state.browseError &&
							state.browseEntries
								.filter((e) => e.isDir)
								.sort((a, b) => {
									const ah = a.name.startsWith(".") ? 1 : 0;
									const bh = b.name.startsWith(".") ? 1 : 0;
									if (ah !== bh) return ah - bh;
									return a.name.localeCompare(b.name);
								})
								.map((e) => {
									const childPath =
										state.browsePath === "/" ? `/${e.name}` : `${state.browsePath}/${e.name}`;
									const isHidden = e.name.startsWith(".");
									return (
										<button
											key={e.name}
											type="button"
											className={`fs-entry${isHidden ? " fs-entry-hidden" : ""}`}
											onClick={() => handleBrowseTo(childPath)}
										>
											<svg className="fs-entry-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
												<path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
											</svg>
											<span>{e.name}</span>
										</button>
									);
								})}
					</div>

					<div className="fs-browser-actions">
						<span className="fs-current-path">{state.browsePath}</span>
						<div className="row" style={{ gap: "var(--space-2)", flex: 1 }}>
							<input
								type="text"
								className="fs-new-folder-input"
								placeholder="new-folder"
								value={state.newFolderName}
								onChange={(e) => update({ newFolderName: e.target.value, browseError: null })}
								onKeyDown={(e) => {
									if (e.key === "Enter" && !state.creatingFolder && state.newFolderName.trim()) {
										e.preventDefault();
										handleCreateFolder();
									}
								}}
								disabled={state.creatingFolder}
							/>
							<button
								type="button"
								className="btn btn-ghost"
								onClick={handleCreateFolder}
								disabled={state.creatingFolder || !state.newFolderName.trim()}
							>
								{state.creatingFolder ? "Creating…" : "Create Folder"}
							</button>
						</div>
						<div className="row" style={{ gap: "var(--space-2)" }}>
							<button type="button" className="btn btn-primary" onClick={handleSelectFolder}>
								Select This Folder
							</button>
							<button type="button" className="btn btn-ghost" onClick={() => update({ browseMode: false })}>
								Cancel
							</button>
						</div>
					</div>
				</div>
			)}
		</div>
	);

	const renderBranchStep = () => {
		const branches = state.gitInfo?.branches ?? [];
		const hasBranchList = branches.length > 0;
		const currentBranch = state.gitInfo?.currentBranch ?? state.selectedProject?.branch ?? "—";
		return (
		<div className="wizard-step-content">
			<h3 className="wizard-step-title">Branch & Setup</h3>
			<p className="wizard-step-desc">Configure how the mission interacts with version control.</p>

			{state.selectedProject && (
				<div className="field">
					<label className="field-label">Current Branch</label>
					<div className="input fake-input">
						{currentBranch}
					</div>
				</div>
			)}

			<div className="field">
				<label className="field-label" htmlFor="base-branch">Base Branch</label>
				{hasBranchList ? (
					<select
						id="base-branch"
						className="input"
						value={state.baseBranch}
						onChange={(e) => update({ baseBranch: e.target.value })}
					>
						{branches.map((b) => (
							<option key={b} value={b}>{b}</option>
						))}
						{/* Allow a custom value not in the list */}
						{state.baseBranch && !branches.includes(state.baseBranch) && (
							<option value={state.baseBranch}>{state.baseBranch}</option>
						)}
					</select>
				) : (
					<input
						id="base-branch"
						className="input"
						type="text"
						value={state.baseBranch}
						onChange={(e) => update({ baseBranch: e.target.value })}
						placeholder="main"
					/>
				)}
			</div>

			<div className="field">
				<label className="checkbox-row">
					<input
						type="checkbox"
						checked={state.autoCheckout}
						onChange={(e) => update({ autoCheckout: e.target.checked })}
					/>
					<span>Auto checkout & pull base branch before starting</span>
				</label>
			</div>

			<div className="field">
				<label className="checkbox-row">
					<input
						type="checkbox"
						checked={state.createFeatureBranch}
						onChange={(e) => update({ createFeatureBranch: e.target.checked })}
					/>
					<span>Create feature branch for this mission</span>
				</label>
			</div>

			{state.createFeatureBranch && (
				<div className="field">
					<label className="field-label">Branch Preview</label>
					<div className="input fake-input mono">{branchPreview}</div>
				</div>
			)}
		</div>
		);
	};

	const renderGoalStep = () => (
		<div className="wizard-step-content">
			<h3 className="wizard-step-title">Define Your Goal</h3>
			<p className="wizard-step-desc">
				Describe what you want the mission to accomplish. Optionally use "Refine with AI" to clarify scope and requirements, or skip it and use your goal as written.
			</p>

			<div className="field">
				<label className="field-label" htmlFor="mission-goal">Mission Goal</label>
				<textarea
					id="mission-goal"
					className="input wizard-textarea"
					rows={4}
					value={state.goal}
					onChange={(e) => update({ goal: e.target.value })}
					placeholder="e.g., Add dark mode support with a theme toggle button in the header"
					disabled={state.goalAccepted}
				/>
			</div>

			{!state.goalAccepted && (
				<>
					<div className="field" style={{ marginTop: "var(--space-2)" }}>
						<ModelCombobox
							id="goal-refine-model-inline"
							label="Refinement model"
							value={state.goalRefineModel}
							models={state.models}
							recommended={state.recommendedModels}
							onChange={(v) => update({ goalRefineModel: v })}
						/>
					</div>
					<div className="row" style={{ gap: "var(--space-2)", marginTop: "var(--space-2)" }}>
						<button
							type="button"
							className="btn btn-primary"
							disabled={!state.goal.trim() || state.refining}
							onClick={handleRefineWithAI}
						>
							{state.refining ? (
								<><div className="spinner"></div> Refining…</>
							) : (
								"Refine with AI"
							)}
						</button>
						<button
							type="button"
							className="btn btn-ghost"
							disabled={!state.goal.trim() || state.refining}
							onClick={handleSkipRefinement}
							data-tip="Use the goal as written, without AI refinement"
						>
							Skip — use raw goal
						</button>
						<button
							type="button"
							className="btn btn-secondary"
							disabled={!state.goal.trim() || state.refining || state.submitting || !state.selectedProject}
							onClick={handleAutoRun}
							data-tip="Plan, approve and run autonomously — skip the review step"
							data-tip-pos="left"
							style={{ marginLeft: "auto" }}
						>
							{state.submitting ? (
								<><div className="spinner"></div> Starting…</>
							) : (
								"⚡ Auto-run"
							)}
						</button>
					</div>
				</>
			)}

			{state.chatMessages.length > 0 && (
				<div className="wizard-chat">
					{state.chatMessages.map((msg, i) => (
						<div key={i} className={`chat-message chat-${msg.role}`}>
							<div className="chat-avatar">{msg.role === "ai" ? "AI" : "You"}</div>
							<div className="chat-bubble">{msg.text}</div>
						</div>
					))}
				</div>
			)}

			{state.refinedGoal && !state.goalAccepted && (
				<div className="field" style={{ marginTop: "var(--space-3)" }}>
					<label className="field-label">Refined Goal (editable)</label>
					<textarea
						className="input wizard-textarea"
						rows={8}
						value={state.refinedGoal}
						onChange={(e) => update({ refinedGoal: e.target.value })}
					/>
					<div className="row" style={{ gap: "var(--space-2)", marginTop: "var(--space-2)" }}>
						<button type="button" className="btn btn-primary" onClick={handleAcceptGoal}>
							Accept Goal
						</button>
						<button
							type="button"
							className="btn btn-ghost"
							onClick={() => {
								update({ chatMessages: [], refinedGoal: "", goalAccepted: false });
							}}
						>
							Start Over
						</button>
					</div>
				</div>
			)}

			{state.goalAccepted && (
				<div className="alert alert-success" style={{ marginTop: "var(--space-3)" }}>
					<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
						<polyline points="20 6 9 17 4 12" />
					</svg>
					Goal accepted! You can review it in the next step.
				</div>
			)}
		</div>
	);


	const renderConfigStep = () => (
		<div className="wizard-step-content">
			<h3 className="wizard-step-title">Review & Configure</h3>
			<p className="wizard-step-desc">Configure your mission settings and review the generated plan.</p>

			{/* Models & Agents Error Banners */}
			{state.modelsError && (
				<div className="alert alert-warning" style={{ marginBottom: "var(--space-3)" }}>
					<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
						<circle cx="12" cy="12" r="10" />
						<line x1="12" y1="8" x2="12" y2="12" />
						<line x1="12" y1="16" x2="12.01" y2="16" />
					</svg>
					{state.modelsError} — check opencode connection.
					<button type="button" className="btn btn-ghost btn-sm" onClick={handleRetryModelsAgents} style={{ marginLeft: "var(--space-2)" }}>
						Retry
					</button>
					<button type="button" className="btn btn-primary btn-sm" onClick={handleStartOpencode} disabled={startingOpencode} style={{ marginLeft: "var(--space-2)" }}>
						{startingOpencode ? "Starting..." : "Start opencode"}
					</button>
				</div>
			)}
			{state.agentsError && (
				<div className="alert alert-warning" style={{ marginBottom: "var(--space-3)" }}>
					<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
						<circle cx="12" cy="12" r="10" />
						<line x1="12" y1="8" x2="12" y2="12" />
						<line x1="12" y1="16" x2="12.01" y2="16" />
					</svg>
					{state.agentsError} — check opencode connection.
					<button type="button" className="btn btn-ghost btn-sm" onClick={handleRetryModelsAgents} style={{ marginLeft: "var(--space-2)" }}>
						Retry
					</button>
					<button type="button" className="btn btn-primary btn-sm" onClick={handleStartOpencode} disabled={startingOpencode} style={{ marginLeft: "var(--space-2)" }}>
						{startingOpencode ? "Starting..." : "Start opencode"}
					</button>
				</div>
			)}

			{/* Config Fields */}
			<div className="review-config">
				<div className="field" style={{ gridColumn: "1 / -1" }}>
					<label className="field-label">Models per phase</label>
					<p className="muted" style={{ fontSize: "0.85rem", margin: "0 0 var(--space-2)" }}>
						Pick a different model for each phase if you want. Defaults to the same model everywhere. Search to find more models.
					</p>
					<div className="review-config" style={{ marginTop: "var(--space-2)" }}>
						<ModelCombobox
							id="goal-refine-model"
							label="Goal refinement"
							value={state.goalRefineModel}
							models={state.models}
							recommended={state.recommendedModels}
							onChange={(v) => update({ goalRefineModel: v })}
						/>
						<ModelCombobox
							id="plan-model"
							label="Plan generation"
							value={state.planModel}
							models={state.models}
							recommended={state.recommendedModels}
							onChange={(v) => update({ planModel: v })}
						/>
						<ModelCombobox
							id="plan-refine-model"
							label="Plan refinement"
							value={state.planRefineModel}
							models={state.models}
							recommended={state.recommendedModels}
							onChange={(v) => update({ planRefineModel: v })}
						/>
						<ModelCombobox
							id="worker-model"
							label="Worker execution"
							value={state.workerModel}
							models={state.models}
							recommended={state.recommendedModels}
							onChange={(v) => update({ workerModel: v })}
						/>
					</div>
				</div>

				<div className="field">
					<label className="field-label">Max Parallel Workers</label>
					<div className="field-with-value">
						<input
							type="range"
							min={1}
							max={8}
							step={1}
							value={state.maxParallel}
							onChange={(e) => update({ maxParallel: Number(e.target.value) })}
							className="range-input"
						/>
						<span className="range-value">{state.maxParallel}</span>
					</div>
				</div>

				<div className="field">
					<label className="field-label">Planner Agent</label>
					<YdSelect
						options={state.agents.map((a) => ({ value: a, label: a }))}
						value={state.plannerAgent}
						onChange={(v) => update({ plannerAgent: v })}
						placeholder="No agents available"
						disabled={state.agents.length === 0}
					/>
				</div>

				<div className="field">
					<label className="field-label">Worker Agent</label>
					<YdSelect
						options={state.agents.map((a) => ({ value: a, label: a }))}
						value={state.workerAgent}
						onChange={(v) => update({ workerAgent: v })}
						placeholder="No agents available"
						disabled={state.agents.length === 0}
					/>
				</div>
			</div>

			{/* Mission Summary */}
			<div className="review-summary">
				<h4 className="review-summary-title">Mission Summary</h4>
				<div className="review-grid">
					<div className="review-item">
						<span className="review-label">Project</span>
						<span className="review-value">{state.selectedProject?.name ?? "—"}</span>
					</div>
					<div className="review-item">
						<span className="review-label">Path</span>
						<span className="review-value">{state.selectedProject?.path ?? "—"}</span>
					</div>
					<div className="review-item">
						<span className="review-label">Base Branch</span>
						<span className="review-value">{state.baseBranch}</span>
					</div>
					<div className="review-item">
						<span className="review-label">Goal</span>
						<span className="review-value" style={{ whiteSpace: "pre-wrap" }}>{state.refinedGoal.slice(0, 200)}{state.refinedGoal.length > 200 ? "…" : ""}</span>
					</div>
				</div>
			</div>

			{/* Editable Plan */}
			{state.plan && (
				<div className="plan-editor" style={{ marginTop: "var(--space-3)" }}>
					<h4 className="review-summary-title">Mission Plan</h4>
					<textarea
						className="input"
						rows={16}
						value={state.planMarkdown}
						onChange={(e) => update({ planMarkdown: e.target.value })}
						style={{ fontFamily: "monospace", fontSize: "0.85rem", whiteSpace: "pre-wrap", resize: "vertical" }}
					/>
					{state.planChatMessages.length > 0 && (
						<div className="wizard-chat" style={{ marginTop: "var(--space-2)" }}>
							{state.planChatMessages.map((msg, i) => (
								<div key={i} className={`chat-message chat-${msg.role}`}>
									<div className="chat-avatar">{msg.role === "ai" ? "AI" : "You"}</div>
									<div className="chat-bubble">{msg.text}</div>
								</div>
							))}
						</div>
					)}
					<div className="plan-refine-bar" style={{ display: "flex", gap: "var(--space-2)", marginTop: "var(--space-2)" }}>
						<input
							type="text"
							className="input"
							placeholder="Ask AI to refine the plan…"
							disabled={state.planRefining}
							onKeyDown={(e) => {
								if (e.key === "Enter" && !state.planRefining) {
									const input = e.currentTarget;
									handleRefinePlan(input.value);
									input.value = "";
								}
							}}
						/>
						<button
							type="button"
							className="btn btn-ghost btn-sm"
							disabled={state.planRefining}
							onClick={() => {
								const input = document.querySelector(".plan-refine-bar input") as HTMLInputElement;
								if (input?.value) {
									handleRefinePlan(input.value);
									input.value = "";
								}
							}}
						>
							{state.planRefining ? "Refining…" : "Send"}
						</button>
					</div>
				</div>
			)}

			{state.error && (
				<div className="alert alert-danger" style={{ marginTop: "var(--space-3)" }}>
					<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
						<circle cx="12" cy="12" r="10" />
						<line x1="12" y1="8" x2="12" y2="12" />
						<line x1="12" y1="16" x2="12.01" y2="16" />
					</svg>
					{state.error}
				</div>
			)}
		</div>
	);
	const renderStepContent = () => {
		switch (state.step) {
			case 1: return renderProjectStep();
			case 2: return renderBranchStep();
			case 3: return renderGoalStep();
			case 4: return renderConfigStep();
			default: return null;
		}
	};

	const stepTitle = STEPS[state.step - 1]?.label ?? "";
	const modalTitle = `New Mission — ${stepTitle}`;
	const modalSubtitle =
		state.step === 1 ? "Choose a project for your mission" :
		state.step === 2 ? "Configure branch and version control options" :
		state.step === 3 ? "Define and refine your mission goal" :
		"Review configuration and create the mission";

	const footer = (
		<>
			<button type="button" className="btn btn-ghost" onClick={onClose}>
				Cancel
			</button>
			<div className="row" style={{ gap: "var(--space-2)", marginLeft: "auto" }}>
				{state.step > 1 && (
					<button type="button" className="btn btn-ghost" onClick={goBack}>
						Back
					</button>
				)}
				{state.step < 4 ? (
					<button
						type="button"
						className="btn btn-primary"
						disabled={!canGoNext()}
						onClick={goNext}
					>
						Next
					</button>
				) : state.plan === null ? (
					<button
						type="button"
						className="btn btn-primary"
						disabled={state.planGenerating}
						onClick={handleGeneratePlan}
					>
						{state.planGenerating ? (
							<><div className="spinner"></div> Generating Plan…</>
						) : (
							"Generate Plan"
						)}
					</button>
				) : (
					<>
						<button
							type="button"
							className="btn btn-secondary"
							onClick={handleDiscardPlan}
							disabled={state.submitting}
						>
							Discard Plan
						</button>
						<button
							type="button"
							className="btn btn-primary"
							disabled={state.submitting}
							onClick={handleApprovePlan}
						>
							{state.submitting ? (
								<><div className="spinner"></div> Creating…</>
							) : (
								"Approve & Create Mission"
							)}
						</button>
					</>
				)}
			</div>
		</>
	);

	return (
		<Modal open={open} onClose={onClose} title={modalTitle} subtitle={modalSubtitle} footer={footer} width="960px">
			{renderStepIndicator()}
			{renderStepContent()}
		</Modal>
	);
}
