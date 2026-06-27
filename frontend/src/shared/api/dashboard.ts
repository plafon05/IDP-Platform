import { api } from './client';

export type DashboardPlan = { id: string; title: string; end_date: string; progress: number };
export type DashboardTask = { id: string; idp_id: string; title: string; due_date: string; priority: 'low' | 'medium' | 'high'; status: string; progress: number };
export type DashboardCompetency = { id: string; name: string; current_level?: number; target_level: number };
export type DashboardActivity = { action: string; actor_name: string; created_at: string };
export type TeamMember = { id: string; name: string; position: string; avatar_url?: string; plan_id?: string; progress: number };
export type AttentionItem = { employee_id: string; employee_name: string; reason: string; count: number };
export type ManagementSummary = {
  team: TeamMember[];
  attention: AttentionItem[];
  upcoming_endings: DashboardPlan[];
  active_plans: number;
  average_progress: number;
  task_statuses: Record<string, number>;
};
export type Dashboard = {
  active_plans: DashboardPlan[];
  upcoming_tasks: DashboardTask[];
  overdue_tasks: DashboardTask[];
  competencies: DashboardCompetency[];
  activities: DashboardActivity[];
  management?: ManagementSummary;
};

export async function getDashboard() {
  const response = await api.get<Dashboard>('/api/v1/dashboard');
  return response.data;
}
