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
  OLT_VLAN_PROFILES: (id: string) => `/api/v1/olts/${id}/vlan-profiles`,
  OLT_ONU_TYPES: (id: string) => `/api/v1/olts/${id}/onu-types`,
  OLT_SYSTEM: (id: string) => `/api/v1/olts/${id}/system`,
  OLT_TRAFFIC: (id: string) => `/api/v1/olts/${id}/metrics/traffic`,
  OLT_SYSTEM_REFRESH: (id: string) => `/api/v1/olts/${id}/system/refresh`,
  TEST_OLT_CONNECTION: "/api/v1/olts/test-connection",

  // ONTs
  ONTS: "/api/v1/onts",
  ONT_BY_ID: (id: string) => `/api/v1/onts/${id}`,
  ONT_LATEST_METRICS: (id: string) => `/api/v1/onts/${id}/metrics`,
  ONT_METRICS_HISTORY: (id: string) => `/api/v1/onts/${id}/metrics/history`,
  ONT_REALTIME_METRICS: (id: string) => `/api/v1/onts/${id}/metrics/realtime`,
  ONT_TIMESERIES: (id: string) => `/api/v1/onts/${id}/metrics/timeseries`,
  ONT_REMOVAL_PREVIEW: (id: string) => `/api/v1/onts/${id}/removal/preview`,
  ONT_EVENTS: (id: string) => `/api/v1/onts/${id}/events`,
  ONT_SERVICE_CONFIG: (id: string) => `/api/v1/onts/${id}/service-config`,
  ONT_AVAILABILITY: (id: string) => `/api/v1/onts/${id}/availability`,

  // Provisioning
  ONT_PROVISION: (id: string) => `/api/v1/onts/${id}/provision`,
  ONT_PROVISION_JOBS: (id: string) => `/api/v1/onts/${id}/provision-jobs`,
  PROVISION_JOB_BY_ID: (id: string) => `/api/v1/provision-jobs/${id}`,
  ZTE_GPON_REGISTER: (id: string) => `/api/v1/olts/${id}/gpon/register`,
  ZTE_GPON_CONFIGURE: (id: string) => `/api/v1/onts/${id}/gpon/configure`,
  ZTE_GPON_PREVIEW_REGISTER: (id: string) => `/api/v1/olts/${id}/gpon/preview`,
  ZTE_GPON_PREVIEW_CONFIGURE: (id: string) => `/api/v1/onts/${id}/gpon/preview`,

  // Config Templates
  CONFIG_TEMPLATES: "/api/v1/config-templates",
  CONFIG_TEMPLATE_BY_ID: (id: string) => `/api/v1/config-templates/${id}`,
} as const;
