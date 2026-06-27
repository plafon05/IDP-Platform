import { api } from './client';

export type IDPStatus = 'draft' | 'active' | 'completed' | 'cancelled';

export type IDPCompetency = {
  competency_id: string;
  name?: string;
  target_level: number;
  current_level?: number;
};

export type IDP = {
  id: string;
  employee_id: string;
  employee_name: string;
  manager_id: string;
  manager_name: string;
  title: string;
  goals?: string;
  start_date: string;
  end_date: string;
  status: IDPStatus;
  cancel_reason?: string;
  tasks_total: number;
  tasks_completed: number;
  progress: number;
  competencies: IDPCompetency[];
  created_at: string;
  updated_at: string;
};

export type IDPPayload = {
  employee_id: string;
  title: string;
  goals?: string;
  start_date: string;
  end_date: string;
  competencies: IDPCompetency[];
};

type IDPListResponse = {
  data: IDP[];
  meta: {
    total: number;
    page: number;
    limit: number;
    total_pages: number;
  };
};

export async function listIDPs(filters: { employeeId?: string; managerId?: string } = {}) {
  const response = await api.get<IDPListResponse>('/api/v1/idps', { params: { page: 1, limit: 50, ...filters } });
  return response.data.data;
}

export async function getIDP(id: string) {
  const response = await api.get<IDP>(`/api/v1/idps/${id}`);
  return response.data;
}

export async function createIDP(payload: IDPPayload) {
  const response = await api.post<IDP>('/api/v1/idps', payload);
  return response.data;
}

export async function updateIDP(id: string, payload: IDPPayload) {
  const response = await api.put<IDP>(`/api/v1/idps/${id}`, payload);
  return response.data;
}

export async function changeIDPStatus(id: string, status: IDPStatus, comment?: string, reason?: string) {
  const response = await api.patch<IDP>(`/api/v1/idps/${id}/status`, { status, comment, reason });
  return response.data;
}

export async function archiveIDP(id: string) {
  await api.delete(`/api/v1/idps/${id}`);
}
