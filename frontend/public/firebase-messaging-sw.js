// This file carries no Firebase configuration. A service worker cannot import
// src/shared/config/firebase.ts, and duplicating the six values here would put
// the project identity in the repository and give it a second copy that can
// drift. Instead messaging.ts appends them to the registration URL, and this
// script reads them back off its own location — so there is still exactly one
// source, the build-time environment.
//
// A service worker whose URL changes is treated as a new registration, so
// rotating the Firebase project re-registers rather than leaving a worker
// bound to the old one.
const params = new URL(self.location).searchParams;

importScripts("https://www.gstatic.com/firebasejs/12.18.0/firebase-app-compat.js");
importScripts("https://www.gstatic.com/firebasejs/12.18.0/firebase-messaging-compat.js");

firebase.initializeApp({
  apiKey: params.get("apiKey") || "",
  authDomain: params.get("authDomain") || "",
  projectId: params.get("projectId") || "",
  storageBucket: params.get("storageBucket") || "",
  messagingSenderId: params.get("messagingSenderId") || "",
  appId: params.get("appId") || "",
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
