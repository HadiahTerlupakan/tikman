// Keep the six config values below identical to the firebaseConfig in
// src/shared/config/firebase.ts — a service worker cannot import that module,
// so this is a deliberate duplicate, not a second source of truth. Change one,
// change the other in the same commit. Filled in for real in the task that runs
// once the Firebase project exists (see
// docs/superpowers/specs/2026-09-03-cs-push-notifications-design.md §7).
importScripts("https://www.gstatic.com/firebasejs/12.18.0/firebase-app-compat.js");
importScripts("https://www.gstatic.com/firebasejs/12.18.0/firebase-messaging-compat.js");

firebase.initializeApp({
  apiKey: "REPLACE_WITH_FIREBASE_API_KEY",
  authDomain: "REPLACE_WITH_FIREBASE_AUTH_DOMAIN",
  projectId: "REPLACE_WITH_FIREBASE_PROJECT_ID",
  storageBucket: "REPLACE_WITH_FIREBASE_STORAGE_BUCKET",
  messagingSenderId: "REPLACE_WITH_FIREBASE_MESSAGING_SENDER_ID",
  appId: "REPLACE_WITH_FIREBASE_APP_ID",
});

const messaging = firebase.messaging();

// The backend sends data-only pushes (see internal/push/client.go): a payload
// carrying a notification block would be displayed by the SDK itself as well as
// handed here, so every push arrived twice with two different click behaviours.
messaging.onBackgroundMessage((payload) => {
  const title = payload.data?.title || "TikMan";
  const body = payload.data?.body || "Pesan baru masuk";
  self.registration.showNotification(title, {
    body,
    data: payload.data,
  });
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  event.waitUntil(
    clients.matchAll({ type: "window", includeUncontrolled: true }).then((windowClients) => {
      for (const client of windowClients) {
        if (client.url.includes("/cs") && "focus" in client) {
          return client.focus();
        }
      }
      return clients.openWindow("/cs");
    }),
  );
});
