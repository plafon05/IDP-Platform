import { RefreshCw, Search, UserMinus, UserPlus } from 'lucide-react';
import type { Dispatch, SetStateAction } from 'react';
import { FormEvent, useEffect, useMemo, useState } from 'react';
import {
  createUser,
  deactivateUser,
  listUsers,
  type CreateUserPayload,
  type User,
  type UserRole,
} from '../shared/api/users';

const roleLabels: Record<UserRole, string> = {
  employee: 'Сотрудник',
  manager: 'Руководитель',
  hr_admin: 'HR',
};

const emptyForm = {
  email: '',
  password: '',
  first_name: '',
  last_name: '',
  middle_name: '',
  position: '',
  manager: false,
  hr_admin: false,
};

type UserForm = typeof emptyForm;

export function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [query, setQuery] = useState('');
  const [form, setForm] = useState<UserForm>(emptyForm);
  const [status, setStatus] = useState<'idle' | 'loading' | 'saving'>('loading');
  const [error, setError] = useState<string | null>(null);

  const activeCount = useMemo(() => users.filter((user) => user.is_active).length, [users]);

  async function loadUsers(search = query) {
    setStatus('loading');
    setError(null);
    try {
      const result = await listUsers(search);
      setUsers(result.data);
    } catch {
      setError('Не удалось загрузить пользователей');
    } finally {
      setStatus('idle');
    }
  }

  useEffect(() => {
    void loadUsers('');
  }, []);

  async function handleSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await loadUsers(query);
  }

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setStatus('saving');
    setError(null);
    try {
      await createUser(toPayload(form));
      setForm(emptyForm);
      await loadUsers(query);
    } catch {
      setError('Не удалось создать пользователя');
    } finally {
      setStatus('idle');
    }
  }

  async function handleDeactivate(userID: string) {
    setStatus('saving');
    setError(null);
    try {
      await deactivateUser(userID);
      await loadUsers(query);
    } catch {
      setError('Не удалось деактивировать пользователя');
      setStatus('idle');
    }
  }

  return (
    <div className="users-page">
      <section className="section-header">
        <div>
          <span>Администрирование</span>
          <h2>Пользователи</h2>
        </div>
        <div className="summary-strip" aria-label="Статистика пользователей">
          <strong>{users.length}</strong>
          <span>Всего</span>
          <strong>{activeCount}</strong>
          <span>Активны</span>
        </div>
      </section>

      <section className="users-layout">
        <div className="panel">
          <div className="panel-header">
            <div>
              <h2>Список</h2>
              <p>HR управляет доступом сотрудников к платформе</p>
            </div>
            <button className="icon-button" onClick={() => void loadUsers(query)} type="button" aria-label="Обновить">
              <RefreshCw size={18} />
            </button>
          </div>

          <form className="table-toolbar" onSubmit={handleSearch}>
            <label className="search-field table-search">
              <Search size={18} aria-hidden="true" />
              <input
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Поиск по имени или email"
                value={query}
              />
            </label>
            <button className="secondary-button" type="submit">
              Найти
            </button>
          </form>

          {error && <div className="form-error">{error}</div>}

          <div className="users-table" aria-busy={status === 'loading'}>
            {users.map((user) => (
              <article className="user-row" key={user.id}>
                <div className="person">
                  <div className="person-avatar">{initials(user)}</div>
                  <div>
                    <strong>
                      {user.last_name} {user.first_name}
                    </strong>
                    <span>{user.email}</span>
                  </div>
                </div>
                <span>{user.position}</span>
                <div className="role-list">
                  {user.roles.map((role) => (
                    <span className="role-chip" key={role}>
                      {roleLabels[role]}
                    </span>
                  ))}
                </div>
                <span className={`status-pill ${user.is_active ? 'online' : 'offline'}`}>
                  {user.is_active ? 'Активен' : 'Отключен'}
                </span>
                <button
                  className="icon-button danger"
                  disabled={!user.is_active || status === 'saving'}
                  onClick={() => void handleDeactivate(user.id)}
                  type="button"
                  aria-label="Деактивировать"
                >
                  <UserMinus size={18} />
                </button>
              </article>
            ))}
          </div>
        </div>

        <form className="panel user-form" onSubmit={handleCreate}>
          <div className="panel-header">
            <div>
              <h2>Новый пользователь</h2>
              <p>Сотрудник получит роль employee автоматически</p>
            </div>
            <UserPlus size={20} aria-hidden="true" />
          </div>

          <label className="form-field">
            <span>Email</span>
            <input
              onChange={(event) => setFormValue(setForm, 'email', event.target.value)}
              required
              type="email"
              value={form.email}
            />
          </label>
          <label className="form-field">
            <span>Пароль</span>
            <input
              onChange={(event) => setFormValue(setForm, 'password', event.target.value)}
              required
              type="password"
              value={form.password}
            />
          </label>
          <div className="form-grid">
            <label className="form-field">
              <span>Имя</span>
              <input
                onChange={(event) => setFormValue(setForm, 'first_name', event.target.value)}
                required
                value={form.first_name}
              />
            </label>
            <label className="form-field">
              <span>Фамилия</span>
              <input
                onChange={(event) => setFormValue(setForm, 'last_name', event.target.value)}
                required
                value={form.last_name}
              />
            </label>
          </div>
          <label className="form-field">
            <span>Отчество</span>
            <input
              onChange={(event) => setFormValue(setForm, 'middle_name', event.target.value)}
              value={form.middle_name}
            />
          </label>
          <label className="form-field">
            <span>Должность</span>
            <input
              onChange={(event) => setFormValue(setForm, 'position', event.target.value)}
              required
              value={form.position}
            />
          </label>

          <div className="checkbox-list">
            <label>
              <input
                checked={form.manager}
                onChange={(event) => setFormValue(setForm, 'manager', event.target.checked)}
                type="checkbox"
              />
              Руководитель
            </label>
            <label>
              <input
                checked={form.hr_admin}
                onChange={(event) => setFormValue(setForm, 'hr_admin', event.target.checked)}
                type="checkbox"
              />
              HR
            </label>
          </div>

          <button className="primary-button" disabled={status === 'saving'} type="submit">
            <UserPlus size={18} />
            Создать
          </button>
        </form>
      </section>
    </div>
  );
}

function toPayload(form: UserForm): CreateUserPayload {
  const roles: UserRole[] = ['employee'];
  if (form.manager) {
    roles.push('manager');
  }
  if (form.hr_admin) {
    roles.push('hr_admin');
  }

  return {
    email: form.email.trim(),
    password: form.password,
    first_name: form.first_name.trim(),
    last_name: form.last_name.trim(),
    middle_name: form.middle_name.trim() || undefined,
    position: form.position.trim(),
    roles,
  };
}

function setFormValue<T extends keyof UserForm>(
  setForm: Dispatch<SetStateAction<UserForm>>,
  key: T,
  value: UserForm[T],
) {
  setForm((current) => ({ ...current, [key]: value }));
}

function initials(user: User) {
  return `${user.first_name[0] ?? ''}${user.last_name[0] ?? ''}`.toUpperCase();
}
