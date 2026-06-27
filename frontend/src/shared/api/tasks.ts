import { api } from './client';

export type TaskStatus = 'not_started' | 'in_progress' | 'completed' | 'cancelled';
export type TaskPriority = 'low' | 'medium' | 'high';
export type TaskRating = 'met' | 'partially_met' | 'not_met';

export type TaskReference = { id: string; name: string };
export type TaskResource = { id?: string; title: string; url: string };

export type IDPTask = {
  id: string;
  idp_id: string;
  title: string;
  description?: string;
  category?: TaskReference;
  priority: TaskPriority;
  due_date?: string;
  status: TaskStatus;
  progress: number;
  manager_rating?: TaskRating;
  manager_comment?: string;
  self_rating?: TaskRating;
  self_comment?: string;
  competencies: TaskReference[];
  tags: TaskReference[];
  resources: TaskResource[];
  created_at: string;
  updated_at: string;
};

export type TaskPayload = {
  title: string;
  description?: string;
  category_id?: string;
  priority: TaskPriority;
  due_date?: string;
  status: TaskStatus;
  progress: number;
  manager_rating?: TaskRating;
  manager_comment?: string;
  competency_ids: string[];
  tag_ids: string[];
  resources: TaskResource[];
};

export type TaskProgressPayload = {
  status: TaskStatus;
  progress: number;
  self_rating?: TaskRating;
  self_comment?: string;
};

export type TaskFilters = {
  status?: TaskStatus;
  priority?: TaskPriority;
  competencyId?: string;
  sort?: 'due_date' | 'priority' | 'status';
  order?: 'asc' | 'desc';
};

export async function listTasks(idpID: string, filters: TaskFilters = {}) {
  const response = await api.get<IDPTask[]>(`/api/v1/idps/${idpID}/tasks`, { params: filters });
  return response.data;
}

export async function createTask(idpID: string, payload: TaskPayload) {
  const response = await api.post<IDPTask>(`/api/v1/idps/${idpID}/tasks`, payload);
  return response.data;
}

export async function updateTask(id: string, payload: TaskPayload) {
  const response = await api.put<IDPTask>(`/api/v1/tasks/${id}`, payload);
  return response.data;
}

export async function updateTaskProgress(id: string, payload: TaskProgressPayload) {
  const response = await api.patch<IDPTask>(`/api/v1/tasks/${id}/progress`, payload);
  return response.data;
}

export async function deleteTask(id: string) {
  await api.delete(`/api/v1/tasks/${id}`);
}
