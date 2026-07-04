import { Archive, CopyPlus, Edit3, Plus, Save, Trash2, X } from 'lucide-react';
import { FormEvent, useEffect, useState } from 'react';
import { useSessionStore } from '../entities/session/model';
import { listCompetencies, listTaskCategories, type Competency, type NamedCatalogItem } from '../shared/api/catalog';
import { listSubordinates, listUsers, type User } from '../shared/api/users';
import { applyTemplate, archiveTemplate, createTemplate, listTemplates, updateTemplate, type IDPTemplate, type TemplatePayload, type TemplateTask } from '../shared/api/templates';
import { MarkdownEditor } from '../components/MarkdownEditor';

const emptyTask: TemplateTask = { title: '', description: '', priority: 'medium' };
const emptyForm: TemplatePayload = { title: '', description: '', goals: '', target_role: '', is_active: true, tasks: [], competencies: [] };

export function TemplatesPage() {
  const user = useSessionStore((state) => state.user);
  const isHR = user?.roles.includes('hr_admin') ?? false;
  const [templates, setTemplates] = useState<IDPTemplate[]>([]);
  const [competencies, setCompetencies] = useState<Competency[]>([]);
  const [categories, setCategories] = useState<NamedCatalogItem[]>([]);
  const [employees, setEmployees] = useState<User[]>([]);
  const [form, setForm] = useState<TemplatePayload>(emptyForm);
  const [editingID, setEditingID] = useState<string | null>(null);
  const [applyTo, setApplyTo] = useState<IDPTemplate | null>(null);
  const [application, setApplication] = useState({ employee_id: '', title: '', start_date: '', end_date: '' });
  const [taskEditor, setTaskEditor] = useState<{ index: number | null; value: TemplateTask } | null>(null);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  async function load() {
    setBusy(true); setError(null);
    try {
      const [templateResult, competencyResult, categoryResult, employeeResult] = await Promise.all([
        listTemplates(), listCompetencies(false), listTaskCategories(),
        isHR ? listUsers().then((result) => result.data.filter((item) => item.is_active && item.manager_id)) : listSubordinates(),
      ]);
      setTemplates(templateResult); setCompetencies(competencyResult); setCategories(categoryResult); setEmployees(employeeResult);
    } catch { setError('Не удалось загрузить шаблоны'); } finally { setBusy(false); }
  }
  useEffect(() => { void load(); }, []);

  async function submit(event: FormEvent) {
    event.preventDefault(); setError(null); setNotice(null);
    if (form.tasks.length === 0) { setError('Добавьте хотя бы одну задачу в шаблон'); return; }
    setBusy(true);
    const payload = { ...form, title: form.title.trim(), description: form.description?.trim() || undefined, goals: form.goals?.trim() || undefined, target_role: form.target_role?.trim() || undefined, tasks: form.tasks.map((task) => ({ ...task, title: task.title.trim(), description: task.description?.trim() || undefined, category_id: task.category_id || undefined })) };
    try { if (editingID) await updateTemplate(editingID, payload); else await createTemplate(payload); reset(); setNotice(editingID ? 'Шаблон обновлён' : 'Шаблон создан'); await load(); } catch { setError('Не удалось сохранить шаблон'); setBusy(false); }
  }

  function edit(item: IDPTemplate) { setEditingID(item.id); setForm({ title: item.title, description: item.description ?? '', goals: item.goals ?? '', target_role: item.target_role ?? '', is_active: item.is_active, tasks: item.tasks.map(({ id: _id, ...task }) => task), competencies: item.competencies.map(({ name: _name, ...value }) => value) }); }
  function reset() { setEditingID(null); setTaskEditor(null); setForm({ ...emptyForm, tasks: [], competencies: [] }); }
  function saveTask(event: FormEvent) { event.preventDefault(); if (!taskEditor) return; const task = { ...taskEditor.value, title: taskEditor.value.title.trim(), description: taskEditor.value.description?.trim() || undefined, category_id: taskEditor.value.category_id || undefined }; setForm((current) => ({ ...current, tasks: taskEditor.index === null ? [...current.tasks, task] : current.tasks.map((item, index) => index === taskEditor.index ? task : item) })); setTaskEditor(null); }
  function toggleCompetency(id: string, checked: boolean) { setForm((current) => ({ ...current, competencies: checked ? [...current.competencies, { competency_id: id, target_level: 2 }] : current.competencies.filter((item) => item.competency_id !== id) })); }

  async function apply(event: FormEvent) {
    event.preventDefault(); if (!applyTo) return; setBusy(true); setError(null);
    try { await applyTemplate(applyTo.id, application); setApplyTo(null); setApplication({ employee_id: '', title: '', start_date: '', end_date: '' }); setNotice('Черновик ИПР создан из шаблона'); } catch { setError('Не удалось применить шаблон. Проверьте период и сроки задач.'); } finally { setBusy(false); }
  }

  return <div className="templates-page">
    <section className="section-header"><div><span>Повторное использование</span><h2>Шаблоны ИПР</h2></div><div className="summary-strip"><strong>{templates.filter((item) => item.is_active).length}</strong><span>Активны</span></div></section>
    {error && <div className="form-error">{error}</div>}{notice && <div className="form-success">{notice}</div>}
    <section className="templates-layout">
      <div className="panel"><div className="panel-header"><div><h2>Доступные шаблоны</h2><p>Наборы задач и компетенций для новых ИПР</p></div></div><div className="template-list" aria-busy={busy}>{templates.length === 0 && !busy && <div className="empty-state">Шаблонов пока нет</div>}{templates.map((item) => { const own = isHR || item.creator_id === user?.id; return <article className="template-row" key={item.id}><div className="template-heading"><div><strong>{item.title}</strong><span>{item.target_role || 'Для любой роли'}</span></div><span className={`status-pill ${item.is_active ? 'idp-active' : 'idp-cancelled'}`}>{item.is_active ? 'Активен' : 'Архив'}</span></div>{item.description && <p>{item.description}</p>}<div className="template-summary"><span>{item.tasks.length} задач</span><span>{item.competencies.length} компетенций</span></div><div className="row-actions">{item.is_active && <button className="primary-button compact" onClick={() => { setApplyTo(item); setApplication((current) => ({ ...current, title: item.title })); }} type="button"><CopyPlus size={16} />Применить</button>}{own && <button className="icon-button" onClick={() => edit(item)} aria-label="Редактировать" title="Редактировать" type="button"><Edit3 size={17} /></button>}{own && item.is_active && <button className="icon-button danger" onClick={() => void archiveTemplate(item.id).then(load)} aria-label="Архивировать" title="Архивировать" type="button"><Archive size={17} /></button>}</div></article>; })}</div></div>
      <form className="panel template-form" onSubmit={submit}><div className="panel-header"><div><h2>{editingID ? 'Редактирование шаблона' : 'Новый шаблон'}</h2><p>Сроки задач задаются в днях от начала ИПР</p></div><Plus size={20} /></div>
        <label className="form-field"><span>Название</span><input required maxLength={300} value={form.title} onChange={(event) => setForm({ ...form, title: event.target.value })} /></label>
        <label className="form-field"><span>Целевая роль</span><input maxLength={200} value={form.target_role ?? ''} onChange={(event) => setForm({ ...form, target_role: event.target.value })} /></label>
        <label className="form-field"><span>Описание</span><textarea maxLength={2000} value={form.description ?? ''} onChange={(event) => setForm({ ...form, description: event.target.value })} /></label>
        <label className="form-field"><span>Цели ИПР</span><MarkdownEditor value={form.goals ?? ''} onChange={(goals) => setForm({ ...form, goals })} /></label>
        <fieldset className="choice-group"><legend>Компетенции</legend>{competencies.map((competency) => { const selected = form.competencies.find((item) => item.competency_id === competency.id); return <div className="template-competency" key={competency.id}><label><input type="checkbox" checked={Boolean(selected)} onChange={(event) => toggleCompetency(competency.id, event.target.checked)} />{competency.name}</label>{selected && <select aria-label={`Целевой уровень ${competency.name}`} value={selected.target_level} onChange={(event) => setForm((current) => ({ ...current, competencies: current.competencies.map((item) => item.competency_id === competency.id ? { ...item, target_level: Number(event.target.value) } : item) }))}>{[1,2,3,4].map((level) => <option key={level} value={level}>Уровень {level}</option>)}</select>}</div>; })}</fieldset>
        <div className="template-tasks"><div className="task-panel-heading"><div><strong>Задачи</strong><span>{form.tasks.length ? `${form.tasks.length} добавлено` : 'Добавьте задачи шаблона'}</span></div><button className="secondary-button compact" type="button" onClick={() => setTaskEditor({ index: null, value: { ...emptyTask } })}><Plus size={15} />Добавить</button></div><div className="template-task-list">{form.tasks.map((task, index) => { const category = categories.find((item) => item.id === task.category_id)?.name; return <div className="template-task-row" key={`${index}-${task.title}`}><div><strong>{task.title}</strong><span>{[category, priorityLabel(task.priority), task.due_offset_days === undefined ? null : `срок: день ${task.due_offset_days}`].filter(Boolean).join(' · ')}</span>{task.description && <small>{task.description}</small>}</div><div className="row-actions"><button className="icon-button" type="button" aria-label="Редактировать задачу" title="Редактировать" onClick={() => setTaskEditor({ index, value: { ...task } })}><Edit3 size={16} /></button><button className="icon-button danger" type="button" aria-label="Удалить задачу" title="Удалить" onClick={() => setForm((current) => ({ ...current, tasks: current.tasks.filter((_, i) => i !== index) }))}><Trash2 size={16} /></button></div></div>; })}{form.tasks.length === 0 && <div className="empty-state compact">Задач пока нет</div>}</div></div>
        <div className="button-row"><button className="primary-button" disabled={busy} type="submit"><Save size={17} />Сохранить</button>{editingID && <button className="secondary-button" type="button" onClick={reset}><X size={17} />Отмена</button>}</div>
      </form>
    </section>
    {taskEditor && <div className="modal-backdrop" role="presentation"><form className="panel template-task-dialog" onSubmit={saveTask} role="dialog" aria-modal="true" aria-labelledby="template-task-title"><div className="panel-header"><div><h2 id="template-task-title">{taskEditor.index === null ? 'Новая задача' : 'Редактирование задачи'}</h2><p>Задача будет добавлена в шаблон</p></div><button className="icon-button" type="button" aria-label="Закрыть" onClick={() => setTaskEditor(null)}><X size={18} /></button></div><label className="form-field"><span>Название</span><input autoFocus required maxLength={200} value={taskEditor.value.title} onChange={(event) => setTaskEditor({ ...taskEditor, value: { ...taskEditor.value, title: event.target.value } })} /></label><label className="form-field"><span>Описание</span><textarea maxLength={2000} value={taskEditor.value.description ?? ''} onChange={(event) => setTaskEditor({ ...taskEditor, value: { ...taskEditor.value, description: event.target.value } })} /></label><div className="form-grid"><label className="form-field"><span>Категория</span><select value={taskEditor.value.category_id ?? ''} onChange={(event) => setTaskEditor({ ...taskEditor, value: { ...taskEditor.value, category_id: event.target.value || undefined } })}><option value="">Без категории</option>{categories.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label><label className="form-field"><span>Приоритет</span><select value={taskEditor.value.priority} onChange={(event) => setTaskEditor({ ...taskEditor, value: { ...taskEditor.value, priority: event.target.value as TemplateTask['priority'] } })}><option value="low">Низкий</option><option value="medium">Средний</option><option value="high">Высокий</option></select></label><label className="form-field"><span>День срока</span><input type="number" min={0} max={3650} value={taskEditor.value.due_offset_days ?? ''} onChange={(event) => setTaskEditor({ ...taskEditor, value: { ...taskEditor.value, due_offset_days: event.target.value === '' ? undefined : Number(event.target.value) } })} /></label></div><div className="button-row"><button className="primary-button" type="submit"><Save size={17} />Сохранить задачу</button><button className="secondary-button" type="button" onClick={() => setTaskEditor(null)}>Отмена</button></div></form></div>}
    {applyTo && <div className="modal-backdrop" role="presentation"><form className="panel apply-template-dialog" onSubmit={apply} role="dialog" aria-modal="true"><div className="panel-header"><div><h2>Создать ИПР из шаблона</h2><p>{applyTo.title}</p></div><button className="icon-button" type="button" aria-label="Закрыть" onClick={() => setApplyTo(null)}><X size={18} /></button></div><label className="form-field"><span>Сотрудник</span><select required value={application.employee_id} onChange={(event) => setApplication({ ...application, employee_id: event.target.value })}><option value="">Выберите сотрудника</option>{employees.map((employee) => <option key={employee.id} value={employee.id}>{employee.last_name} {employee.first_name} - {employee.position}</option>)}</select></label><label className="form-field"><span>Название ИПР</span><input required value={application.title} onChange={(event) => setApplication({ ...application, title: event.target.value })} /></label><div className="form-grid"><label className="form-field"><span>Начало</span><input required type="date" value={application.start_date} onChange={(event) => setApplication({ ...application, start_date: event.target.value })} /></label><label className="form-field"><span>Окончание</span><input required type="date" min={application.start_date} value={application.end_date} onChange={(event) => setApplication({ ...application, end_date: event.target.value })} /></label></div><button className="primary-button" disabled={busy} type="submit"><CopyPlus size={17} />Создать черновик</button></form></div>}
  </div>;
}

function priorityLabel(priority: TemplateTask['priority']) { return { low: 'низкий приоритет', medium: 'средний приоритет', high: 'высокий приоритет' }[priority]; }
