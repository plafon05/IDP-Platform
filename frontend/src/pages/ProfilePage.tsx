import { KeyRound, Save, UserRound } from 'lucide-react';
import { FormEvent, useEffect, useState } from 'react';
import { useSessionStore } from '../entities/session/model';
import { changePassword, updateProfile } from '../shared/api/auth';

const emptyPasswordForm = {
  current_password: '',
  new_password: '',
};

export function ProfilePage() {
  const user = useSessionStore((state) => state.user);
  const setUser = useSessionStore((state) => state.setUser);
  const [profile, setProfile] = useState({
    first_name: '',
    last_name: '',
    middle_name: '',
    position: '',
  });
  const [passwords, setPasswords] = useState(emptyPasswordForm);
  const [profileStatus, setProfileStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const [passwordStatus, setPasswordStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!user) {
      return;
    }

    setProfile({
      first_name: user.first_name,
      last_name: user.last_name,
      middle_name: user.middle_name ?? '',
      position: user.position,
    });
  }, [user]);

  async function handleProfileSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setProfileStatus('saving');
    setError(null);

    try {
      const updatedUser = await updateProfile({
        first_name: profile.first_name.trim(),
        last_name: profile.last_name.trim(),
        middle_name: profile.middle_name.trim() || undefined,
        position: profile.position.trim(),
      });
      setUser(updatedUser);
      setProfileStatus('saved');
    } catch {
      setError('Не удалось сохранить профиль');
      setProfileStatus('idle');
    }
  }

  async function handlePasswordSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPasswordStatus('saving');
    setError(null);

    try {
      await changePassword(passwords);
      setPasswords(emptyPasswordForm);
      setPasswordStatus('saved');
    } catch {
      setError('Не удалось сменить пароль');
      setPasswordStatus('idle');
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
          <div className="person-avatar">{user ? initials(user.first_name, user.last_name) : 'ID'}</div>
          <div>
            <strong>{user ? `${user.first_name} ${user.last_name}` : 'Пользователь'}</strong>
            <span>{user?.email}</span>
          </div>
        </div>
      </section>

      {error && <div className="form-error">{error}</div>}

      <section className="profile-layout">
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
            <input
              onChange={(event) => setProfile((current) => ({ ...current, position: event.target.value }))}
              required
              value={profile.position}
            />
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
      </section>
    </div>
  );
}

function initials(firstName: string, lastName: string) {
  return `${firstName[0] ?? ''}${lastName[0] ?? ''}`.toUpperCase();
}
