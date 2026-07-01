import { Activity, Check, Edit3, ExternalLink, MessageSquare, Plus, RotateCcw, Save, Trash2, X } from 'lucide-react';
import { FormEvent, useEffect, useState } from 'react';
import { listTags, listTaskCategories, type NamedCatalogItem } from '../shared/api/catalog';
import { CommentsThread } from '../components/CommentsThread';
import { AuditTrail } from '../components/AuditTrail';
import type { IDP } from '../shared/api/idps';
import {
  createTask,
  deleteTask,
  listTasks,
  updateTask,
  updateTaskProgress,
  type IDPTask,
  type TaskPayload,
  type TaskPriority,
  type TaskRating,
  type TaskResource,
  type TaskStatus,
  type TaskFilters,
} from '../shared/api/tasks';

const statusLabels: Record<TaskStatus, string> = {
  not_started: 'Не начата',
  in_progress: 'В работе',
  completed: 'Завершена',
  cancelled: 'Отменена',
};

const ratingLabels: Record<TaskRating, string> = {
  met: 'Выполнено',
  partially_met: 'Частично выполнено',
  not_met: 'Не выполнено',
};

const emptyForm: TaskPayload = {
  title: '', description: '', priority: 'medium', status: 'not_started', progress: 0,
  competency_ids: [], tag_ids: [], resources: [],
};

type Props = {
  plan: IDP;
  canManage: boolean;
  isEmployee: boolean;
  onChanged: () => Promise<void>;
};

export function IDPTasksPanel({ plan, canManage, isEmployee, onChanged }: Props) {
  const [tasks, setTasks] = useState<IDPTask[]>([]);
  const [categories, setCategories] = useState<NamedCatalogItem[]>([]);
  const [tags, setTags] = useState<NamedCatalogItem[]>([]);
  const [form, setForm] = useState<TaskPayload>(emptyForm);
  const [editingID, setEditingID] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [commentTaskID, setCommentTaskID] = useState<string | null>(null);
  const [auditTaskID, setAuditTaskID] = useState<string | null>(null);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<TaskFilters>({ sort: 'due_date', order: 'asc' });

  const editable = canManage && (plan.status === 'draft' || plan.status === 'active');
  const canReport = isEmployee && plan.status === 'active';

  async function loadTasks() {
    setBusy(true);
    setError(null);
    try {
      setTasks(await listTasks(plan.id, filters));
    } catch {
      setError('Не удалось загрузить задачи');
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => { void loadTasks(); }, [plan.id, filters.status, filters.priority, filters.competencyId, filters.sort, filters.order]);

  useEffect(() => {
    if (!editable) {
      setCategories([]);
      setTags([]);
      return;
    }
    void Promise.all([listTaskCategories(), listTags()])
      .then(([categoryResult, tagResult]) => { setCategories(categoryResult); setTags(tagResult); })
      .catch(() => setError('Не удалось загрузить справочники задач'));
  }, [plan.id, editable]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const payload = {
        ...form,
        title: form.title.trim(),
        description: form.description?.trim() || undefined,
        manager_comment: form.manager_comment?.trim() || undefined,
        due_date: form.due_date || undefined,
        category_id: form.category_id || undefined,
        resources: form.resources.filter((item) => item.title.trim() && item.url.trim()),
      };
      if (editingID) await updateTask(editingID, payload);
      else await createTask(plan.id, payload);
      closeForm();
      await Promise.all([loadTasks(), onChanged()]);
    } catch {
      setError('Не удалось сохранить задачу. Проверьте даты, ссылки и выбранные справочники.');
      setBusy(false);
    }
  }

  function edit(task: IDPTask) {
    setEditingID(task.id);
    setForm({
      title: task.title, description: task.description ?? '', category_id: task.category?.id,
      priority: task.priority, due_date: task.due_date, status: task.status, progress: task.progress,
      manager_rating: task.manager_rating, manager_comment: task.manager_comment ?? '',
      competency_ids: task.competencies.map((item) => item.id),
      tag_ids: task.tags.map((item) => item.id), resources: task.resources.map(({ title, url }) => ({ title, url })),
    });
    setShowForm(true);
  }

  async function remove(task: IDPTask) {
    if (!window.confirm(`Удалить задачу «${task.title}»?`)) return;
    setBusy(true);
    try {
      await deleteTask(task.id);
      await Promise.all([loadTasks(), onChanged()]);
    } catch {
      setError('Не удалось удалить задачу');
      setBusy(false);
    }
  }

  async function report(task: IDPTask, status: TaskStatus, progress: number) {
    setBusy(true);
    setError(null);
    try {
      await updateTaskProgress(task.id, { status, progress });
      await Promise.all([loadTasks(), onChanged()]);
    } catch {
      setError('Не удалось обновить прогресс');
      setBusy(false);
    }
  }

  function closeForm() {
    setEditingID(null);
    setForm(emptyForm);
    setShowForm(false);
  }

  function toggleList(field: 'competency_ids' | 'tag_ids', id: string, checked: boolean) {
    setForm((current) => ({
      ...current,
      [field]: checked ? [...current[field], id] : current[field].filter((item) => item !== id),
    }));
  }

  function setResource(index: number, value: TaskResource) {
    setForm((current) => ({ ...current, resources: current.resources.map((item, i) => i === index ? value : item) }));
  }

  return (
    <div className="task-panel">
      <div className="task-panel-heading">
        <strong>Задачи</strong>
        {editable && !showForm && (
          <button className="secondary-button compact" type="button" onClick={() => setShowForm(true)}>
            <Plus size={16} /> Добавить
          </button>
        )}
      </div>
      {error && <div className="form-error">{error}</div>}
      <div className="task-filters" aria-label="Фильтры задач">
        <label><span>Статус</span><select value={filters.status ?? ''} onChange={(event) => setFilters((current) => ({ ...current, status: event.target.value as TaskStatus || undefined }))}><option value="">Все</option>{Object.entries(statusLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
        <label><span>Приоритет</span><select value={filters.priority ?? ''} onChange={(event) => setFilters((current) => ({ ...current, priority: event.target.value as TaskPriority || undefined }))}><option value="">Все</option><option value="high">Высокий</option><option value="medium">Средний</option><option value="low">Низкий</option></select></label>
        <label><span>Компетенция</span><select value={filters.competencyId ?? ''} onChange={(event) => setFilters((current) => ({ ...current, competencyId: event.target.value || undefined }))}><option value="">Все</option>{plan.competencies.map((item) => <option key={item.competency_id} value={item.competency_id}>{item.name ?? item.competency_id}</option>)}</select></label>
        <label><span>Сортировка</span><select value={`${filters.sort}:${filters.order}`} onChange={(event) => { const [sort, order] = event.target.value.split(':') as [TaskFilters['sort'], TaskFilters['order']]; setFilters((current) => ({ ...current, sort, order })); }}><option value="due_date:asc">Срок: сначала ближайшие</option><option value="due_date:desc">Срок: сначала поздние</option><option value="priority:asc">Сначала высокий приоритет</option><option value="status:asc">По статусу</option></select></label>
        <button className="icon-button" type="button" title="Сбросить фильтры" aria-label="Сбросить фильтры" onClick={() => setFilters({ sort: 'due_date', order: 'asc' })}><RotateCcw size={17} /></button>
      </div>
      {!busy && tasks.length === 0 && <div className="task-empty">Задач пока нет</div>}

      <div className="task-list" aria-busy={busy}>
        {tasks.map((task) => (
          <div className="task-item" key={task.id}>
            <div className="task-title-row">
              <div>
                <strong>{task.title}</strong>
                <span>{task.category?.name ?? 'Без категории'} · {task.due_date ? formatDate(task.due_date) : 'Без срока'}</span>
              </div>
              <span className={`status-pill task-${task.status}`}>{statusLabels[task.status]}</span>
            </div>
            {task.description && <p>{task.description}</p>}
            <div className="task-progress-row">
              <div className="progress-track"><span style={{ width: `${task.progress}%` }} /></div>
              <strong>{task.progress}%</strong>
            </div>
            {(task.competencies.length > 0 || task.tags.length > 0) && (
              <div className="task-chips">
                {[...task.competencies, ...task.tags].map((item) => <span key={item.id}>{item.name}</span>)}
              </div>
            )}
            {task.resources.length > 0 && (
              <div className="task-resources">
                {task.resources.map((resource) => (
                  <a href={resource.url} key={resource.id ?? resource.url} rel="noreferrer" target="_blank">
                    <ExternalLink size={14} /> {resource.title}
                  </a>
                ))}
              </div>
            )}
            {task.manager_comment && (
              <div className="task-feedback">
                <span><strong>Руководитель:</strong> {task.manager_comment}</span>
              </div>
            )}
            <div className="row-actions">
              <button className="secondary-button compact" type="button" onClick={() => setCommentTaskID(commentTaskID === task.id ? null : task.id)}><MessageSquare size={16} />Комментарии</button>
              <button className="icon-button" type="button" title="История изменений" aria-label="История изменений" onClick={() => setAuditTaskID(auditTaskID === task.id ? null : task.id)}><Activity size={17} /></button>
              {canReport && <ProgressEditor task={task} disabled={busy} onSave={report} />}
              {editable && <button className="icon-button" type="button" title="Редактировать" aria-label="Редактировать" onClick={() => edit(task)}><Edit3 size={17} /></button>}
              {editable && <button className="icon-button danger" type="button" title="Удалить" aria-label="Удалить" onClick={() => void remove(task)}><Trash2 size={17} /></button>}
            </div>
            {commentTaskID === task.id && <CommentsThread entityType="task" entityID={task.id} title="Комментарии к задаче" />}
            {auditTaskID === task.id && <AuditTrail entityType="task" entityID={task.id} title="История задачи" />}
          </div>
        ))}
      </div>

      {showForm && editable && (
        <form className="task-form" onSubmit={submit}>
          <div className="task-panel-heading"><strong>{editingID ? 'Редактирование задачи' : 'Новая задача'}</strong></div>
          <label className="form-field"><span>Название</span><input required maxLength={200} value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} /></label>
          <label className="form-field"><span>Описание</span><textarea maxLength={5000} value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} /></label>
          <div className="form-grid">
            <label className="form-field"><span>Категория</span><select value={form.category_id ?? ''} onChange={(e) => setForm({ ...form, category_id: e.target.value })}><option value="">{categories.length === 0 ? 'Категории не созданы' : 'Без категории'}</option>{categories.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
            <label className="form-field"><span>Приоритет</span><select value={form.priority} onChange={(e) => setForm({ ...form, priority: e.target.value as TaskPriority })}><option value="low">Низкий</option><option value="medium">Средний</option><option value="high">Высокий</option></select></label>
            <label className="form-field"><span>Срок</span><input type="date" min={plan.start_date} max={plan.end_date} value={form.due_date ?? ''} onChange={(e) => setForm({ ...form, due_date: e.target.value })} /></label>
            {editingID && form.status === 'completed' && <label className="form-field"><span>Оценка руководителя</span><select value={form.manager_rating ?? ''} onChange={(e) => setForm({ ...form, manager_rating: e.target.value as TaskRating || undefined })}><option value="">Без оценки</option>{Object.entries(ratingLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>}
          </div>
          {editingID && form.status === 'completed' && <label className="form-field"><span>Комментарий руководителя</span><textarea value={form.manager_comment ?? ''} onChange={(e) => setForm({ ...form, manager_comment: e.target.value })} /></label>}
          <ChoiceGroup title="Компетенции ИПР" items={plan.competencies.map((item) => ({ id: item.competency_id, name: item.name ?? item.competency_id }))} selected={form.competency_ids} onToggle={(id, checked) => toggleList('competency_ids', id, checked)} />
          <ChoiceGroup title="Теги" items={tags} selected={form.tag_ids} onToggle={(id, checked) => toggleList('tag_ids', id, checked)} />
          <div className="resource-editor"><strong>Ресурсы</strong>{form.resources.map((resource, index) => <div className="resource-row" key={index}><input placeholder="Название" value={resource.title} onChange={(e) => setResource(index, { ...resource, title: e.target.value })} /><input placeholder="https://..." type="url" value={resource.url} onChange={(e) => setResource(index, { ...resource, url: e.target.value })} /><button className="icon-button danger" type="button" aria-label="Удалить ресурс" onClick={() => setForm({ ...form, resources: form.resources.filter((_, i) => i !== index) })}><X size={16} /></button></div>)}<button className="secondary-button compact" type="button" onClick={() => setForm({ ...form, resources: [...form.resources, { title: '', url: '' }] })}><Plus size={16} /> Ресурс</button></div>
          <div className="button-row"><button className="primary-button" disabled={busy} type="submit"><Save size={17} /> Сохранить</button><button className="secondary-button" type="button" onClick={closeForm}><X size={17} /> Отмена</button></div>
        </form>
      )}
    </div>
  );
}

function ProgressEditor({ task, disabled, onSave }: { task: IDPTask; disabled: boolean; onSave: (task: IDPTask, status: TaskStatus, progress: number) => Promise<void> }) {
  const [open, setOpen] = useState(false);
  const [progress, setProgress] = useState(task.progress);
  const [status, setStatus] = useState(task.status);
  if (!open) return <button className="secondary-button compact" type="button" onClick={() => setOpen(true)}>Обновить прогресс</button>;
  return <div className="progress-editor"><input aria-label="Прогресс" type="number" min={0} max={100} value={progress} onChange={(e) => setProgress(Number(e.target.value))} /><select aria-label="Статус" value={status} onChange={(e) => { const value = e.target.value as TaskStatus; setStatus(value); if (value === 'completed') setProgress(100); }}>{Object.entries(statusLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select><button className="icon-button success" disabled={disabled} type="button" aria-label="Сохранить прогресс" onClick={() => void onSave(task, status, progress).then(() => setOpen(false))}><Check size={17} /></button><button className="icon-button" type="button" aria-label="Отмена" onClick={() => setOpen(false)}><X size={17} /></button></div>;
}

function ChoiceGroup({ title, items, selected, onToggle }: { title: string; items: NamedCatalogItem[]; selected: string[]; onToggle: (id: string, checked: boolean) => void }) {
  if (items.length === 0) return null;
  return <fieldset className="choice-group"><legend>{title}</legend>{items.map((item) => <label key={item.id}><input type="checkbox" checked={selected.includes(item.id)} onChange={(e) => onToggle(item.id, e.target.checked)} />{item.name}</label>)}</fieldset>;
}

function formatDate(value: string) { return new Intl.DateTimeFormat('ru-RU').format(new Date(`${value}T00:00:00`)); }
