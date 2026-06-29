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
