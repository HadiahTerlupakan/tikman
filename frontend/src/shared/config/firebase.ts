// These six values plus the VAPID key are public by Firebase's own design —
// they identify the project to the browser and are meant to ship in a frontend
// bundle — so they are committed literals, not build-time environment reads.
// Vite inlines import.meta.env at build time, and the production image builds
// with no .env in reach, so an env-driven config could never have reached a
// deployed bundle at all.
//
// public/firebase-messaging-sw.js repeats them, because a service worker cannot
// import this module. Change one, change the other in the same commit.
// Filled in for real by the task that runs once the Firebase project exists
// (see docs/superpowers/specs/2026-09-03-cs-push-notifications-design.md §7).
export const firebaseConfig = {
  apiKey: "REPLACE_WITH_FIREBASE_API_KEY",
  authDomain: "REPLACE_WITH_FIREBASE_AUTH_DOMAIN",
  projectId: "REPLACE_WITH_FIREBASE_PROJECT_ID",
  storageBucket: "REPLACE_WITH_FIREBASE_STORAGE_BUCKET",
  messagingSenderId: "REPLACE_WITH_FIREBASE_MESSAGING_SENDER_ID",
  appId: "REPLACE_WITH_FIREBASE_APP_ID",
} as const;

export const firebaseVapidKey = "REPLACE_WITH_FIREBASE_VAPID_KEY";

/** False until a real Firebase project is configured. Every function in
 * infrastructure/firebase/messaging.ts checks this first and resolves to a
 * safe "nothing happened" instead of throwing against an unconfigured app,
 * and usePushNotifications reports "unsupported" so the opt-in control is not
 * offered at all. */
export const isFirebaseConfigured =
  !firebaseConfig.apiKey.startsWith("REPLACE_WITH_");
