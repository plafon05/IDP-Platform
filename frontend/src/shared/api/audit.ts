import { api } from './client';

export type AuditEntry = {
  id: string;
  actor_id?: string;
  actor_name: string;
  action: string;
  old_value?: Record<string, unknown>;
  new_value?: Record<string, unknown>;
  created_at: string;
};

export async function listAudit(entityType: 'idp' | 'task', entityID: string) {
  const response = await api.get<AuditEntry[]>(`/api/v1/${entityType === 'idp' ? 'idps' : 'tasks'}/${entityID}/audit`);
  return response.data;
}
