import { api } from './client';

export type NamedMetric = { name: string; value: number };
export type AnalyticsResponse = {
  summary: { plans: number; employees: number; tasks: number; average_progress: number };
  statuses: NamedMetric[];
  activity: Array<{ week: string; value: number }>;
  competencies: NamedMetric[];
  categories: NamedMetric[];
  employees: Array<{ id: string; name: string; position: string; plans: number; tasks: number; average_progress: number }>;
};

export type AnalyticsFilters = { from: string; to: string; status?: string };

export async function getAnalytics(filters: AnalyticsFilters) {
  const response = await api.get<AnalyticsResponse>('/api/v1/analytics/overview', { params: filters });
  return response.data;
}
