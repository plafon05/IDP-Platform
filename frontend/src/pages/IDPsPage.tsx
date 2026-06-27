import { Archive, CheckCircle2, ChevronDown, ChevronUp, Edit3, Play, Plus, RefreshCw, Save, X, XCircle } from 'lucide-react';
import { FormEvent, useEffect, useMemo, useState } from 'react';
import { useSessionStore } from '../entities/session/model';
import { listCompetencies, type Competency } from '../shared/api/catalog';
import {
  archiveIDP,
  changeIDPStatus,
  createIDP,
  getIDP,
  listIDPs,
  updateIDP,
  type IDP,
  type IDPCompetency,
  type IDPStatus,
} from '../shared/api/idps';
import { listSubordinates, listUsers, type User } from '../shared/api/users';
import { IDPTasksPanel } from './IDPTasksPanel';

const statusLabels: Record<IDPStatus, string> = {
  draft: 'Черновик',
  active: 'Активен',
  completed: 'Завершён',
  cancelled: 'Отменён',
};

const emptyForm = {
  employee_id: '',
  title: '',
  goals: '',
  start_date: '',
  end_date: '',
  competencies: [] as IDPCompetency[],
};

export function IDPsPage() {
  const currentUser = useSessionStore((state) => state.user);
  const isHR = currentUser?.roles.includes('hr_admin') ?? false;
  const isManager = currentUser?.roles.includes('manager') ?? false;
  type Scope = 'own' | 'team' | 'all';
  const [scope, setScope] = useState<Scope>(isHR ? 'all' : isManager ? 'team' : 'own');
  const [plans, setPlans] = useState<IDP[]>([]);
  const [employees, setEmployees] = useState<User[]>([]);
  const [competencies, setCompetencies] = useState<Competency[]>([]);
  const [form, setForm] = useState(emptyForm);
  const [editingID, setEditingID] = useState<string | null>(null);
  const [expandedPlan, setExpandedPlan] = useState<IDP | null>(null);
  const [status, setStatus] = useState<'loading' | 'idle' | 'saving'>('loading');
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const activeCount = useMemo(() => plans.filter((plan) => plan.status === 'active').length, [plans]);
  const canCreateInScope = isHR ? scope === 'all' : isManager && scope === 'team';

  async function load() {
    setStatus('loading');
    setError(null);
    try {
      const filters = scope === 'own'
        ? { employeeId: currentUser?.id }
        : scope === 'team'
          ? { managerId: currentUser?.id }
          : {};
      const plansResult = await listIDPs(filters);
      setPlans(plansResult);

      if (canCreateInScope) {
        const [employeesResult, competenciesResult] = await Promise.all([
          isHR ? listUsers().then((result) => result.data.filter((user) => user.is_active && user.manager_id)) : listSubordinates(),
          listCompetencies(false),
        ]);
        setEmployees(employeesResult);
        setCompetencies(competenciesResult);
      }
    } catch {
      setError('Не удалось загрузить ИПР');
    } finally {
      setStatus('idle');
    }
  }

  useEffect(() => {
    setExpandedPlan(null);
    resetForm();
    void load();
  }, [scope]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setStatus('saving');
    setError(null);
    setNotice(null);
    try {
      const payload = {
        employee_id: form.employee_id,
        title: form.title.trim(),
        goals: form.goals.trim() || undefined,
        start_date: form.start_date,
        end_date: form.end_date,
        competencies: form.competencies,
      };
      if (editingID) {
        await updateIDP(editingID, payload);
        setNotice('ИПР обновлён');
      } else {
        await createIDP(payload);
        setNotice('ИПР создан');
      }
      resetForm();
      await load();
    } catch {
      setError('Не удалось сохранить ИПР. Проверьте период, сотрудника и компетенции.');
      setStatus('idle');
    }
  }

  async function startEdit(plan: IDP) {
    setStatus('loading');
    setError(null);
    setNotice(null);
    try {
      const detail = await getIDP(plan.id);
      setEditingID(detail.id);
      setForm({
        employee_id: detail.employee_id,
        title: detail.title,
        goals: detail.goals ?? '',
        start_date: detail.start_date,
        end_date: detail.end_date,
        competencies: detail.competencies,
      });
    } catch {
      setError('Не удалось открыть ИПР для редактирования');
    } finally {
      setStatus('idle');
    }
  }

  async function toggleTasks(plan: IDP) {
    if (expandedPlan?.id === plan.id) {
      setExpandedPlan(null);
      return;
    }
    setStatus('loading');
    setError(null);
    try {
      setExpandedPlan(await getIDP(plan.id));
    } catch {
      setError('Не удалось открыть задачи ИПР');
    } finally {
      setStatus('idle');
    }
  }

  async function handleStatus(plan: IDP, next: IDPStatus) {
    let comment: string | undefined;
    let reason: string | undefined;
    if (next === 'completed') {
      const result = window.prompt('Финальный комментарий (необязательно)');
      if (result === null) {
        return;
      }
      comment = result.trim() || undefined;
    }
    if (next === 'cancelled') {
      reason = window.prompt('Причина отмены ИПР')?.trim();
      if (!reason) {
        return;
      }
    }

    setStatus('saving');
    setError(null);
    setNotice(null);
    try {
      await changeIDPStatus(plan.id, next, comment, reason);
      await load();
      setNotice(`Статус изменён: ${statusLabels[next]}`);
    } catch {
      setError('Не удалось изменить статус ИПР');
      setStatus('idle');
    }
  }

  async function handleArchive(plan: IDP) {
    if (!window.confirm(`Архивировать ИПР "${plan.title}"?`)) {
      return;
    }
    setStatus('saving');
    setError(null);
    setNotice(null);
    try {
      await archiveIDP(plan.id);
      await load();
      setNotice('ИПР архивирован');
    } catch {
      setError('Не удалось архивировать ИПР');
      setStatus('idle');
    }
  }

  function toggleCompetency(competency: Competency, checked: boolean) {
    setForm((current) => ({
      ...current,
      competencies: checked
        ? [...current.competencies, { competency_id: competency.id, target_level: 1 }]
        : current.competencies.filter((item) => item.competency_id !== competency.id),
    }));
  }

  function setCompetencyLevel(competencyID: string, field: 'target_level' | 'current_level', value: number) {
    setForm((current) => ({
      ...current,
      competencies: current.competencies.map((item) =>
        item.competency_id === competencyID ? { ...item, [field]: value } : item,
      ),
    }));
  }

  function resetForm() {
    setEditingID(null);
    setForm(emptyForm);
  }

  return (
    <div className="idps-page">
      <section className="section-header">
        <div>
          <span>Развитие</span>
          <h2>Индивидуальные планы</h2>
        </div>
        <div className="summary-strip">
          <strong>{plans.length}</strong>
          <span>Всего</span>
          <strong>{activeCount}</strong>
          <span>Активны</span>
        </div>
      </section>

      {(isManager || isHR) && (
        <div className="segmented-control" role="tablist" aria-label="Область ИПР">
          <button className={scope === 'own' ? 'active' : ''} onClick={() => setScope('own')} role="tab" type="button">Мои ИПР</button>
          {isManager && <button className={scope === 'team' ? 'active' : ''} onClick={() => setScope('team')} role="tab" type="button">ИПР команды</button>}
          {isHR && <button className={scope === 'all' ? 'active' : ''} onClick={() => setScope('all')} role="tab" type="button">Все ИПР</button>}
        </div>
      )}

      {error && <div className="form-error">{error}</div>}
      {notice && <div className="form-success">{notice}</div>}

      <section className={`idps-layout ${canCreateInScope ? '' : 'single'}`}>
        <div className="panel">
          <div className="panel-header">
            <div>
              <h2>{scope === 'own' ? 'Мои планы развития' : scope === 'team' ? 'Планы команды' : 'Все планы развития'}</h2>
              <p>{scope === 'own' ? 'ИПР, назначенные вам' : scope === 'team' ? 'ИПР ваших прямых подчинённых' : 'ИПР сотрудников организации'}</p>
            </div>
            <button className="icon-button" onClick={() => void load()} type="button" aria-label="Обновить">
              <RefreshCw size={18} />
            </button>
          </div>

          <div className="idp-list" aria-busy={status === 'loading'}>
            {plans.length === 0 && status !== 'loading' && <div className="empty-state">ИПР пока нет</div>}
            {plans.map((plan) => {
              const canManagePlan = isHR || (isManager && plan.manager_id === currentUser?.id);
              return <article className="idp-row" key={plan.id}>
                <div className="idp-main">
                  <div>
                    <strong>{plan.title}</strong>
                    <span>{plan.employee_name}</span>
                  </div>
                  <span className={`status-pill idp-${plan.status}`}>{statusLabels[plan.status]}</span>
                </div>
                <div className="idp-meta">
                  <span>
                    {formatDate(plan.start_date)} - {formatDate(plan.end_date)}
                  </span>
                  <span>Руководитель: {plan.manager_name}</span>
                </div>
                <div className="progress-block">
                  <div className="progress-meta">
                    <span>
                      Задачи: {plan.tasks_completed}/{plan.tasks_total}
                    </span>
                    <strong>{plan.progress}%</strong>
                  </div>
                  <div className="progress-track">
                    <span style={{ width: `${plan.progress}%` }} />
                  </div>
                </div>
                {plan.cancel_reason && <p className="cancel-reason">Причина отмены: {plan.cancel_reason}</p>}
                <div className="row-actions">
                  <button
                    className="secondary-button compact"
                    onClick={() => void toggleTasks(plan)}
                    type="button"
                  >
                    {expandedPlan?.id === plan.id ? <ChevronUp size={17} /> : <ChevronDown size={17} />}
                    Задачи
                  </button>
                  {canManagePlan && (
                    <>
                    {(plan.status === 'draft' || plan.status === 'active') && (
                      <button
                        className="icon-button"
                        disabled={status === 'saving'}
                        onClick={() => void startEdit(plan)}
                        title="Редактировать"
                        type="button"
                        aria-label="Редактировать"
                      >
                        <Edit3 size={18} />
                      </button>
                    )}
                    {plan.status === 'draft' && (
                      <button
                        className="icon-button success"
                        onClick={() => void handleStatus(plan, 'active')}
                        title="Активировать"
                        type="button"
                        aria-label="Активировать"
                      >
                        <Play size={18} />
                      </button>
                    )}
                    {plan.status === 'active' && (
                      <button
                        className="icon-button success"
                        onClick={() => void handleStatus(plan, 'completed')}
                        title="Завершить"
                        type="button"
                        aria-label="Завершить"
                      >
                        <CheckCircle2 size={18} />
                      </button>
                    )}
                    {(plan.status === 'draft' || plan.status === 'active') && (
                      <button
                        className="icon-button danger"
                        onClick={() => void handleStatus(plan, 'cancelled')}
                        title="Отменить"
                        type="button"
                        aria-label="Отменить"
                      >
                        <XCircle size={18} />
                      </button>
                    )}
                    {isHR && (
                      <button
                        className="icon-button danger"
                        onClick={() => void handleArchive(plan)}
                        title="Архивировать"
                        type="button"
                        aria-label="Архивировать"
                      >
                        <Archive size={18} />
                      </button>
                    )}
                    </>
                  )}
                </div>
                {expandedPlan?.id === plan.id && (
                  <IDPTasksPanel
                    plan={expandedPlan}
                    canManage={canManagePlan}
                    isEmployee={currentUser?.id === expandedPlan.employee_id}
                    onChanged={load}
                  />
                )}
              </article>
            })}
          </div>
        </div>

        {canCreateInScope && (
          <form className="panel idp-form" onSubmit={handleSubmit}>
            <div className="panel-header">
              <div>
                <h2>{editingID ? 'Редактирование ИПР' : 'Новый ИПР'}</h2>
                <p>План создаётся для сотрудника с назначенным руководителем</p>
              </div>
              <Plus size={20} aria-hidden="true" />
            </div>

            <label className="form-field">
              <span>Сотрудник</span>
              <select
                disabled={Boolean(editingID)}
                onChange={(event) => setForm((current) => ({ ...current, employee_id: event.target.value }))}
                required
                value={form.employee_id}
              >
                <option value="">Выберите сотрудника</option>
                {employees.map((employee) => (
                  <option key={employee.id} value={employee.id}>
                    {employee.last_name} {employee.first_name} - {employee.position}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-field">
              <span>Название</span>
              <input
                maxLength={300}
                onChange={(event) => setForm((current) => ({ ...current, title: event.target.value }))}
                required
                value={form.title}
              />
            </label>
            <label className="form-field">
              <span>Цели</span>
              <textarea
                onChange={(event) => setForm((current) => ({ ...current, goals: event.target.value }))}
                value={form.goals}
              />
            </label>
            <div className="form-grid">
              <label className="form-field">
                <span>Начало</span>
                <input
                  onChange={(event) => setForm((current) => ({ ...current, start_date: event.target.value }))}
                  required
                  type="date"
                  value={form.start_date}
                />
              </label>
              <label className="form-field">
                <span>Окончание</span>
                <input
                  min={form.start_date}
                  onChange={(event) => setForm((current) => ({ ...current, end_date: event.target.value }))}
                  required
                  type="date"
                  value={form.end_date}
                />
              </label>
            </div>

            <div className="competency-picker">
              <strong>Целевые компетенции</strong>
              {competencies.map((competency) => {
                const selected = form.competencies.find((item) => item.competency_id === competency.id);
                return (
                  <div className="competency-option" key={competency.id}>
                    <label>
                      <input
                        checked={Boolean(selected)}
                        onChange={(event) => toggleCompetency(competency, event.target.checked)}
                        type="checkbox"
                      />
                      {competency.name}
                    </label>
                    {selected && (
                      <div>
                        <select
                          aria-label={`Текущий уровень: ${competency.name}`}
                          onChange={(event) =>
                            setCompetencyLevel(competency.id, 'current_level', Number(event.target.value))
                          }
                          value={selected.current_level ?? 1}
                        >
                          {[1, 2, 3, 4].map((level) => (
                            <option key={level} value={level}>
                              Текущий {level}
                            </option>
                          ))}
                        </select>
                        <select
                          aria-label={`Целевой уровень: ${competency.name}`}
                          onChange={(event) =>
                            setCompetencyLevel(competency.id, 'target_level', Number(event.target.value))
                          }
                          value={selected.target_level}
                        >
                          {[1, 2, 3, 4].map((level) => (
                            <option key={level} value={level}>
                              Цель {level}
                            </option>
                          ))}
                        </select>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>

            <div className="button-row">
              <button className="primary-button" disabled={status === 'saving'} type="submit">
                <Save size={18} />
                Сохранить
              </button>
              {editingID && (
                <button className="secondary-button" onClick={resetForm} type="button">
                  <X size={18} />
                  Отмена
                </button>
              )}
            </div>
          </form>
        )}
      </section>
    </div>
  );
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat('ru-RU').format(new Date(`${value}T00:00:00`));
}
