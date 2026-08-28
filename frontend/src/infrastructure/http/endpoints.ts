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
  OLT_UNCONFIGURED_ONUS: (id: string) => `/api/v1/olts/${id}/unconfigured-onus`,
  OLT_VLANS: (id: string) => `/api/v1/olts/${id}/vlans`,
  OLT_TCONT_PROFILES: (id: string) => `/api/v1/olts/${id}/tcont-profiles`,
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

  // Provisioning
  ONT_PROVISION: (id: string) => `/api/v1/onts/${id}/provision`,
  ONT_PROVISION_JOBS: (id: string) => `/api/v1/onts/${id}/provision-jobs`,
  PROVISION_JOB_BY_ID: (id: string) => `/api/v1/provision-jobs/${id}`,
  BATCH_PROVISION: "/api/v1/batch-provision",
  BATCH_JOB_BY_ID: (id: string) => `/api/v1/batch-jobs/${id}`,
  ZTE_GPON_REGISTER: (id: string) => `/api/v1/olts/${id}/gpon/register`,
  ZTE_GPON_CONFIGURE: (id: string) => `/api/v1/onts/${id}/gpon/configure`,

  // Config Templates
  CONFIG_TEMPLATES: "/api/v1/config-templates",
  CONFIG_TEMPLATE_BY_ID: (id: string) => `/api/v1/config-templates/${id}`,
} as const;
