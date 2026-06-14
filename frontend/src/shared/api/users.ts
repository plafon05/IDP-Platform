import { api } from './client';

export type UserRole = 'employee' | 'manager' | 'hr_admin';

export type User = {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  middle_name?: string;
  position: string;
  manager_id?: string;
  avatar_url?: string;
  is_active: boolean;
  roles: UserRole[];
};

export type UsersListResponse = {
  data: User[];
  meta: {
    total: number;
    page: number;
    limit: number;
    total_pages: number;
  };
};

export type CreateUserPayload = {
  email: string;
  password: string;
  first_name: string;
  last_name: string;
  middle_name?: string;
  position: string;
  manager_id?: string;
  roles: UserRole[];
};

export type ImportUsersResult = {
  created: number;
  failed: number;
  errors: Array<{
    row: number;
    email?: string;
    message: string;
  }>;
};

export async function listUsers(query = '') {
  const response = await api.get<UsersListResponse>('/api/v1/users', {
    params: { q: query, page: 1, limit: 50 },
  });
  return response.data;
}

export async function createUser(payload: CreateUserPayload) {
  const response = await api.post<User>('/api/v1/users', payload);
  return response.data;
}

export async function deactivateUser(userID: string) {
  await api.delete(`/api/v1/users/${userID}`);
}

export async function activateUser(userID: string) {
  const response = await api.patch<User>(`/api/v1/users/${userID}/activate`);
  return response.data;
}

export async function importUsersCSV(file: File) {
  const formData = new FormData();
  formData.append('file', file);

  const response = await api.post<ImportUsersResult>('/api/v1/users/import', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });
  return response.data;
}
