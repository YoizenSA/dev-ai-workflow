import { useState } from "react";
import { Link, useLocation } from "react-router-dom";
import {
	Brain,
	ChevronDown,
	ChevronRight,
	Cloud,
	Heart,
	LineChart,
	MessageSquare,
	PanelLeftClose,
	PanelLeftOpen,
	Settings,
	Store,
	Workflow,
} from "lucide-react";
import { ThemeToggle } from "../shared/ThemeToggle";
import { VersionUpdate } from "./VersionUpdate";

interface SidebarProps {
	open: boolean;
	onClose: () => void;
	collapsed?: boolean;
	onToggleCollapse?: () => void;
}

// Core nav (excludes Azure DevOps, rendered after the Beta group).
const NAV_ITEMS = [
	{
		path: "/workflows",
		label: "Workflows",
		icon: <Workflow size={20} />,
	},
	{
		path: "/memories",
		label: "Memories",
		icon: <Brain size={20} />,
	},
	{
		path: "/evals",
		label: "Evals",
		icon: <LineChart size={20} />,
	},
	{
		path: "/settings",
		label: "Settings",
		icon: <Settings size={20} />,
	},
	{
		path: "/mcp-store",
		label: "MCP Store",
		icon: <Store size={20} />,
	},
	{
		path: "/ado",
		label: "Azure DevOps",
		icon: <Cloud size={20} />,
	},
];

const BETA_ITEMS = [
	{
		path: "/chat",
		label: "Chat",
		icon: <MessageSquare size={20} />,
	},
	{
		path: "/health",
		label: "Health",
		icon: <Heart size={20} />,
	},
];

export default function Sidebar({ open, onClose, collapsed, onToggleCollapse }: SidebarProps) {
	const [betaOpen, setBetaOpen] = useState(false);
	const location = useLocation();
	// Core items before ADO; ADO stays after the Beta group.
	const coreNav = NAV_ITEMS.slice(0, -1);
	const adoNav = NAV_ITEMS.slice(-1);

	return (
		<aside className={`sidebar${open ? " open" : ""}`}>
			{/* Brand block */}
			<div className="brand">
				<div className="brand-mark">
					<img
						src="/icon.svg"
						alt="ywai"
						className="brand-mark-img brand-mark-img-dark"
					/>
					<img
						src="/icon-negro.svg"
						alt=""
						className="brand-mark-img brand-mark-img-light"
						aria-hidden="true"
					/>
				</div>
				<span className="brand-name"><span className="grad-text">y</span>wai</span>
				<span className="brand-sub">Control Dashboard</span>
			</div>

			{/* Navigation */}
			<nav className="nav">
				<span className="nav-section-label">CORE</span>
				{coreNav.map((item) => {
					const isActive = location.pathname === item.path;

					return (
						<Link
							key={item.path}
							to={item.path}
							className={`nav-link${isActive ? " is-active" : ""}`}
							onClick={onClose}
						>
							{item.icon}
							<span className="nav-label">{item.label}</span>
						</Link>
					);
				})}

				{/* Beta group (collapsible) */}
			<div className="nav-group">
				<button
					className="nav-group-header"
					onClick={() => setBetaOpen((v) => !v)}
					aria-expanded={betaOpen}
				>
					{betaOpen ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
					<span className="nav-group-label">Beta</span>
				</button>
				{betaOpen && (
					<div className="nav-group-items">
						{BETA_ITEMS.map((item) => {
							const isActive = location.pathname === item.path;
							return (
								<Link
									key={item.path}
									to={item.path}
									className={`nav-link${isActive ? " is-active" : ""}`}
									onClick={onClose}
								>
									{item.icon}
									<span className="nav-label">{item.label}</span>
								</Link>
							);
						})}
					</div>
				)}
			</div>

			{/* ADO standalone */}
			{adoNav.map((item) => {
				const isActive = location.pathname === item.path;
				return (
					<Link
						key={item.path}
						to={item.path}
						className={`nav-link${isActive ? " is-active" : ""}`}
						onClick={onClose}
					>
						{item.icon}
						<span className="nav-label">{item.label}</span>
					</Link>
				);
			})}
			</nav>

			{/* Sidebar foot: footer tools (theme toggle).
			    Pushed to the bottom by .sidebar-foot { margin-top: auto } in shell.css. */}
			<div className="sidebar-foot">
				<VersionUpdate />
				<div className="foot-tools">
					<ThemeToggle />
					{onToggleCollapse && (
						<button
							className="btn btn-icon"
							onClick={onToggleCollapse}
							aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
							title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
						>
							{collapsed ? <PanelLeftOpen size={18} /> : <PanelLeftClose size={18} />}
						</button>
					)}
				</div>
			</div>
		</aside>
	);
}
