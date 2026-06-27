import {
  Bell,
  BookOpenCheck,
  ChartNoAxesCombined,
  LayoutDashboard,
  LibraryBig,
  LogOut,
  Search,
  Settings,
  Users,
} from 'lucide-react';
import { lazy, Suspense, useEffect, useMemo, useState } from 'react';
import { useSessionStore } from '../entities/session/model';
import { CatalogPage } from '../pages/CatalogPage';
import { IDPsPage } from '../pages/IDPsPage';
import { LoginPage } from '../pages/LoginPage';
import { ProfilePage } from '../pages/ProfilePage';
import { ResetPasswordPage } from '../pages/ResetPasswordPage';
import { UsersPage } from '../pages/UsersPage';

const DashboardPage = lazy(() => import('../pages/DashboardPage').then((module) => ({ default: module.DashboardPage })));

type Section = 'dashboard' | 'users' | 'catalog' | 'plans' | 'profile';
type NavItem = {
  id: Exclude<Section, 'profile'> | 'analytics' | 'settings';
  icon: typeof LayoutDashboard;
  label: string;
  disabled?: boolean;
};

export function App() {
  const status = useSessionStore((state) => state.status);
  const user = useSessionStore((state) => state.user);
  const bootstrap = useSessionStore((state) => state.bootstrap);
  const logout = useSessionStore((state) => state.logout);
  const [section, setSection] = useState<Section>('dashboard');

  const navItems = useMemo<NavItem[]>(() => {
    const items: NavItem[] = [
      { id: 'dashboard' as const, icon: LayoutDashboard, label: 'Дашборд' },
      { id: 'plans' as const, icon: BookOpenCheck, label: 'Мои ИПР' },
      { id: 'analytics' as const, icon: ChartNoAxesCombined, label: 'Аналитика', disabled: true },
      { id: 'settings' as const, icon: Settings, label: 'Настройки', disabled: true },
    ];

    if (user?.roles.includes('hr_admin')) {
      items.splice(1, 0, { id: 'users' as const, icon: Users, label: 'Пользователи' });
      items.splice(2, 0, { id: 'catalog' as const, icon: LibraryBig, label: 'Справочники' });
    }

    return items;
  }, [user?.roles]);

  useEffect(() => {
    void bootstrap();
  }, [bootstrap]);

  if (window.location.pathname === '/reset-password') {
    return <ResetPasswordPage />;
  }

  if (status === 'checking') {
    return (
      <main className="loading-screen">
        <div className="loading-mark">IDP</div>
      </main>
    );
  }

  if (status === 'anonymous') {
    return <LoginPage />;
  }

  const initials = user ? `${user.first_name[0] ?? ''}${user.last_name[0] ?? ''}` : 'ID';
  const pageTitle = {
    dashboard: 'Индивидуальные планы развития',
    users: 'Управление пользователями',
    catalog: 'Справочники развития',
    plans: 'Индивидуальные планы развития',
    profile: 'Профиль пользователя',
  }[section];
  const breadcrumb = {
    dashboard: 'Главная / Дашборд',
    users: 'Главная / Пользователи',
    catalog: 'Главная / Справочники',
    plans: 'Главная / ИПР',
    profile: 'Главная / Профиль',
  }[section];

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
            <button
              className={`nav-item ${section === item.id ? 'active' : ''}`}
              disabled={item.disabled}
              key={item.label}
              onClick={() => {
                if (
                  item.id === 'dashboard' ||
                  item.id === 'users' ||
                  item.id === 'catalog' ||
                  item.id === 'plans'
                ) {
                  setSection(item.id);
                }
              }}
              type="button"
            >
              <item.icon size={18} aria-hidden="true" />
              <span>{item.label}</span>
            </button>
          ))}
        </nav>
      </aside>

      <div className="workspace">
        <header className="topbar">
          <div>
            <div className="breadcrumbs">{breadcrumb}</div>
            <h1>{pageTitle}</h1>
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
            <button className="icon-button" onClick={() => void logout()} type="button" aria-label="Выйти">
              <LogOut size={20} />
            </button>
            <button
              className="avatar-button"
              onClick={() => setSection('profile')}
              type="button"
              aria-label="Профиль пользователя"
            >
              {user?.avatar_url ? <img src={user.avatar_url} alt="" /> : initials}
            </button>
          </div>
        </header>

        <main>
          {section === 'profile' && <ProfilePage />}
          {section === 'users' && <UsersPage />}
          {section === 'catalog' && <CatalogPage />}
          {section === 'plans' && <IDPsPage />}
          {section === 'dashboard' && <Suspense fallback={<div className="empty-state">Загрузка дашборда...</div>}><DashboardPage /></Suspense>}
        </main>
      </div>
    </div>
  );
}
