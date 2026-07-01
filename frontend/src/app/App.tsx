import {
  Bell,
  BookOpenCheck,
  ChartNoAxesCombined,
  ClipboardCopy,
  Building2,
  LayoutDashboard,
  LibraryBig,
  LogOut,
  Moon,
  Search,
  Settings,
  Sun,
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
import { UnsubscribePage } from '../pages/UnsubscribePage';
import { TemplatesPage } from '../pages/TemplatesPage';
import { DepartmentsPage } from '../pages/DepartmentsPage';

const DashboardPage = lazy(() => import('../pages/DashboardPage').then((module) => ({ default: module.DashboardPage })));
const AnalyticsPage = lazy(() => import('../pages/AnalyticsPage').then((module) => ({ default: module.AnalyticsPage })));
const EmployeeProfilePage = lazy(() => import('../pages/EmployeeProfilePage').then((module) => ({ default: module.EmployeeProfilePage })));

type Section = 'dashboard' | 'users' | 'departments' | 'catalog' | 'plans' | 'templates' | 'analytics' | 'profile' | 'employee-profile';
type NavItem = {
	id: Exclude<Section, 'profile'> | 'settings';
  icon: typeof LayoutDashboard;
  label: string;
  disabled?: boolean;
};

function sectionFromPath(): Section {
  const value = window.location.pathname.slice(1);
  if (value.startsWith('employees/')) return 'employee-profile';
  return value === 'users' || value === 'departments' || value === 'catalog' || value === 'plans' || value === 'templates' || value === 'analytics' || value === 'profile' ? value : 'dashboard';
}

export function App() {
  const status = useSessionStore((state) => state.status);
  const user = useSessionStore((state) => state.user);
  const bootstrap = useSessionStore((state) => state.bootstrap);
  const logout = useSessionStore((state) => state.logout);
  const [section, setSection] = useState<Section>(sectionFromPath);
  const [theme, setTheme] = useState<'light' | 'dark'>(() => document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light');

  function toggleTheme() {
    const next = theme === 'dark' ? 'light' : 'dark';
    document.documentElement.dataset.theme = next;
    localStorage.setItem('idp-theme', next);
    setTheme(next);
  }

  function navigate(next: Section) {
    window.history.pushState({}, '', next === 'dashboard' ? '/' : `/${next}`);
    setSection(next);
  }

  const navItems = useMemo<NavItem[]>(() => {
    const plansLabel = user?.roles.includes('hr_admin') ? 'Все ИПР' : user?.roles.includes('manager') ? 'ИПР' : 'Мои ИПР';
    const items: NavItem[] = [
      { id: 'dashboard' as const, icon: LayoutDashboard, label: 'Дашборд' },
      { id: 'plans' as const, icon: BookOpenCheck, label: plansLabel },
      { id: 'settings' as const, icon: Settings, label: 'Настройки', disabled: true },
    ];

    if (user?.roles.includes('manager') || user?.roles.includes('hr_admin')) {
      items.splice(2, 0, { id: 'analytics' as const, icon: ChartNoAxesCombined, label: 'Аналитика' });
      items.splice(2, 0, { id: 'templates' as const, icon: ClipboardCopy, label: 'Шаблоны ИПР' });
    }

    if (user?.roles.includes('hr_admin')) {
      items.splice(1, 0, { id: 'users' as const, icon: Users, label: 'Пользователи' });
      items.splice(2, 0, { id: 'departments' as const, icon: Building2, label: 'Подразделения' });
      items.splice(3, 0, { id: 'catalog' as const, icon: LibraryBig, label: 'Справочники' });
    }

    return items;
  }, [user?.roles]);

  useEffect(() => {
    void bootstrap();
  }, [bootstrap]);

  useEffect(() => {
    const handlePopState = () => setSection(sectionFromPath());
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  if (window.location.pathname === '/reset-password') {
    return <ResetPasswordPage />;
  }
  if (window.location.pathname === '/unsubscribe') {
    return <UnsubscribePage />;
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
    departments: 'Структура организации',
    catalog: 'Справочники развития',
    plans: 'Индивидуальные планы развития',
    analytics: 'Аналитика развития',
    templates: 'Шаблоны ИПР',
    profile: 'Профиль пользователя',
    'employee-profile': 'Профиль сотрудника',
  }[section];
  const breadcrumb = {
    dashboard: 'Главная / Дашборд',
    users: 'Главная / Пользователи',
    departments: 'Главная / Подразделения',
    catalog: 'Главная / Справочники',
    plans: 'Главная / ИПР',
    analytics: 'Главная / Аналитика',
    templates: 'Главная / Шаблоны ИПР',
    profile: 'Главная / Профиль',
    'employee-profile': 'Главная / Сотрудники / Профиль',
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
                  item.id === 'departments' ||
                  item.id === 'catalog' ||
                  item.id === 'plans' ||
                  item.id === 'templates' ||
                  item.id === 'analytics'
                ) {
                  navigate(item.id);
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
            <button className="icon-button" onClick={toggleTheme} type="button" aria-label={theme === 'dark' ? 'Включить светлую тему' : 'Включить тёмную тему'} title={theme === 'dark' ? 'Светлая тема' : 'Тёмная тема'}>
              {theme === 'dark' ? <Sun size={20} /> : <Moon size={20} />}
            </button>
            <button className="icon-button" onClick={() => void logout()} type="button" aria-label="Выйти">
              <LogOut size={20} />
            </button>
            <button
              className="avatar-button"
              onClick={() => navigate('profile')}
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
          {section === 'departments' && <DepartmentsPage />}
          {section === 'catalog' && <CatalogPage />}
          {section === 'plans' && <IDPsPage />}
          {section === 'templates' && <TemplatesPage />}
          {section === 'employee-profile' && <Suspense fallback={<div className="empty-state">Загрузка профиля...</div>}><EmployeeProfilePage employeeID={window.location.pathname.split('/')[2] ?? ''} /></Suspense>}
          {section === 'analytics' && <Suspense fallback={<div className="empty-state">Загрузка аналитики...</div>}><AnalyticsPage /></Suspense>}
          {section === 'dashboard' && <Suspense fallback={<div className="empty-state">Загрузка дашборда...</div>}><DashboardPage /></Suspense>}
        </main>
      </div>
    </div>
  );
}
