// ywai service worker — push notifications
self.addEventListener("push", (event) => {
  if (!event.data) return;

  try {
    const data = event.data.json();
    const title = data.title || "ywai";
    const body = data.body || "";
    const options = {
      body,
      icon: "/icon.svg",
      badge: "/icon-negro.svg",
      vibrate: [200, 100, 200],
    };
    event.waitUntil(self.registration.showNotification(title, options));
  } catch {
    // ponytail: ignore malformed payloads
  }
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  // Open the ywai UI when the notification is clicked
  const urlToOpen = new URL("/", self.location.origin);
  event.waitUntil(
    clients
      .matchAll({ type: "window", includeUncontrolled: true })
      .then((windowClients) => {
        for (const client of windowClients) {
          if (client.url === urlToOpen.href && "focus" in client) {
            return client.focus();
          }
        }
        if (clients.openWindow) {
          return clients.openWindow(urlToOpen.href);
        }
      }),
  );
});
