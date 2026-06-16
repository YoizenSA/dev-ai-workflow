import { useState, type ReactNode } from "react";
import Sidebar from "./Sidebar";
import "./Layout.css";

interface LayoutProps {
	children: ReactNode;
}

export default function Layout({ children }: LayoutProps) {
	const [sidebarOpen, setSidebarOpen] = useState(false);

	return (
		<div className="app-shell">
			<Sidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)} />

			{/* Scrim overlay for mobile */}
			{sidebarOpen && (
				<div className="scrim" onClick={() => setSidebarOpen(false)} />
			)}

			<div className="main-col">
				<main className="content">{children}</main>
			</div>

			{/* Mobile FAB */}
			<button
				className="mobile-fab"
				onClick={() => setSidebarOpen(true)}
				aria-label="Open menu"
			>
				<svg
					width="20"
					height="20"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					strokeWidth="2.5"
					strokeLinecap="round"
				>
					<line x1="3" y1="6" x2="21" y2="6" />
					<line x1="3" y1="12" x2="21" y2="12" />
					<line x1="3" y1="18" x2="21" y2="18" />
				</svg>
			</button>
		</div>
	);
}
