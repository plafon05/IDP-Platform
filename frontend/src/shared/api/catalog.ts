import { api } from './client';

export type CompetencyCategory = 'hard' | 'soft' | 'leadership' | 'management' | 'technical';

export type CompetencyLevel = {
  level: number;
  title: string;
  description?: string;
};

export type Competency = {
  id: string;
  name: string;
  description?: string;
  category: CompetencyCategory;
  is_active: boolean;
  levels?: CompetencyLevel[];
};

export type CompetencyPayload = {
  name: string;
  description?: string;
  category: CompetencyCategory;
  is_active: boolean;
  levels: CompetencyLevel[];
};

export type NamedCatalogItem = {
  id: string;
  name: string;
};

export async function listCompetencies(includeInactive = true) {
  const response = await api.get<Competency[]>('/api/v1/competencies', {
    params: { include_inactive: includeInactive },
  });
  return response.data;
}

export async function createCompetency(payload: CompetencyPayload) {
  const response = await api.post<Competency>('/api/v1/competencies', payload);
  return response.data;
}

export async function updateCompetency(id: string, payload: CompetencyPayload) {
  const response = await api.put<Competency>(`/api/v1/competencies/${id}`, payload);
  return response.data;
}

export async function archiveCompetency(id: string) {
  await api.delete(`/api/v1/competencies/${id}`);
}

export async function listTaskCategories() {
  const response = await api.get<NamedCatalogItem[]>('/api/v1/task-categories');
  return response.data;
}

export async function createTaskCategory(name: string) {
  const response = await api.post<NamedCatalogItem>('/api/v1/task-categories', { name });
  return response.data;
}

export async function updateTaskCategory(id: string, name: string) {
  const response = await api.put<NamedCatalogItem>(`/api/v1/task-categories/${id}`, { name });
  return response.data;
}

export async function deleteTaskCategory(id: string) {
  await api.delete(`/api/v1/task-categories/${id}`);
}

export async function listTags() {
  const response = await api.get<NamedCatalogItem[]>('/api/v1/tags');
  return response.data;
}

export async function createTag(name: string) {
  const response = await api.post<NamedCatalogItem>('/api/v1/tags', { name });
  return response.data;
}

export async function updateTag(id: string, name: string) {
  const response = await api.put<NamedCatalogItem>(`/api/v1/tags/${id}`, { name });
  return response.data;
}

export async function deleteTag(id: string) {
  await api.delete(`/api/v1/tags/${id}`);
}
