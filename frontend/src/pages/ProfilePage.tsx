import { Bell, KeyRound, Save, Upload, UserRound } from 'lucide-react';
import { ChangeEvent, FormEvent, useEffect, useRef, useState } from 'react';
import { useSessionStore } from '../entities/session/model';
import { changePassword, updateAvatar, updateProfile } from '../shared/api/auth';
import { getNotificationPreferences, updateNotificationPreferences, type NotificationPreferences } from '../shared/api/notifications';

const emptyPasswordForm = {
  current_password: '',
  new_password: '',
};

const defaultNotifications: NotificationPreferences = {
  email_enabled: true, idp_updates: true, task_updates: true, comments: true, reminders: true,
};

export function ProfilePage() {
  const user = useSessionStore((state) => state.user);
  const setUser = useSessionStore((state) => state.setUser);
  const [profile, setProfile] = useState({
    first_name: '',
    last_name: '',
    middle_name: '',
  });
  const [passwords, setPasswords] = useState(emptyPasswordForm);
  const [profileStatus, setProfileStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const [passwordStatus, setPasswordStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const [avatarStatus, setAvatarStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const [notifications, setNotifications] = useState(defaultNotifications);
  const [notificationStatus, setNotificationStatus] = useState<'loading' | 'idle' | 'saving' | 'saved'>('loading');
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const avatarInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!user) {
      return;
    }

    setProfile({
      first_name: user.first_name,
      last_name: user.last_name,
      middle_name: user.middle_name ?? '',
    });
  }, [user]);

  useEffect(() => {
    void getNotificationPreferences()
      .then((value) => { setNotifications(value); setNotificationStatus('idle'); })
      .catch(() => { setError('Не удалось загрузить настройки уведомлений'); setNotificationStatus('idle'); });
  }, []);

  async function saveNotifications() {
    setNotificationStatus('saving');
    setError(null);
    setNotice(null);
    try {
      setNotifications(await updateNotificationPreferences(notifications));
      setNotificationStatus('saved');
      setNotice('Настройки уведомлений сохранены');
    } catch {
      setError('Не удалось сохранить настройки уведомлений');
      setNotificationStatus('idle');
    }
  }

  async function handleProfileSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setProfileStatus('saving');
    setError(null);
    setNotice(null);

    try {
      const updatedUser = await updateProfile({
        first_name: profile.first_name.trim(),
        last_name: profile.last_name.trim(),
        middle_name: profile.middle_name.trim() || undefined,
      });
      setUser(updatedUser);
      setProfileStatus('saved');
      setNotice('Профиль сохранён');
    } catch {
      setError('Не удалось сохранить профиль');
      setProfileStatus('idle');
    }
  }

  async function handlePasswordSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPasswordStatus('saving');
    setError(null);
    setNotice(null);

    try {
      await changePassword(passwords);
      setPasswords(emptyPasswordForm);
      setPasswordStatus('saved');
      setNotice('Пароль изменён');
    } catch {
      setError('Не удалось сменить пароль');
      setPasswordStatus('idle');
    }
  }

  async function handleAvatarChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }

    setAvatarStatus('saving');
    setError(null);
    setNotice(null);
    try {
      const updatedUser = await updateAvatar(file);
      setUser(updatedUser);
      setAvatarStatus('saved');
      setNotice('Аватар загружен');
    } catch {
      setError('Не удалось загрузить аватар');
      setAvatarStatus('idle');
    } finally {
      event.target.value = '';
    }
  }

  return (
    <div className="profile-page">
      <section className="section-header">
        <div>
          <span>Аккаунт</span>
          <h2>Профиль</h2>
        </div>
        <div className="profile-identity">
          <AvatarImage user={user} />
          <div>
            <strong>{user ? `${user.first_name} ${user.last_name}` : 'Пользователь'}</strong>
            <span>{user?.email}</span>
          </div>
        </div>
      </section>

      {error && <div className="form-error">{error}</div>}
      {notice && <div className="form-success">{notice}</div>}

      <section className="profile-layout">
        <section className="panel profile-form avatar-card">
          <div className="panel-header">
            <div>
              <h2>Аватар</h2>
              <p>JPEG, PNG или WebP до 2 MB</p>
            </div>
            <UserRound size={20} aria-hidden="true" />
          </div>

          <div className="avatar-preview">
            <AvatarImage user={user} />
          </div>

          <input className="visually-hidden" accept="image/jpeg,image/png,image/webp" onChange={(event) => void handleAvatarChange(event)} ref={avatarInputRef} type="file" />
          <button className="secondary-button" disabled={avatarStatus === 'saving'} onClick={() => avatarInputRef.current?.click()} type="button">
            <Upload size={18} />
            {avatarStatus === 'saving' ? 'Загрузка...' : avatarStatus === 'saved' ? 'Загружено' : 'Загрузить'}
          </button>
        </section>

        <form className="panel profile-form" onSubmit={handleProfileSubmit}>
          <div className="panel-header">
            <div>
              <h2>Основные данные</h2>
              <p>Эти данные видят HR и руководители</p>
            </div>
            <UserRound size={20} aria-hidden="true" />
          </div>

          <div className="form-grid">
            <label className="form-field">
              <span>Имя</span>
              <input
                onChange={(event) => setProfile((current) => ({ ...current, first_name: event.target.value }))}
                required
                value={profile.first_name}
              />
            </label>
            <label className="form-field">
              <span>Фамилия</span>
              <input
                onChange={(event) => setProfile((current) => ({ ...current, last_name: event.target.value }))}
                required
                value={profile.last_name}
              />
            </label>
          </div>

          <label className="form-field">
            <span>Отчество</span>
            <input
              onChange={(event) => setProfile((current) => ({ ...current, middle_name: event.target.value }))}
              value={profile.middle_name}
            />
          </label>

          <label className="form-field">
            <span>Должность</span>
            <input readOnly value={user?.position ?? ''} />
          </label>

          <button className="primary-button" disabled={profileStatus === 'saving'} type="submit">
            <Save size={18} />
            {profileStatus === 'saved' ? 'Сохранено' : 'Сохранить'}
          </button>
        </form>

        <form className="panel profile-form" onSubmit={handlePasswordSubmit}>
          <div className="panel-header">
            <div>
              <h2>Пароль</h2>
              <p>Минимум 8 символов, заглавная буква и цифра</p>
            </div>
            <KeyRound size={20} aria-hidden="true" />
          </div>

          <label className="form-field">
            <span>Текущий пароль</span>
            <input
              autoComplete="current-password"
              onChange={(event) => setPasswords((current) => ({ ...current, current_password: event.target.value }))}
              required
              type="password"
              value={passwords.current_password}
            />
          </label>

          <label className="form-field">
            <span>Новый пароль</span>
            <input
              autoComplete="new-password"
              onChange={(event) => setPasswords((current) => ({ ...current, new_password: event.target.value }))}
              required
              type="password"
              value={passwords.new_password}
            />
          </label>

          <button className="primary-button" disabled={passwordStatus === 'saving'} type="submit">
            <KeyRound size={18} />
            {passwordStatus === 'saved' ? 'Пароль изменён' : 'Сменить пароль'}
          </button>
        </form>

        <section className="panel profile-form notification-settings" id="notifications">
          <div className="panel-header"><div><h2>Уведомления</h2><p>Управление письмами на {user?.email}</p></div><Bell size={20} aria-hidden="true" /></div>
          <NotificationToggle label="Email-уведомления" checked={notifications.email_enabled} onChange={(checked) => setNotifications((current) => ({ ...current, email_enabled: checked }))} />
          <div className="notification-options" aria-disabled={!notifications.email_enabled}>
            <NotificationToggle label="Изменения ИПР" checked={notifications.idp_updates} disabled={!notifications.email_enabled} onChange={(checked) => setNotifications((current) => ({ ...current, idp_updates: checked }))} />
            <NotificationToggle label="Назначение и оценка задач" checked={notifications.task_updates} disabled={!notifications.email_enabled} onChange={(checked) => setNotifications((current) => ({ ...current, task_updates: checked }))} />
            <NotificationToggle label="Новые комментарии" checked={notifications.comments} disabled={!notifications.email_enabled} onChange={(checked) => setNotifications((current) => ({ ...current, comments: checked }))} />
            <NotificationToggle label="Сроки и просрочки" checked={notifications.reminders} disabled={!notifications.email_enabled} onChange={(checked) => setNotifications((current) => ({ ...current, reminders: checked }))} />
          </div>
          <button className="primary-button" disabled={notificationStatus === 'loading' || notificationStatus === 'saving'} onClick={() => void saveNotifications()} type="button"><Save size={18} />{notificationStatus === 'saved' ? 'Сохранено' : 'Сохранить'}</button>
        </section>
      </section>
    </div>
  );
}

function NotificationToggle({ label, checked, disabled = false, onChange }: { label: string; checked: boolean; disabled?: boolean; onChange: (checked: boolean) => void }) {
  return <label className="notification-toggle"><span>{label}</span><input type="checkbox" checked={checked} disabled={disabled} onChange={(event) => onChange(event.target.checked)} /></label>;
}

function AvatarImage({ user }: { user: { avatar_url?: string; first_name: string; last_name: string } | null }) {
  if (user?.avatar_url) {
    return <img className="person-avatar image" src={user.avatar_url} alt="" />;
  }

  return <div className="person-avatar">{user ? initials(user.first_name, user.last_name) : 'ID'}</div>;
}

function initials(firstName: string, lastName: string) {
  return `${firstName[0] ?? ''}${lastName[0] ?? ''}`.toUpperCase();
}
