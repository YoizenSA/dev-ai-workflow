import { useState, type ReactNode } from "react";
import { useLocation } from "react-router-dom";
import { Menu } from "lucide-react";
import Sidebar from "./Sidebar";
import "./Layout.css";

interface LayoutProps {
	children: ReactNode;
}

export default function Layout({ children }: LayoutProps) {
	const [sidebarOpen, setSidebarOpen] = useState(false);
	// Desktop collapse (icon rail) — persisted so it survives reloads.
	const [collapsed, setCollapsed] = useState(
		() => localStorage.getItem("ywai.sidebarCollapsed") === "1",
	);
	const toggleCollapse = () => {
		setCollapsed((c) => {
			const next = !c;
			localStorage.setItem("ywai.sidebarCollapsed", next ? "1" : "0");
			return next;
		});
	};

	// On /chat the dashboard nav is forced to its thin icon rail so it doesn't
	// sit next to the chat's own session sidebar (avoids a double-sidebar). The
	// user's saved collapse preference still applies on every other route.
	const location = useLocation();
	const onChat = location.pathname.startsWith("/chat");
	const effectiveCollapsed = collapsed || onChat;

	return (
		<div className={`app-shell${effectiveCollapsed ? " collapsed" : ""}`}>
			<Sidebar
				open={sidebarOpen}
				onClose={() => setSidebarOpen(false)}
				collapsed={effectiveCollapsed}
				// On /chat the dashboard rail has no expand toggle — it would only
				// fight the chat sidebar. Keep the toggle on every other route.
				onToggleCollapse={onChat ? undefined : toggleCollapse}
			/>

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
