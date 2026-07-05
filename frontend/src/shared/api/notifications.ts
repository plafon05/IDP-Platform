import { api } from './client';

export type NotificationPreferences = {
  email_enabled: boolean;
  idp_updates: boolean;
  task_updates: boolean;
  comments: boolean;
  reminders: boolean;
};

export async function getNotificationPreferences() {
  const response = await api.get<NotificationPreferences>('/api/v1/notifications/preferences');
  return response.data;
}

export async function updateNotificationPreferences(value: NotificationPreferences) {
  const response = await api.put<NotificationPreferences>('/api/v1/notifications/preferences', value);
  return response.data;
}

export async function unsubscribeFromNotifications(token: string) {
  await api.post('/api/v1/notifications/unsubscribe', { token });
}

export type InAppNotification = {
  id: string;
  kind: string;
  title: string;
  message: string;
  action_url: string | null;
  read_at: string | null;
  created_at: string;
};

export type NotificationList = {
  data: InAppNotification[];
  unread_count: number;
};

export async function getNotifications(params: { kind?: string; unread?: boolean; sort?: 'newest' | 'oldest'; date_from?: string; date_to?: string } = {}) {
  const response = await api.get<NotificationList>('/api/v1/notifications', { params });
  return response.data;
}

export async function markNotificationRead(id: string) {
  await api.patch(`/api/v1/notifications/${id}/read`);
}

export async function markAllNotificationsRead() {
  await api.patch('/api/v1/notifications/read-all');
}
