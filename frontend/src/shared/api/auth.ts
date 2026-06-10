import { api } from './client';

export type User = {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  middle_name?: string;
  position: string;
  roles: string[];
};

export type AuthResponse = {
  access_token: string;
  access_token_expires_at: string;
  user: User;
};

export async function login(email: string, password: string) {
  const response = await api.post<AuthResponse>('/api/v1/auth/login', { email, password });
  return response.data;
}

export async function refreshSession() {
  const response = await api.post<AuthResponse>('/api/v1/auth/refresh');
  return response.data;
}

export async function logout() {
  await api.post('/api/v1/auth/logout');
}

export async function getCurrentUser() {
  const response = await api.get<User>('/api/v1/users/me');
  return response.data;
}
