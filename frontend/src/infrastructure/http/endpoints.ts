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

  // Settings
  SETTINGS: "/api/v1/settings",
  SETTINGS_BROWSER: "/api/v1/settings/browser",
  SETTING_BY_NAME: (name: string) => `/api/v1/settings/${name}`,

  // OLTs
  OLTS: "/api/v1/olts",
  OLT_BY_ID: (id: string) => `/api/v1/olts/${id}`,
  OLT_STATS: (id: string) => `/api/v1/olts/${id}/stats`,
  OLT_UNCONFIGURED_ONUS: (id: string) => `/api/v1/olts/${id}/unconfigured-onus`,
  OLT_VLANS: (id: string) => `/api/v1/olts/${id}/vlans`,
  OLT_TCONT_PROFILES: (id: string) => `/api/v1/olts/${id}/tcont-profiles`,
  OLT_VLAN_PROFILES: (id: string) => `/api/v1/olts/${id}/vlan-profiles`,
  OLT_ONU_TYPES: (id: string) => `/api/v1/olts/${id}/onu-types`,
  OLT_DISCOVER_NOW: (id: string) => `/api/v1/olts/${id}/discover-now`,
  OLT_PON_HEALTH: (id: string) => `/api/v1/olts/${id}/pon-health`,
  ONTS_TROUBLED: "/api/v1/onts/troubled",
  OLT_SYSTEM: (id: string) => `/api/v1/olts/${id}/system`,
  OLT_TRAFFIC: (id: string) => `/api/v1/olts/${id}/metrics/traffic`,
  OLT_SYSTEM_REFRESH: (id: string) => `/api/v1/olts/${id}/system/refresh`,
  TEST_OLT_CONNECTION: "/api/v1/olts/test-connection",

  // ONTs
  ONTS: "/api/v1/onts",
  DASHBOARD_STATS: "/api/v1/dashboard/stats",
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

  // WireGuard VPN
  WIREGUARD_SERVER: "/api/v1/wireguard/server",
  WIREGUARD_PEERS: "/api/v1/wireguard/peers",
  WIREGUARD_PEER_BY_ID: (id: string) => `/api/v1/wireguard/peers/${id}`,
  WIREGUARD_PEER_CONFIG: (id: string, format: string) =>
    `/api/v1/wireguard/peers/${id}/config?format=${format}`,
  WIREGUARD_PEER_TEST: (id: string) => `/api/v1/wireguard/peers/${id}/test`,
  WIREGUARD_SUGGESTED_SUBNETS: (siteId: string) =>
    `/api/v1/wireguard/sites/${siteId}/suggested-subnets`,

  // Fibre plant: cabinets, distribution boxes, and where a drop lands
  ODCS: "/api/v1/odcs",
  ODC_FEEDS: (id: string) => `/api/v1/odcs/${id}/feeds`,
  ODPS: "/api/v1/odps",
  ODP_SUBSCRIBERS: (id: string) => `/api/v1/odps/${id}/subscribers`,
  ONT_ODP: (id: string) => `/api/v1/onts/${id}/odp`,
  ODC_FEED_LIST: "/api/v1/odc-feeds",
  ODC_FEED_ROUTE: (id: string) => `/api/v1/odc-feeds/${id}/route`,
  ODP_ROUTE: (id: string) => `/api/v1/odps/${id}/route`,

  // Push notifications
  PUSH_SUBSCRIBE: "/api/v1/push/subscribe",

  // CS inbox
  CS_STREAM: "/api/v1/cs/stream",
  CS_ONLINE: "/api/v1/cs/online",
  CS_CONVERSATIONS: "/api/v1/cs/conversations",
  CS_MESSAGES: (id: string) => `/api/v1/cs/conversations/${id}/messages`,
  CS_MEDIA_UPLOAD: (id: string) => `/api/v1/cs/conversations/${id}/media`,
  CS_TYPING: (id: string) => `/api/v1/cs/conversations/${id}/typing`,
  CS_ASSIGN: (id: string) => `/api/v1/cs/conversations/${id}/assign`,
  CS_STATUS: (id: string) => `/api/v1/cs/conversations/${id}/status`,
  CS_LINK_ONT: (id: string) => `/api/v1/cs/conversations/${id}/ont`,
  CS_MEDIA: (messageId: string) => `/api/v1/cs/media/${messageId}`,
  CS_AVATAR: (id: string) => `/api/v1/cs/conversations/${id}/avatar`,
  CS_QUICK_REPLIES: "/api/v1/cs/quick-replies",
  CS_QUICK_REPLY_BY_ID: (id: string) => `/api/v1/cs/quick-replies/${id}`,
  CS_WA_ACCOUNTS: "/api/v1/cs/wa-accounts",
  CS_WA_CONNECT: (id: string) => `/api/v1/cs/wa-accounts/${id}/connect`,
  CS_WA_DISCONNECT: (id: string) => `/api/v1/cs/wa-accounts/${id}/disconnect`,
  CS_WA_ACCOUNT_BY_ID: (id: string) => `/api/v1/cs/wa-accounts/${id}`,
  CS_WA_ACCOUNT_MESSAGES: (id: string) =>
    `/api/v1/cs/wa-accounts/${id}/messages`,
  CS_MESSAGE_BY_ID: (id: string) => `/api/v1/cs/messages/${id}`,
  CS_ALL_MESSAGES: "/api/v1/cs/messages",
  CS_WA_CHANNELS: "/api/v1/cs/wa-channels",
  CS_WA_CHANNELS_REFRESH: "/api/v1/cs/wa-channels/refresh",
  CS_BROADCASTS: "/api/v1/cs/broadcasts",
  CS_BROADCASTS_MEDIA: "/api/v1/cs/broadcasts/media",
} as const;
