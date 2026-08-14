import axios from "axios";
import { camelizeKeys, decamelizeKeys } from "humps";
import { env } from "@/shared/config/env";
import { mapApiError } from "./errorMapper";

export const apiClient = axios.create({
  baseURL: env.apiUrl,
  withCredentials: true, // Important: send cookies
  timeout: 30000,
  headers: {
    "Content-Type": "application/json",
  },
});

// Request interceptor
apiClient.interceptors.request.use(
  (config) => {
    // Add correlation ID for logging
    config.headers["X-Request-ID"] = crypto.randomUUID();

    // Transform request data from camelCase to snake_case
    if (config.data) {
      config.data = decamelizeKeys(config.data);
    }

    return config;
  },
  (error) => Promise.reject(error),
);

// Response interceptor
apiClient.interceptors.response.use(
  (response) => {
    // Transform response data from snake_case to camelCase
    if (response.data) {
      response.data = camelizeKeys(response.data);
    }
    return response;
  },
  async (error) => {
    const mappedError = mapApiError(error);

    // Auto logout on 401 will be handled by auth store
    // (imported dynamically to avoid circular dependency)
    if (error.response?.status === 401) {
      // Store handles logout
    }

    return Promise.reject(mappedError);
  },
);
