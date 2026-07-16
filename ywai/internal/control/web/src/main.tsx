import React from "react";
import ReactDOM from "react-dom/client";
import { configureBoneyard } from "boneyard-js/react";
import App from "./App";
import "./bones/registry";
import "./styles/theme/index.css";
import "./styles/globals.css";

// Global skeleton look — dark UI defaults; light mode uses `color`.
configureBoneyard({
	color: "rgba(0, 0, 0, 0.08)",
	darkColor: "rgba(255, 255, 255, 0.08)",
	animate: "shimmer",
	stagger: true,
	transition: true,
	select: "viewport",
});

// Register the service worker so push notifications can activate.
// Without this, navigator.serviceWorker.ready never resolves and the
// "Enable Push Notifications" button hangs on "Enabling…".
if ("serviceWorker" in navigator) {
	navigator.serviceWorker.register("/sw.js").catch(() => {})
}

ReactDOM.createRoot(document.getElementById("root")!).render(
	<React.StrictMode>
		<App />
	</React.StrictMode>,
);
