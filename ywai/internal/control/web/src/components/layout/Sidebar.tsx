import { Link, useLocation } from "react-router-dom";
import {
	Brain,
	LayoutGrid,
	LineChart,
	Settings,
	Sparkles,
} from "lucide-react";
import { useKanbanStore } from "../../stores/kanbanStore";
import { useMissionsStore } from "../../stores/missionsStore";
import { SessionSidebar } from "../kanban/SessionSidebar";

interface SidebarProps {
	open: boolean;
	onClose: () => void;
}

const NAV_ITEMS = [
	{
		path: "/",
		label: "Kanban",
		icon: <LayoutGrid size={20} />,
	},
	{
		path: "/missions",
		label: "Missions",
		icon: <Sparkles size={20} />,
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
		icon: (
			<span className="sidebar-icon-badge">MCP</span>
		),
	},
	{
		path: "/ado-config",
		label: "Azure DevOps",
		icon: (
			<span className="sidebar-icon-badge">ADO</span>
		),
	},
];

export default function Sidebar({ open, onClose }: SidebarProps) {
	const location = useLocation();
	const sessionCount = useKanbanStore(
		(s) => (s.sessions ?? []).filter((sess) => sess.status === "active").length,
	);
	const activeMissions = useMissionsStore(
		(s) =>
			(Array.isArray(s.missions) ? s.missions : []).filter(
				(m) => !["completed", "cancelled", "failed"].includes(m.status),
			).length,
	);

	return (
		<aside className={`sidebar${open ? " open" : ""}`}>
			{/* Brand block */}
			<div className="brand">
				
				<span className="brand-name"><span className="grad-text">y</span>wai</span>
				<span className="brand-sub">Control Dashboard</span>
			</div>

			{/* Navigation */}
			<nav className="nav">
				<span className="sidebar-section-label">CORE</span>
				{NAV_ITEMS.slice(0, 5).map((item) => {
					const isActive = location.pathname === item.path;
					const badge =
						item.path === "/"
							? sessionCount
							: item.path === "/missions"
								? activeMissions
								: 0;

					return (
						<Link
							key={item.path}
							to={item.path}
							className={`nav-link${isActive ? " is-active" : ""}`}
							onClick={onClose}
						>
							{item.icon}
							<span className="nav-label">{item.label}</span>
							{badge > 0 && <span className="nav-badge">{badge}</span>}
						</Link>
					);
				})}

				<span className="sidebar-section-label">PLUGINS</span>
				{NAV_ITEMS.slice(5).map((item) => {
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

			{/* Kanban: sessions live inside the main sidebar so the board
			    gets full width for its 5 columns. */}
			{location.pathname === "/" && (
				<div className="sidebar-sessions">
					<SessionSidebar />
				</div>
			)}
		</aside>
	);
}
