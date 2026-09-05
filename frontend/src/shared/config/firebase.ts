// These six values plus the VAPID key identify the Firebase project to the
// browser. They are public by Firebase's own design — they ship in the bundle
// either way — but they are kept out of this repository deliberately, so the
// project can be rotated or replaced without a code change.
//
// Vite inlines import.meta.env at BUILD time, so they must reach the build,
// not the container: frontend/Dockerfile declares a matching ARG/ENV pair for
// each, and docker-compose.yml passes them through build.args from the env
// file. Adding a value here without adding it in both of those places gives a
// bundle that silently reads "" — which is exactly how this went wrong once
// before.
//
// The service worker cannot import this module, so messaging.ts hands it the
// same values on its registration query string rather than repeating them in
// a second file that could drift.
export const firebaseConfig = {
  apiKey: import.meta.env.VITE_FIREBASE_API_KEY || "",
  authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN || "",
  projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID || "",
  storageBucket: import.meta.env.VITE_FIREBASE_STORAGE_BUCKET || "",
  messagingSenderId: import.meta.env.VITE_FIREBASE_MESSAGING_SENDER_ID || "",
  appId: import.meta.env.VITE_FIREBASE_APP_ID || "",
  databaseURL: import.meta.env.VITE_FIREBASE_DATABASE_URL || "",
} as const;

export const firebaseVapidKey = import.meta.env.VITE_FIREBASE_VAPID_KEY || "";

/** False until a real Firebase project is configured. Every function in
 * infrastructure/firebase/messaging.ts checks this first and resolves to a
 * safe "nothing happened" instead of throwing against an unconfigured app,
 * and usePushNotifications reports "unsupported" so the opt-in control is not
 * offered at all. */
export const isFirebaseConfigured = Boolean(firebaseConfig.apiKey);
