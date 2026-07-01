import { api } from './client';

export type TemplateTask = {
  id?: string;
  title: string;
  description?: string;
  category_id?: string;
  priority: 'low' | 'medium' | 'high';
  due_offset_days?: number;
};

export type TemplateCompetency = { competency_id: string; name?: string; target_level: number };
export type IDPTemplate = {
  id: string;
  creator_id: string;
  title: string;
  description?: string;
  goals?: string;
  target_role?: string;
  is_active: boolean;
  tasks: TemplateTask[];
  competencies: TemplateCompetency[];
};
export type TemplatePayload = Omit<IDPTemplate, 'id' | 'creator_id'>;
export type ApplyTemplatePayload = { employee_id: string; title: string; start_date: string; end_date: string };

export async function listTemplates() { return (await api.get<IDPTemplate[]>('/api/v1/idp-templates')).data; }
export async function createTemplate(payload: TemplatePayload) { return (await api.post<IDPTemplate>('/api/v1/idp-templates', payload)).data; }
export async function updateTemplate(id: string, payload: TemplatePayload) { return (await api.put<IDPTemplate>(`/api/v1/idp-templates/${id}`, payload)).data; }
export async function archiveTemplate(id: string) { await api.delete(`/api/v1/idp-templates/${id}`); }
export async function applyTemplate(id: string, payload: ApplyTemplatePayload) { return (await api.post<{ id: string }>(`/api/v1/idp-templates/${id}/apply`, payload)).data; }
