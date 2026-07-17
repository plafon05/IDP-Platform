import axios from 'axios';

let accessToken: string | null = null;
let refreshPromise: Promise<string> | null = null;

export const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL ?? '',
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json',
  },
});

api.interceptors.request.use((config) => {
  if (accessToken) {
    config.headers.Authorization = `Bearer ${accessToken}`;
  }

  return config;
});

api.interceptors.response.use(undefined, async (error) => {
  const response = error.response;
  const original = error.config as (typeof error.config & { _retried?: boolean }) | undefined;
  const isAuthRequest = original?.url?.includes('/api/v1/auth/login')
    || original?.url?.includes('/api/v1/auth/refresh')
    || original?.url?.includes('/api/v1/auth/logout');
  if (response?.status !== 401 || !original || original._retried || isAuthRequest) {
    return Promise.reject(error);
  }

  original._retried = true;
  try {
    if (!refreshPromise) {
      refreshPromise = api.post<{ access_token: string }>('/api/v1/auth/refresh')
        .then((result) => {
          setAccessToken(result.data.access_token);
          return result.data.access_token;
        })
        .finally(() => {
          refreshPromise = null;
        });
    }
    const token = await refreshPromise;
    original.headers ??= {};
    original.headers.Authorization = `Bearer ${token}`;
    return api(original);
  } catch (refreshError) {
    setAccessToken(null);
    return Promise.reject(refreshError);
  }
});

export function setAccessToken(token: string | null) {
  accessToken = token;
}
