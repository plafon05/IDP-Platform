import { Bell, CheckCheck, ChevronRight, X } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import {
  getNotifications,
  markAllNotificationsRead,
  markNotificationRead,
  type InAppNotification,
} from '../shared/api/notifications';

type Props = { onNavigate: (path: string) => void };

const kindOptions = [
  ['', 'Все события'],
  ['idp_status_changed', 'ИПР'],
  ['task_changed', 'Задачи'],
  ['task_manager_review', 'Оценки'],
  ['comment_created', 'Комментарии'],
  ['task_deadline', 'Сроки'],
] as const;

export function NotificationCenter({ onNavigate }: Props) {
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<InAppNotification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [unreadOnly, setUnreadOnly] = useState(false);
  const [kind, setKind] = useState('');
  const [sort, setSort] = useState<'newest' | 'oldest'>('newest');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const rootRef = useRef<HTMLDivElement>(null);

  async function load(silent = false) {
    if (!silent) setLoading(true);
    try {
      const result = await getNotifications({
        kind: kind || undefined, unread: unreadOnly || undefined, sort,
        date_from: dateFrom || undefined, date_to: dateTo || undefined,
      });
      setItems(result.data);
      setUnreadCount(result.unread_count);
      setError('');
    } catch {
      setError('Не удалось загрузить уведомления');
    } finally {
      if (!silent) setLoading(false);
    }
  }

  useEffect(() => {
    void load(true);
    const timer = window.setInterval(() => void load(true), 30_000);
    return () => window.clearInterval(timer);
  }, [kind, unreadOnly, sort, dateFrom, dateTo]);

  useEffect(() => {
    if (open) void load();
  }, [open, kind, unreadOnly, sort, dateFrom, dateTo]);

  useEffect(() => {
    function close(event: MouseEvent) {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    }
    function closeOnEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('mousedown', close);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, []);

  async function openNotification(item: InAppNotification) {
    if (!item.read_at) {
      await markNotificationRead(item.id);
      setUnreadCount((value) => Math.max(0, value - 1));
      setItems((current) => current.map((entry) => entry.id === item.id ? { ...entry, read_at: new Date().toISOString() } : entry));
    }
    setOpen(false);
    if (item.action_url) onNavigate(item.action_url);
  }

  async function markAll() {
    await markAllNotificationsRead();
    setUnreadCount(0);
    if (unreadOnly) setItems([]);
    else setItems((current) => current.map((item) => ({ ...item, read_at: item.read_at ?? new Date().toISOString() })));
  }

  return (
    <div className="notification-center" ref={rootRef}>
      <button className="icon-button" onClick={() => setOpen((value) => !value)} type="button" aria-label="Уведомления" title="Уведомления" aria-expanded={open}>
        <Bell size={20} />
        {unreadCount > 0 && <span className="notification-badge">{unreadCount > 99 ? '99+' : unreadCount}</span>}
      </button>

      {open && (
        <section className="notification-popover" aria-label="Центр уведомлений">
          <header className="notification-header">
            <div><h2>Уведомления</h2><span>{unreadCount ? `Непрочитанных: ${unreadCount}` : 'Всё прочитано'}</span></div>
            <button className="bare-icon-button" onClick={() => setOpen(false)} type="button" aria-label="Закрыть"><X size={19} /></button>
          </header>

          <div className="notification-controls">
            <div className="segmented-control" aria-label="Состояние уведомлений">
              <button className={!unreadOnly ? 'active' : ''} onClick={() => setUnreadOnly(false)} type="button">Все</button>
              <button className={unreadOnly ? 'active' : ''} onClick={() => setUnreadOnly(true)} type="button">Непрочитанные</button>
            </div>
            <select value={kind} onChange={(event) => setKind(event.target.value)} aria-label="Тип уведомлений">
              {kindOptions.map(([value, label]) => <option value={value} key={value}>{label}</option>)}
            </select>
            <select value={sort} onChange={(event) => setSort(event.target.value as 'newest' | 'oldest')} aria-label="Сортировка уведомлений">
              <option value="newest">Сначала новые</option>
              <option value="oldest">Сначала старые</option>
            </select>
            <label className="notification-date"><span>С даты</span><input type="date" max={dateTo || undefined} value={dateFrom} onChange={(event) => setDateFrom(event.target.value)} /></label>
            <label className="notification-date"><span>По дату</span><input type="date" min={dateFrom || undefined} value={dateTo} onChange={(event) => setDateTo(event.target.value)} /></label>
          </div>

          <div className="notification-list">
            {loading && <div className="notification-empty">Загрузка...</div>}
            {!loading && error && <div className="notification-empty error-text">{error}</div>}
            {!loading && !error && items.length === 0 && <div className="notification-empty">Уведомлений пока нет</div>}
            {!loading && !error && items.map((item) => (
              <button className={`notification-item ${item.read_at ? '' : 'unread'}`} onClick={() => void openNotification(item)} type="button" key={item.id}>
                <span className="notification-unread-dot" aria-hidden="true" />
                <span className="notification-copy">
                  <strong>{item.title}</strong>
                  <span>{item.message}</span>
                  <time dateTime={item.created_at}>{formatDate(item.created_at)}</time>
                </span>
                {item.action_url && <ChevronRight size={17} aria-hidden="true" />}
              </button>
            ))}
          </div>

          {unreadCount > 0 && <button className="notification-read-all" onClick={() => void markAll()} type="button"><CheckCheck size={17} />Прочитать все</button>}
        </section>
      )}
    </div>
  );
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }).format(new Date(value));
}
