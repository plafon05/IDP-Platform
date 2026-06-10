import { AlertTriangle, CalendarDays, CheckCircle2, Clock3, Target, UsersRound } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useSessionStore } from '../entities/session/model';
import { api } from '../shared/api/client';
import { MetricCard } from '../shared/ui/MetricCard';

const team = [
  { name: 'Иван Петров', role: 'Product Manager', progress: 72, status: 'Активен' },
  { name: 'Мария Смирнова', role: 'Frontend Developer', progress: 48, status: 'Требует внимания' },
  { name: 'Олег Андреев', role: 'Backend Developer', progress: 91, status: 'В срок' },
];

const tasks = [
  { title: 'Завершить курс по фасилитации', due: '5 июн', priority: 'Высокий' },
  { title: 'Подготовить демо по проектной метрике', due: '8 июн', priority: 'Средний' },
  { title: 'Провести встречу с ментором', due: '10 июн', priority: 'Средний' },
];

export function DashboardPage() {
  const [apiStatus, setAPIStatus] = useState('checking');
  const user = useSessionStore((state) => state.user);

  useEffect(() => {
    api
      .get('/health')
      .then(() => setAPIStatus('online'))
      .catch(() => setAPIStatus('offline'));
  }, []);

  return (
    <div className="dashboard">
      <section className="welcome-band">
        <div>
          <span>Рабочая область</span>
          <h2>{user ? `${user.first_name} ${user.last_name}` : 'Пользователь'}</h2>
        </div>
        <strong>{user?.position}</strong>
      </section>

      <section className="metrics-grid" aria-label="Ключевые метрики">
        <MetricCard icon={Target} label="Активные ИПР" value="24" tone="blue" />
        <MetricCard icon={CheckCircle2} label="Средний прогресс" value="68%" tone="green" />
        <MetricCard icon={AlertTriangle} label="Просроченные задачи" value="7" tone="orange" />
        <MetricCard icon={UsersRound} label="Охват команды" value="82%" tone="violet" />
      </section>

      <section className="content-grid">
        <div className="panel panel-wide">
          <div className="panel-header">
            <div>
              <h2>Команда</h2>
              <p>Прогресс прямых подчинённых по активным ИПР</p>
            </div>
            <span className={`status-pill ${apiStatus}`}>API {apiStatus}</span>
          </div>

          <div className="team-list">
            {team.map((member) => (
              <article className="team-row" key={member.name}>
                <div className="person">
                  <div className="person-avatar">{initials(member.name)}</div>
                  <div>
                    <strong>{member.name}</strong>
                    <span>{member.role}</span>
                  </div>
                </div>
                <div className="progress-block">
                  <div className="progress-meta">
                    <span>{member.status}</span>
                    <strong>{member.progress}%</strong>
                  </div>
                  <div className="progress-track">
                    <span style={{ width: `${member.progress}%` }} />
                  </div>
                </div>
              </article>
            ))}
          </div>
        </div>

        <div className="panel">
          <div className="panel-header">
            <div>
              <h2>Ближайшие задачи</h2>
              <p>Дедлайны на 7 дней</p>
            </div>
            <CalendarDays size={20} aria-hidden="true" />
          </div>

          <div className="task-list">
            {tasks.map((task) => (
              <article className="task-row" key={task.title}>
                <div>
                  <strong>{task.title}</strong>
                  <span>
                    <Clock3 size={14} aria-hidden="true" />
                    {task.due}
                  </span>
                </div>
                <span className="priority">{task.priority}</span>
              </article>
            ))}
          </div>
        </div>
      </section>
    </div>
  );
}

function initials(name: string) {
  return name
    .split(' ')
    .map((part) => part[0])
    .join('')
    .slice(0, 2);
}
