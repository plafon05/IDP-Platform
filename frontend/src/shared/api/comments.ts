import { api } from './client';

export type Comment = {
  id: string;
  entity_type: 'idp' | 'task';
  entity_id: string;
  author_id: string;
  author_name: string;
  author_avatar?: string;
  content: string;
  is_deleted: boolean;
  can_edit: boolean;
  can_delete: boolean;
  created_at: string;
  updated_at: string;
};

export async function listComments(entityType: 'idp' | 'task', entityID: string) {
  const response = await api.get<Comment[]>(`/api/v1/${entityType === 'idp' ? 'idps' : 'tasks'}/${entityID}/comments`);
  return response.data;
}

export async function createComment(entityType: 'idp' | 'task', entityID: string, content: string) {
  const response = await api.post<Comment>(`/api/v1/${entityType === 'idp' ? 'idps' : 'tasks'}/${entityID}/comments`, { content });
  return response.data;
}

export async function updateComment(id: string, content: string) {
  const response = await api.put<Comment>(`/api/v1/comments/${id}`, { content });
  return response.data;
}

export async function deleteComment(id: string) { await api.delete(`/api/v1/comments/${id}`); }
