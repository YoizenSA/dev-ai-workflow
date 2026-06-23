import { useState, type ReactNode } from "react";
import { Menu } from "lucide-react";
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
				<Menu size={20} strokeWidth={2.5} />
			</button>
		</div>
	);
}
