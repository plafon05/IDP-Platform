import { ArrowLeft, BookOpenCheck, Target, UserRound } from 'lucide-react';
import { useEffect, useState } from 'react';
import { CartesianGrid, Legend, Line, LineChart, PolarAngleAxis, PolarGrid, Radar, RadarChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { getEmployeeProfile, type EmployeeProfile } from '../shared/api/employeeProfile';
import { MetricCard } from '../shared/ui/MetricCard';

const statusLabels: Record<string, string> = { draft: 'Черновик', active: 'Активен', completed: 'Завершён', cancelled: 'Отменён' };

export function EmployeeProfilePage({ employeeID }: { employeeID: string }) {
  const [data, setData] = useState<EmployeeProfile | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => { getEmployeeProfile(employeeID).then(setData).catch(() => setError('Не удалось загрузить профиль сотрудника')); }, [employeeID]);
  if (error) return <div className="form-error">{error}</div>;
  if (!data) return <div className="empty-state">Загрузка профиля...</div>;
  const name = `${data.user.first_name} ${data.user.last_name}`;
  const average = data.idps.length ? Math.round(data.idps.reduce((sum, item) => sum + item.progress, 0) / data.idps.length) : 0;
  const progress = data.progress.map((item) => ({ ...item, label: formatShortDate(item.week) }));
  return <div className="employee-profile-page">
    <button
      className="secondary-button compact profile-back"
      type="button"
      onClick={() => {
        if (window.history.length > 1) window.history.back()
        else window.location.href = '/'
      }}
    >
      <ArrowLeft size={16} />Назад
    </button>
    <section className="employee-profile-header"><div className={`employee-profile-avatar ${data.user.avatar_url ? 'image' : ''}`}>{data.user.avatar_url ? <img src={data.user.avatar_url} alt="" /> : initials(data.user.first_name, data.user.last_name)}</div><div><span>{data.user.position}</span><h2>{name}</h2><p>{data.user.email}</p></div><div className="employee-profile-facts"><span>Подразделение<strong>{data.department_name ?? 'Не назначено'}</strong></span><span>Руководитель<strong>{data.manager_name ?? 'Не назначен'}</strong></span></div></section>
    <section className="metrics-grid"><MetricCard icon={BookOpenCheck} label="Всего ИПР" value={String(data.idps.length)} tone="blue" /><MetricCard icon={Target} label="Средний прогресс" value={`${average}%`} tone="green" /><MetricCard icon={UserRound} label="Компетенции" value={String(data.competencies.length)} tone="violet" /></section>
    <section className="employee-profile-grid"><div className="panel"><div className="panel-header"><div><h2>Динамика прогресса</h2><p>Среднее выполнение задач по неделям</p></div></div><div className="employee-profile-chart">{progress.length ? <ResponsiveContainer width="100%" height="100%"><LineChart data={progress}><CartesianGrid strokeDasharray="3 3" vertical={false} /><XAxis dataKey="label" tick={{ fontSize: 11 }} /><YAxis domain={[0,100]} width={34} /><Tooltip /><Line type="monotone" dataKey="progress" name="Прогресс" stroke="#2563eb" strokeWidth={2} dot={false} /></LineChart></ResponsiveContainer> : <Empty />}</div></div>
    <div className="panel"><div className="panel-header"><div><h2>Профиль компетенций</h2><p>Текущий и целевой уровни</p></div></div><div className="employee-profile-chart">{data.competencies.length ? <ResponsiveContainer width="100%" height="100%"><RadarChart data={data.competencies} outerRadius="68%"><PolarGrid /><PolarAngleAxis dataKey="name" tick={{ fontSize: 11 }} /><Radar name="Текущий" dataKey="current_level" stroke="#0891b2" fill="#0891b2" fillOpacity={0.25} /><Radar name="Целевой" dataKey="target_level" stroke="#7c3aed" fill="#7c3aed" fillOpacity={0.15} /><Legend /></RadarChart></ResponsiveContainer> : <Empty />}</div></div></section>
    <section className="panel"><div className="panel-header"><div><h2>История ИПР</h2><p>Все планы развития сотрудника</p></div></div><div className="employee-idp-history">{data.idps.length === 0 && <Empty />}{data.idps.map((item) => <a href={`/plans?id=${item.id}`} className="employee-idp-row" key={item.id}><div><strong>{item.title}</strong><span>{formatDate(item.start_date)} - {formatDate(item.end_date)}</span></div><span className={`status-pill idp-${item.status}`}>{statusLabels[item.status] ?? item.status}</span><div className="progress-block"><div className="progress-meta"><span>{item.tasks_completed}/{item.tasks_total} задач</span><strong>{item.progress}%</strong></div><div className="progress-track"><span style={{ width: `${item.progress}%` }} /></div></div></a>)}</div></section>
  </div>;
}
function Empty() { return <div className="empty-state">Нет данных</div>; }
function initials(first: string,last: string){return `${first[0]??''}${last[0]??''}`.toUpperCase();}
function formatDate(value:string){return new Intl.DateTimeFormat('ru-RU').format(new Date(`${value}T00:00:00`));}
function formatShortDate(value:string){return new Intl.DateTimeFormat('ru-RU',{day:'2-digit',month:'short'}).format(new Date(`${value}T00:00:00`));}
