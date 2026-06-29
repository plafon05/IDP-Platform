import { Activity, ClipboardList, Target, UsersRound } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { Bar, BarChart, CartesianGrid, Cell, Line, LineChart, Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { getAnalytics, type AnalyticsFilters, type AnalyticsResponse } from '../shared/api/analytics';
import { MetricCard } from '../shared/ui/MetricCard';

const statusLabels: Record<string, string> = { draft: 'Черновики', active: 'Активные', completed: 'Завершённые', cancelled: 'Отменённые' };
const statusColors: Record<string, string> = { draft: '#94a3b8', active: '#2563eb', completed: '#16a34a', cancelled: '#dc2626' };
const pieColors = ['#2563eb', '#16a34a', '#d97706', '#7c3aed', '#dc2626', '#0891b2'];

function initialFilters(): AnalyticsFilters {
  const to = new Date();
  const from = new Date(to);
  from.setFullYear(from.getFullYear() - 1);
  return { from: dateValue(from), to: dateValue(to) };
}

export function AnalyticsPage() {
  const [filters, setFilters] = useState<AnalyticsFilters>(initialFilters);
  const [data, setData] = useState<AnalyticsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    getAnalytics(filters).then(setData).catch(() => setError('Не удалось загрузить аналитику')).finally(() => setLoading(false));
  }, [filters.from, filters.to, filters.status]);

  const statuses = useMemo(() => (data?.statuses ?? []).map((item) => ({ ...item, label: statusLabels[item.name] ?? item.name })), [data]);
  const activity = useMemo(() => (data?.activity ?? []).map((item) => ({ ...item, label: formatWeek(item.week) })), [data]);

  return <div className="analytics-page" aria-busy={loading}>
    <section className="analytics-toolbar">
      <div><span>Отчётный период</span><h2>Аналитика развития</h2></div>
      <div className="analytics-filters">
        <label><span>С</span><input type="date" max={filters.to} value={filters.from} onChange={(event) => setFilters((current) => ({ ...current, from: event.target.value }))} /></label>
        <label><span>По</span><input type="date" min={filters.from} value={filters.to} onChange={(event) => setFilters((current) => ({ ...current, to: event.target.value }))} /></label>
        <label><span>Статус ИПР</span><select value={filters.status ?? ''} onChange={(event) => setFilters((current) => ({ ...current, status: event.target.value || undefined }))}><option value="">Все статусы</option>{Object.entries(statusLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
      </div>
    </section>
    {error && <div className="form-error">{error}</div>}
    <section className="metrics-grid" aria-label="Сводные показатели">
      <MetricCard icon={Target} label="ИПР" value={String(data?.summary.plans ?? 0)} tone="blue" />
      <MetricCard icon={UsersRound} label="Сотрудники" value={String(data?.summary.employees ?? 0)} tone="violet" />
      <MetricCard icon={ClipboardList} label="Задачи" value={String(data?.summary.tasks ?? 0)} tone="orange" />
      <MetricCard icon={Activity} label="Средний прогресс" value={`${data?.summary.average_progress ?? 0}%`} tone="green" />
    </section>
    <section className="analytics-grid">
      <ChartPanel title="ИПР по статусам" subtitle="Количество планов в выбранном периоде">
        {statuses.length ? <ResponsiveContainer width="100%" height="100%"><BarChart data={statuses}><CartesianGrid strokeDasharray="3 3" vertical={false} /><XAxis dataKey="label" tick={{ fontSize: 12 }} /><YAxis allowDecimals={false} width={32} /><Tooltip /><Bar dataKey="value" name="ИПР" radius={[4, 4, 0, 0]}>{statuses.map((item) => <Cell key={item.name} fill={statusColors[item.name] ?? '#475569'} />)}</Bar></BarChart></ResponsiveContainer> : <Empty />}
      </ChartPanel>
      <ChartPanel title="Динамика активности" subtitle="Обновления прогресса задач по неделям">
        {activity.length ? <ResponsiveContainer width="100%" height="100%"><LineChart data={activity}><CartesianGrid strokeDasharray="3 3" vertical={false} /><XAxis dataKey="label" tick={{ fontSize: 12 }} /><YAxis allowDecimals={false} width={32} /><Tooltip /><Line dataKey="value" name="Обновления" stroke="#0891b2" strokeWidth={2} dot={{ r: 3 }} /></LineChart></ResponsiveContainer> : <Empty />}
      </ChartPanel>
      <ChartPanel title="Популярные компетенции" subtitle="Топ-10 по использованию в ИПР">
        {(data?.competencies.length ?? 0) ? <ResponsiveContainer width="100%" height="100%"><BarChart data={data?.competencies} layout="vertical" margin={{ left: 16 }}><CartesianGrid strokeDasharray="3 3" horizontal={false} /><XAxis type="number" allowDecimals={false} /><YAxis dataKey="name" type="category" width={120} tick={{ fontSize: 12 }} /><Tooltip /><Bar dataKey="value" name="ИПР" fill="#7c3aed" radius={[0, 4, 4, 0]} /></BarChart></ResponsiveContainer> : <Empty />}
      </ChartPanel>
      <ChartPanel title="Категории задач" subtitle="Распределение задач по категориям">
        {(data?.categories.length ?? 0) ? <><ResponsiveContainer width="100%" height="78%"><PieChart><Pie data={data?.categories} dataKey="value" nameKey="name" innerRadius={50} outerRadius={82} paddingAngle={2}>{data?.categories.map((item, index) => <Cell key={item.name} fill={pieColors[index % pieColors.length]} />)}</Pie><Tooltip /></PieChart></ResponsiveContainer><div className="analytics-legend">{data?.categories.map((item, index) => <span key={item.name}><i style={{ background: pieColors[index % pieColors.length] }} />{item.name}: {item.value}</span>)}</div></> : <Empty />}
      </ChartPanel>
    </section>
    <section className="panel analytics-table-panel"><div className="panel-header"><div><h2>Сотрудники</h2><p>Метрики по ИПР и задачам за выбранный период</p></div></div><div className="table-scroll"><table className="analytics-table"><thead><tr><th>Сотрудник</th><th>Должность</th><th>ИПР</th><th>Задачи</th><th>Прогресс</th></tr></thead><tbody>{data?.employees.map((item) => <tr key={item.id}><td><strong>{item.name}</strong></td><td>{item.position}</td><td>{item.plans}</td><td>{item.tasks}</td><td><div className="table-progress"><span>{item.average_progress}%</span><div className="progress-track"><i style={{ width: `${item.average_progress}%` }} /></div></div></td></tr>)}</tbody></table>{!loading && data?.employees.length === 0 && <Empty />}</div></section>
  </div>;
}

function ChartPanel({ title, subtitle, children }: { title: string; subtitle: string; children: React.ReactNode }) { return <section className="panel analytics-chart-panel"><div className="panel-header"><div><h2>{title}</h2><p>{subtitle}</p></div></div><div className="analytics-chart">{children}</div></section>; }
function Empty() { return <div className="empty-state">Нет данных за выбранный период</div>; }
function dateValue(value: Date) { return value.toISOString().slice(0, 10); }
function formatWeek(value: string) { return new Intl.DateTimeFormat('ru-RU', { day: '2-digit', month: 'short' }).format(new Date(`${value}T00:00:00`)); }
