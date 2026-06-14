import { api } from './client';

export type User = {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  middle_name?: string;
  position: string;
  avatar_url?: string;
  roles: string[];
};

export type AuthResponse = {
  access_token: string;
  access_token_expires_at: string;
  user: User;
};

export type UpdateProfilePayload = {
  first_name: string;
  last_name: string;
  middle_name?: string;
};

export type ChangePasswordPayload = {
  current_password: string;
  new_password: string;
};

export type ForgotPasswordResponse = {
  status: string;
  dev_reset_token?: string;
  dev_reset_url?: string;
  expires_at?: string;
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

export async function updateProfile(payload: UpdateProfilePayload) {
  const response = await api.put<User>('/api/v1/users/me', payload);
  return response.data;
}

export async function changePassword(payload: ChangePasswordPayload) {
  await api.put('/api/v1/users/me/password', payload);
}

export async function updateAvatar(file: File) {
  const formData = new FormData();
  formData.append('avatar', file);

  const response = await api.put<User>('/api/v1/users/me/avatar', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });
  return response.data;
}

export async function forgotPassword(email: string) {
  const response = await api.post<ForgotPasswordResponse>('/api/v1/auth/forgot-password', { email });
  return response.data;
}

export async function resetPassword(token: string, newPassword: string) {
  await api.post('/api/v1/auth/reset-password', { token, new_password: newPassword });
}
