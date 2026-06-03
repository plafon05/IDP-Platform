import { Bell, BookOpenCheck, ChartNoAxesCombined, LayoutDashboard, Search, Settings, Users } from 'lucide-react';
import { DashboardPage } from '../pages/DashboardPage';

const navItems = [
  { icon: LayoutDashboard, label: 'Дашборд', active: true },
  { icon: BookOpenCheck, label: 'Мои ИПР' },
  { icon: Users, label: 'Команда' },
  { icon: ChartNoAxesCombined, label: 'Аналитика' },
  { icon: Settings, label: 'Настройки' },
];

export function App() {
  return (
    <div className="shell">
      <aside className="sidebar" aria-label="Основная навигация">
        <div className="brand">
          <div className="brand-mark">IDP</div>
          <div>
            <strong>Platform</strong>
            <span>Development plans</span>
          </div>
        </div>

        <nav className="nav-list">
          {navItems.map((item) => (
            <button className={`nav-item ${item.active ? 'active' : ''}`} key={item.label} type="button">
              <item.icon size={18} aria-hidden="true" />
              <span>{item.label}</span>
            </button>
          ))}
        </nav>
      </aside>

      <div className="workspace">
        <header className="topbar">
          <div>
            <div className="breadcrumbs">Главная / Дашборд</div>
            <h1>Индивидуальные планы развития</h1>
          </div>

          <div className="topbar-actions">
            <label className="search-field">
              <Search size={18} aria-hidden="true" />
              <input placeholder="Поиск сотрудников и ИПР" />
            </label>
            <button className="icon-button" type="button" aria-label="Уведомления">
              <Bell size={20} />
              <span className="notification-dot" />
            </button>
            <button className="avatar-button" type="button" aria-label="Профиль пользователя">
              АИ
            </button>
          </div>
        </header>

        <main>
          <DashboardPage />
        </main>
      </div>
    </div>
  );
}
