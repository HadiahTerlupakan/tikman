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
  TEST_OLT_CONNECTION: "/api/v1/olts/test-connection",
} as const;
