import { RefreshCw, Search, Upload, UserCheck, UserMinus, UserPlus } from 'lucide-react';
import type { Dispatch, SetStateAction } from 'react';
import { FormEvent, useEffect, useMemo, useState } from 'react';
import {
  activateUser,
  createUser,
  deactivateUser,
  importUsersCSV,
  listUsers,
  type CreateUserPayload,
  type ImportUsersResult,
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
  manager_id: '',
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
  const [notice, setNotice] = useState<string | null>(null);
  const [importResult, setImportResult] = useState<ImportUsersResult | null>(null);

  const activeCount = useMemo(() => users.filter((user) => user.is_active).length, [users]);
  const managerOptions = useMemo(
    () => users.filter((user) => user.is_active && user.roles.includes('manager')),
    [users],
  );

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
    setNotice(null);
    try {
      await createUser(toPayload(form));
      setForm(emptyForm);
      await loadUsers(query);
      setNotice('Пользователь создан');
    } catch {
      setError('Не удалось создать пользователя');
    } finally {
      setStatus('idle');
    }
  }

  async function handleDeactivate(userID: string) {
    const user = users.find((item) => item.id === userID);
    const name = user ? `${user.last_name} ${user.first_name}` : 'пользователя';
    if (!window.confirm(`Деактивировать ${name}? Пользователь не сможет войти, но история сохранится.`)) {
      return;
    }

    setStatus('saving');
    setError(null);
    setNotice(null);
    try {
      await deactivateUser(userID);
      await loadUsers(query);
      setNotice('Пользователь деактивирован');
    } catch {
      setError('Не удалось деактивировать пользователя');
      setStatus('idle');
    }
  }

  async function handleActivate(userID: string) {
    setStatus('saving');
    setError(null);
    setNotice(null);
    try {
      await activateUser(userID);
      await loadUsers(query);
      setNotice('Пользователь восстановлен');
    } catch {
      setError('Не удалось восстановить пользователя');
      setStatus('idle');
    }
  }

  async function handleImport(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formData = new FormData(event.currentTarget);
    const file = formData.get('file');
    if (!(file instanceof File) || file.size === 0) {
      setError('Выберите CSV-файл');
      return;
    }

    setStatus('saving');
    setError(null);
    setNotice(null);
    setImportResult(null);
    try {
      const result = await importUsersCSV(file);
      setImportResult(result);
      event.currentTarget.reset();
      try {
        const usersResult = await listUsers(query);
        setUsers(usersResult.data);
      } catch {
        setNotice('CSV импортирован, но список не обновился автоматически');
        return;
      }
      setNotice('CSV импортирован');
    } catch {
      setError('Не удалось импортировать CSV');
    } finally {
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
          {notice && <div className="form-success">{notice}</div>}

          <div className="users-table" aria-busy={status === 'loading'}>
            {users.map((user) => (
              <article className="user-row" key={user.id}>
                <div className="person">
                  {user.avatar_url ? (
                    <img className="person-avatar image" src={user.avatar_url} alt="" />
                  ) : (
                    <div className="person-avatar">{initials(user)}</div>
                  )}
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
                {user.is_active ? (
                  <button
                    className="icon-button danger"
                    disabled={status === 'saving'}
                    onClick={() => void handleDeactivate(user.id)}
                    title="Деактивировать"
                    type="button"
                    aria-label="Деактивировать"
                  >
                    <UserMinus size={18} />
                  </button>
                ) : (
                  <button
                    className="icon-button success"
                    disabled={status === 'saving'}
                    onClick={() => void handleActivate(user.id)}
                    title="Восстановить"
                    type="button"
                    aria-label="Восстановить"
                  >
                    <UserCheck size={18} />
                  </button>
                )}
              </article>
            ))}
          </div>
        </div>

        <div className="side-stack">
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
            <label className="form-field">
              <span>Непосредственный руководитель</span>
              <select
                onChange={(event) => setFormValue(setForm, 'manager_id', event.target.value)}
                value={form.manager_id}
              >
                <option value="">Не назначен</option>
                {managerOptions.map((manager) => (
                  <option key={manager.id} value={manager.id}>
                    {manager.last_name} {manager.first_name}
                  </option>
                ))}
              </select>
            </label>

            <div className="checkbox-list">
              <label>
                <input
                  checked={form.manager}
                  onChange={(event) => setFormValue(setForm, 'manager', event.target.checked)}
                  type="checkbox"
                />
                Роль: руководитель
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

          <form className="panel import-form" onSubmit={handleImport}>
            <div className="panel-header">
              <div>
                <h2>Импорт CSV</h2>
                <p>Колонки: email, password, first_name, last_name, middle_name, position, roles</p>
              </div>
              <Upload size={20} aria-hidden="true" />
            </div>

            <label className="form-field">
              <span>CSV-файл</span>
              <input accept=".csv,text/csv" name="file" required type="file" />
            </label>

            {importResult && (
              <div className="import-result">
                <strong>
                  Создано: {importResult.created}, ошибок: {importResult.failed}
                </strong>
                {importResult.errors.slice(0, 4).map((item) => (
                  <span key={`${item.row}-${item.email ?? item.message}`}>
                    Строка {item.row}: {item.email ? `${item.email} - ` : ''}
                    {item.message}
                  </span>
                ))}
              </div>
            )}

            <button className="secondary-button" disabled={status === 'saving'} type="submit">
              <Upload size={18} />
              Импортировать
            </button>
          </form>
        </div>
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
    manager_id: form.manager_id || undefined,
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
