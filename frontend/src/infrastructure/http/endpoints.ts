export const API_ENDPOINTS = {
  // Auth
  AUTH_LOGIN: "/api/v1/auth/login",
  AUTH_LOGOUT: "/api/v1/auth/logout",
  AUTH_ME: "/api/v1/auth/me",

  // Users
  USERS: "/api/v1/users",
  USER_BY_ID: (id: string) => `/api/v1/users/${id}`,

  // Sites
  SITES: "/api/v1/sites",
  SITE_BY_ID: (id: string) => `/api/v1/sites/${id}`,

  // OLTs
  OLTS: "/api/v1/olts",
  OLT_BY_ID: (id: string) => `/api/v1/olts/${id}`,
  OLT_STATS: (id: string) => `/api/v1/olts/${id}/stats`,
  TEST_OLT_CONNECTION: "/api/v1/olts/test-connection",

  // ONTs
  ONTS: "/api/v1/onts",
  ONT_BY_ID: (id: string) => `/api/v1/onts/${id}`,
  ONT_LATEST_METRICS: (id: string) => `/api/v1/onts/${id}/metrics`,
  ONT_METRICS_HISTORY: (id: string) => `/api/v1/onts/${id}/metrics/history`,
  ONT_REALTIME_METRICS: (id: string) => `/api/v1/onts/${id}/metrics/realtime`,
  ONT_TIMESERIES: (id: string) => `/api/v1/onts/${id}/metrics/timeseries`,
  ONT_EVENTS: (id: string) => `/api/v1/onts/${id}/events`,
  ONT_AVAILABILITY: (id: string) => `/api/v1/onts/${id}/availability`,
} as const;
