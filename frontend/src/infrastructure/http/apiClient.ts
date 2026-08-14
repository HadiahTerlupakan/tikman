import axios from 'axios';
import { env } from '@/shared/config/env';
import { mapApiError } from './errorMapper';

export const apiClient = axios.create({
  baseURL: env.apiUrl,
  withCredentials: true, // Important: send cookies
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor
apiClient.interceptors.request.use(
  (config) => {
    // Add correlation ID for logging
    config.headers['X-Request-ID'] = crypto.randomUUID();
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor
apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    const mappedError = mapApiError(error);

    // Auto logout on 401 will be handled by auth store
    // (imported dynamically to avoid circular dependency)
    if (error.response?.status === 401) {
      // Store handles logout
    }

    return Promise.reject(mappedError);
  }
);
