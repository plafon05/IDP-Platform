import { KeyRound, Save, UserRound } from 'lucide-react';
import { FormEvent, useEffect, useState } from 'react';
import { useSessionStore } from '../entities/session/model';
import { changePassword, updateAvatar, updateProfile } from '../shared/api/auth';

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
  });
  const [passwords, setPasswords] = useState(emptyPasswordForm);
  const [profileStatus, setProfileStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const [passwordStatus, setPasswordStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const [avatarStatus, setAvatarStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

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

  async function handleAvatarSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formData = new FormData(event.currentTarget);
    const file = formData.get('avatar');
    if (!(file instanceof File) || file.size === 0) {
      setError('Выберите файл аватара');
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
      event.currentTarget.reset();
    } catch {
      setError('Не удалось загрузить аватар');
      setAvatarStatus('idle');
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
        <form className="panel profile-form avatar-card" onSubmit={handleAvatarSubmit}>
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

          <label className="form-field">
            <span>Файл</span>
            <input accept="image/jpeg,image/png,image/webp" name="avatar" required type="file" />
          </label>

          <button className="secondary-button" disabled={avatarStatus === 'saving'} type="submit">
            <Save size={18} />
            {avatarStatus === 'saved' ? 'Загружено' : 'Загрузить'}
          </button>
        </form>

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
      </section>
    </div>
  );
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
