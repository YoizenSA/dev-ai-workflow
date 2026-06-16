import { Link, useLocation } from "react-router-dom";
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
		icon: (
			<svg
				width="19"
				height="19"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				strokeWidth="2"
				strokeLinecap="round"
				strokeLinejoin="round"
			>
				<rect x="3" y="3" width="7" height="7" rx="1" />
				<rect x="14" y="3" width="7" height="7" rx="1" />
				<rect x="3" y="14" width="7" height="7" rx="1" />
				<rect x="14" y="14" width="7" height="7" rx="1" />
			</svg>
		),
	},
	{
		path: "/missions",
		label: "Missions",
		icon: (
			<svg
				width="19"
				height="19"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				strokeWidth="2"
				strokeLinecap="round"
				strokeLinejoin="round"
			>
				<path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z" />
				<path d="m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z" />
				<path d="M9 12H4s.55-3.03 2-4c1.62-1.08 3 0 3 0" />
				<path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-3 0-3" />
			</svg>
		),
	},
	{
		path: "/memories",
		label: "Memories",
		icon: (
			<svg
				width="19"
				height="19"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				strokeWidth="2"
				strokeLinecap="round"
				strokeLinejoin="round"
			>
				<path d="M12 5a3 3 0 1 0-5.997.142 4 4 0 0 0-2.526 5.77 4 4 0 0 0 .556 6.588A4 4 0 1 0 12 18Z" />
				<path d="M12 5a3 3 0 1 1 5.997.142 4 4 0 0 1 2.526 5.77 4 4 0 0 1-.556 6.588A4 4 0 1 1 12 18Z" />
				<path d="M15 13a4.5 4.5 0 0 1-3-4 4.5 4.5 0 0 1-3 4" />
			</svg>
		),
	},
	{
		path: "/evals",
		label: "Evals",
		icon: (
			<svg
				width="19"
				height="19"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				strokeWidth="2"
				strokeLinecap="round"
				strokeLinejoin="round"
			>
				<path d="M3 3v18h18" />
				<path d="M7 14l4-4 4 4 5-5" />
				<circle cx="7" cy="14" r="1" />
				<circle cx="11" cy="10" r="1" />
				<circle cx="15" cy="14" r="1" />
				<circle cx="20" cy="9" r="1" />
			</svg>
		),
	},
	{
		path: "/settings",
		label: "Settings",
		icon: (
			<svg
				width="19"
				height="19"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				strokeWidth="2"
				strokeLinecap="round"
				strokeLinejoin="round"
			>
				<path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
				<circle cx="12" cy="12" r="3" />
			</svg>
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
				<span className="nav-section-label">Navigation</span>
				{NAV_ITEMS.map((item) => {
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
