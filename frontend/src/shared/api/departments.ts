import { api } from './client';

export type Department = { id: string; name: string; parent_id?: string; depth: number; employees: Array<{ id: string; name: string; position: string }>; children: Department[] };
export type DepartmentInput = { name: string; parent_id?: string };

export async function listDepartments() { return (await api.get<Department[]>('/api/v1/departments')).data; }
export async function createDepartment(input: DepartmentInput) { return (await api.post<Department>('/api/v1/departments', input)).data; }
export async function updateDepartment(id: string, input: DepartmentInput) { await api.put(`/api/v1/departments/${id}`, input); }
export async function deleteDepartment(id: string) { await api.delete(`/api/v1/departments/${id}`); }
