import type { AxiosRequestConfig, AxiosResponse } from 'axios';
import { afterEach, expect, it, vi } from 'vitest';
import { api, setAccessToken } from './client';

const response = (config: AxiosRequestConfig, data: unknown): AxiosResponse => ({
  config: config as AxiosResponse['config'], data, status: 200, statusText: 'OK', headers: {},
});

afterEach(() => {
  setAccessToken(null);
  api.defaults.adapter = undefined;
});

it('refreshes a session once and retries a failed protected request', async () => {
  let protectedAttempts = 0;
  const adapter = vi.fn(async (config: AxiosRequestConfig) => {
    if (config.url === '/api/v1/auth/refresh') {
      return response(config, { access_token: 'fresh-token' });
    }
    protectedAttempts++;
    if (protectedAttempts === 1) {
      return Promise.reject({ config, response: { status: 401 } });
    }
    return response(config, { ok: true });
  });
  api.defaults.adapter = adapter;

  await expect(api.get('/api/v1/dashboard')).resolves.toMatchObject({ data: { ok: true } });
  expect(adapter).toHaveBeenCalledTimes(3);
  const retriedRequest = adapter.mock.calls.length > 2 ? adapter.mock.calls[2]?.[0] : undefined;
  expect(retriedRequest?.headers?.Authorization).toBe('Bearer fresh-token');
});
