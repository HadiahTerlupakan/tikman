export const env = {
  apiUrl: import.meta.env.VITE_API_URL || "http://localhost:8080",
  appName: import.meta.env.VITE_APP_NAME || "TikMan",
  isDevelopment: import.meta.env.DEV,
  isProduction: import.meta.env.PROD,
} as const;
