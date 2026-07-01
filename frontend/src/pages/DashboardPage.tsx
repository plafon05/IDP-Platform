import { AlertTriangle, CalendarDays, CheckCircle2, Clock3, ExternalLink, Target, UsersRound } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts';
import { useSessionStore } from '../entities/session/model';
import { getDashboard, type Dashboard, type DashboardTask } from '../shared/api/dashboard';
import { MetricCard } from '../shared/ui/MetricCard';

const priorityLabels = { low: 'Низкий', medium: 'Средний', high: 'Высокий' };
const actionLabels: Record<string, string> = {
  'idp.created': 'Создан план развития', 'idp.updated': 'Изменён план развития',
  'idp.status_changed': 'Изменён статус плана', 'task.created': 'Добавлена задача',
  'task.updated': 'Изменена задача', 'task.progress_changed': 'Обновлён прогресс задачи',
  'task.deleted': 'Удалена задача', 'comment.created': 'Добавлен комментарий',
};
const chartColors: Record<string, string> = { not_started: '#94a3b8', in_progress: '#2563eb', completed: '#16a34a', cancelled: '#dc2626' };
const statusLabels: Record<string, string> = { not_started: 'Не начаты', in_progress: 'В работе', completed: 'Завершены', cancelled: 'Отменены' };

export function DashboardPage() {
  const user = useSessionStore((state) => state.user);
  const [data, setData] = useState<Dashboard | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    getDashboard().then(setData).catch(() => setError('Не удалось загрузить дашборд')).finally(() => setLoading(false));
  }, []);

  const management = data?.management;
  const averageOwnProgress = useMemo(() => data?.active_plans.length ? Math.round(data.active_plans.reduce((sum, plan) => sum + plan.progress, 0) / data.active_plans.length) : 0, [data]);
  const chartData = useMemo(() => Object.entries(management?.task_statuses ?? {}).map(([status, value]) => ({ status, name: statusLabels[status] ?? status, value })).filter((item) => item.value > 0), [management]);

  return <div className="dashboard" aria-busy={loading}>
    <section className="welcome-band"><div><span>Рабочая область</span><h2>{user ? `${user.first_name} ${user.last_name}` : 'Пользователь'}</h2></div><strong>{user?.position}</strong></section>
    {error && <div className="form-error">{error}</div>}

    <section className="metrics-grid" aria-label="Ключевые метрики">
      <MetricCard icon={Target} label={management ? 'Активные ИПР команды' : 'Мои активные ИПР'} value={String(management?.active_plans ?? data?.active_plans.length ?? 0)} tone="blue" />
      <MetricCard icon={CheckCircle2} label="Средний прогресс" value={`${management?.average_progress ?? averageOwnProgress}%`} tone="green" />
      <MetricCard icon={AlertTriangle} label={management ? 'Требуют внимания' : 'Просроченные задачи'} value={String(management?.attention.length ?? data?.overdue_tasks.length ?? 0)} tone="orange" />
      <MetricCard icon={UsersRound} label={management ? 'Сотрудники команды' : 'Ближайшие задачи'} value={String(management?.team.length ?? data?.upcoming_tasks.length ?? 0)} tone="violet" />
    </section>

    {management && <ManagerDashboard management={management} chartData={chartData} />}
    <EmployeeDashboard data={data} loading={loading} />
  </div>;
}

function ManagerDashboard({ management, chartData }: { management: NonNullable<Dashboard['management']>; chartData: { status: string; name: string; value: number }[] }) {
  return <section className="dashboard-section"><div className="section-title"><span>Управление</span><h2>Команда</h2></div><div className="content-grid">
    <div className="panel panel-wide"><div className="panel-header"><div><h2>Прямые подчинённые</h2><p>Прогресс активных планов</p></div></div><div className="team-list">{management.team.length === 0 && <div className="empty-state">Сотрудники не назначены</div>}{management.team.map((member) => <div className="team-row" key={`${member.id}-${member.plan_id ?? 'none'}`}><div className="person"><div className={`person-avatar ${member.avatar_url ? 'image' : ''}`}>{member.avatar_url ? <img alt="" src={member.avatar_url} /> : initials(member.name)}</div><div><strong>{member.name}</strong><span>{member.position}</span></div></div><div className="progress-block"><div className="progress-meta"><span>{member.plan_id ? 'Активный ИПР' : 'Нет активного ИПР'}</span><strong>{member.progress}%</strong></div><div className="progress-track"><span style={{ width: `${member.progress}%` }} /></div></div><a className="icon-button" href={`/employees/${member.id}`} aria-label={`Профиль ${member.name}`} title="Профиль сотрудника"><ExternalLink size={16} /></a></div>)}</div></div>
    <div className="panel"><div className="panel-header"><div><h2>Статусы задач</h2><p>По активным ИПР</p></div></div>{chartData.length ? <><div className="dashboard-chart"><ResponsiveContainer width="100%" height="100%"><PieChart><Pie data={chartData} dataKey="value" nameKey="name" innerRadius={48} outerRadius={72} paddingAngle={2}>{chartData.map((item) => <Cell fill={chartColors[item.status]} key={item.status} />)}</Pie><Tooltip /></PieChart></ResponsiveContainer></div><div className="chart-legend">{chartData.map((item) => <span key={item.status}><i style={{ background: chartColors[item.status] }} />{item.name}: {item.value}</span>)}</div></> : <div className="empty-state">Задач пока нет</div>}</div>
    <div className="panel"><div className="panel-header"><div><h2>Требуют внимания</h2><p>Просрочки и отсутствие активности</p></div><AlertTriangle size={20} /></div><div className="attention-list">{management.attention.length === 0 && <div className="empty-state">Проблем не обнаружено</div>}{management.attention.map((item) => <div className="attention-row" key={item.employee_id}><div><strong>{item.employee_name}</strong><span>{item.reason}</span></div><b>{item.count > 0 ? item.count : '14+ дн.'}</b></div>)}</div></div>
    <div className="panel"><div className="panel-header"><div><h2>Завершения в течение 30 дней</h2><p>Ближайшие контрольные точки</p></div><CalendarDays size={20} /></div><PlanList plans={management.upcoming_endings} /></div>
  </div></section>;
}

function EmployeeDashboard({ data, loading }: { data: Dashboard | null; loading: boolean }) {
  return <section className="dashboard-section"><div className="section-title"><span>Личное развитие</span><h2>Мои планы и задачи</h2></div><div className="content-grid">
    <div className="panel panel-wide"><div className="panel-header"><div><h2>Активные ИПР</h2><p>Текущий прогресс и сроки</p></div><Target size={20} /></div>{!loading && <PlanList plans={data?.active_plans ?? []} />}</div>
    <div className="panel"><div className="panel-header"><div><h2>Ближайшие задачи</h2><p>Дедлайн в течение 7 дней</p></div><CalendarDays size={20} /></div><TaskList tasks={data?.upcoming_tasks ?? []} /></div>
    <div className="panel"><div className="panel-header"><div><h2>Просроченные задачи</h2><p>Требуют обновления</p></div><AlertTriangle size={20} /></div><TaskList tasks={data?.overdue_tasks ?? []} overdue /></div>
    <div className="panel"><div className="panel-header"><div><h2>Мои компетенции</h2><p>Текущий и целевой уровни</p></div></div><div className="competency-dashboard">{(data?.competencies ?? []).length === 0 && <div className="empty-state">Компетенции не назначены</div>}{data?.competencies.map((item) => <div key={item.id}><div><strong>{item.name}</strong><span>{item.current_level ?? 0} / {item.target_level}</span></div><div className="level-track"><span style={{ width: `${((item.current_level ?? 0) / 4) * 100}%` }} /><i style={{ left: `${(item.target_level / 4) * 100}%` }} /></div></div>)}</div></div>
    <div className="panel"><div className="panel-header"><div><h2>Последняя активность</h2><p>10 последних событий</p></div><Clock3 size={20} /></div><div className="activity-list">{(data?.activities ?? []).length === 0 && <div className="empty-state">Событий пока нет</div>}{data?.activities.map((item, index) => <div className="activity-row" key={`${item.created_at}-${index}`}><span /><div><strong>{actionLabels[item.action] ?? item.action}</strong><small>{item.actor_name || 'Система'} · {formatDateTime(item.created_at)}</small></div></div>)}</div></div>
  </div></section>;
}

function PlanList({ plans }: { plans: Dashboard['active_plans'] }) { return <div className="dashboard-plan-list">{plans.length === 0 && <div className="empty-state">Активных ИПР нет</div>}{plans.map((plan) => <div className="dashboard-plan" key={plan.id}><div><strong>{plan.title}</strong><span>До {formatDate(plan.end_date)}</span></div><div className="progress-block"><div className="progress-meta"><span>{deadlineLabel(plan.end_date)}</span><strong>{plan.progress}%</strong></div><div className="progress-track"><span style={{ width: `${plan.progress}%` }} /></div></div></div>)}</div>; }
function TaskList({ tasks, overdue = false }: { tasks: DashboardTask[]; overdue?: boolean }) { return <div className="task-list">{tasks.length === 0 && <div className="empty-state">Задач нет</div>}{tasks.map((task) => <div className="task-row" key={task.id}><div><strong>{task.title}</strong><span className={overdue ? 'overdue-text' : ''}><Clock3 size={14} />{formatDate(task.due_date)} · {task.progress}%</span></div><span className={`priority priority-${task.priority}`}>{priorityLabels[task.priority]}</span></div>)}</div>; }
function initials(name: string) { return name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]).join('').toUpperCase(); }
function formatDate(value: string) { return new Intl.DateTimeFormat('ru-RU').format(new Date(`${value}T00:00:00`)); }
function formatDateTime(value: string) { return new Intl.DateTimeFormat('ru-RU', { dateStyle: 'short', timeStyle: 'short' }).format(new Date(value)); }
function deadlineLabel(value: string) { const days = Math.ceil((new Date(`${value}T00:00:00`).getTime() - Date.now()) / 86400000); return days < 0 ? 'Срок прошёл' : days <= 7 ? `${days} дн. до срока` : 'В работе'; }
