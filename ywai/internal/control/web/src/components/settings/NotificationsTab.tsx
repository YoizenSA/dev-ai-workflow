import { useState, useEffect } from "react";

// ── Component ──────────────────────────────────────────────────────────────

export function NotificationsTab() {
  const [supported, setSupported] = useState(true);
  const [subscribed, setSubscribed] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");

  useEffect(() => {
    if (!("serviceWorker" in navigator) || !("PushManager" in window)) {
      setSupported(false);
    } else {
      checkSubscription();
    }
  }, []);

  async function checkSubscription() {
    try {
      const reg = await navigator.serviceWorker.ready;
      const sub = await reg.pushManager.getSubscription();
      setSubscribed(!!sub);
    } catch {
      setSubscribed(false);
    }
  }

  async function subscribe() {
    setLoading(true);
    setError("");
    try {
      const res = await fetch("/api/push/vapid-key");
      if (!res.ok) throw new Error("Failed to get VAPID key");
      const data = (await res.json()) as { publicKey: string };

      const reg = await navigator.serviceWorker.ready;
      const sub = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(data.publicKey) as unknown as BufferSource,
      });

      const subRes = await fetch("/api/push/subscribe", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(sub.toJSON()),
      });
      if (!subRes.ok) throw new Error("Failed to save subscription");

      setSubscribed(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Subscription failed");
    } finally {
      setLoading(false);
    }
  }

  async function unsubscribe() {
    setLoading(true);
    setError("");
    try {
      const reg = await navigator.serviceWorker.ready;
      const sub = await reg.pushManager.getSubscription();
      if (sub) {
        const endpoint = sub.endpoint;
        await sub.unsubscribe();
        await fetch("/api/push/subscribe?endpoint=" + encodeURIComponent(endpoint), {
          method: "DELETE",
        });
      }
      setSubscribed(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unsubscription failed");
    } finally {
      setLoading(false);
    }
  }

  async function sendTest() {
    setError("");
    setInfo("");
    try {
      const res = await fetch("/api/push/test", { method: "POST" });
      const data = (await res.json().catch(() => ({}))) as {
        status?: string;
        sent?: number;
      };
      if (!res.ok) throw new Error("Test notification failed");
      if (data.status === "no-subscribers") {
        setInfo("No active subscription on this server. Re-enable notifications.");
      } else {
        setInfo(`Test sent to ${data.sent ?? 0} subscription(s).`);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Test failed");
    }
  }

  if (!supported) {
    return (
      <div className="settings-section">
        <h3>Push Notifications</h3>
        <p className="text-muted">
          Your browser does not support push notifications.
        </p>
      </div>
    );
  }

  return (
    <div className="settings-section">
      <h3>Push Notifications</h3>
      <p className="text-muted">
        Get notified when delegations complete or fail.
      </p>

      {error && <p className="error">{error}</p>}
      {info && <p className="text-muted">{info}</p>}

      <div className="button-group">
        {!subscribed ? (
          <button
            className="btn btn-primary"
            onClick={subscribe}
            disabled={loading}
          >
            {loading ? "Enabling..." : "Enable Push Notifications"}
          </button>
        ) : (
          <>
            <button
              className="btn btn-secondary"
              onClick={sendTest}
              disabled={loading}
            >
              Send Test Notification
            </button>
            <button
              className="btn btn-danger"
              onClick={unsubscribe}
              disabled={loading}
            >
              Disable Notifications
            </button>
          </>
        )}
      </div>
    </div>
  );
}

// ── Helper ─────────────────────────────────────────────────────────────────

function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding)
    .replace(/-/g, "+")
    .replace(/_/g, "/");
  const rawData = window.atob(base64);
  return Uint8Array.from(rawData.split("").map((c) => c.charCodeAt(0)));
}
