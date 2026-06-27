import { useEffect, useState } from 'react';
import { listAudit, type AuditEntry } from '../shared/api/audit';

const actionLabels: Record<string, string> = {
  'idp.created': 'Создан ИПР',
  'idp.updated': 'Изменён ИПР',
  'idp.status_changed': 'Изменён статус ИПР',
  'idp.archived': 'ИПР перемещён в архив',
  'task.created': 'Создана задача',
  'task.updated': 'Изменена задача',
  'task.progress_changed': 'Обновлён прогресс задачи',
  'task.deleted': 'Удалена задача',
};

export function AuditTrail({ entityType, entityID, title = 'История изменений' }: { entityType: 'idp' | 'task'; entityID: string; title?: string }) {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError(null);
    void listAudit(entityType, entityID)
      .then((result) => { if (active) setEntries(result); })
      .catch(() => { if (active) setError('Не удалось загрузить историю'); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [entityType, entityID]);

  return <section className="audit-trail">
    <div className="comments-heading"><strong>{title}</strong><span>{entries.length}</span></div>
    {error && <div className="form-error">{error}</div>}
    {!loading && entries.length === 0 && <span className="task-empty">Событий пока нет</span>}
    <div className="audit-list" aria-busy={loading}>
      {[...entries].reverse().map((entry) => <div className="audit-entry" key={entry.id}>
        <span className="audit-marker" aria-hidden="true" />
        <div>
          <strong>{actionLabels[entry.action] ?? entry.action}</strong>
          <span>{summary(entry)}</span>
          <small>{entry.actor_name || 'Система'} · {formatDateTime(entry.created_at)}</small>
        </div>
      </div>)}
    </div>
  </section>;
}

function summary(entry: AuditEntry) {
  const value = entry.new_value ?? {};
  const status = textValue(value, 'status', 'Status');
  const progress = numberValue(value, 'progress', 'Progress');
  const title = textValue(value, 'title', 'Title');
  if (status && progress !== undefined) return `Статус: ${statusLabel(status)}, прогресс: ${progress}%`;
  if (status) return `Статус: ${statusLabel(status)}`;
  if (progress !== undefined) return `Прогресс: ${progress}%`;
  if (title) return title;
  return 'Данные объекта обновлены';
}

function textValue(value: Record<string, unknown>, ...keys: string[]) { for (const key of keys) if (typeof value[key] === 'string') return value[key] as string; return undefined; }
function numberValue(value: Record<string, unknown>, ...keys: string[]) { for (const key of keys) if (typeof value[key] === 'number') return value[key] as number; return undefined; }
function statusLabel(value: string) { return ({ draft: 'Черновик', active: 'Активен', completed: 'Завершён', cancelled: 'Отменён', not_started: 'Не начата', in_progress: 'В работе' } as Record<string, string>)[value] ?? value; }
function formatDateTime(value: string) { return new Intl.DateTimeFormat('ru-RU', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)); }
