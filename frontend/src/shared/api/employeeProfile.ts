import { api } from './client';
import type { User } from './users';

export type EmployeeIDP = { id: string; title: string; start_date: string; end_date: string; status: string; tasks_total: number; tasks_completed: number; progress: number };
export type EmployeeProfile = {
  user: User;
  manager_name?: string;
  department_name?: string;
  idps: EmployeeIDP[];
  progress: Array<{ week: string; progress: number }>;
  competencies: Array<{ id: string; name: string; current_level: number; target_level: number }>;
};

export async function getEmployeeProfile(id: string) {
  return (await api.get<EmployeeProfile>(`/api/v1/employees/${id}/profile`)).data;
}
