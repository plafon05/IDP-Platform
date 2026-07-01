import { Building2, ChevronDown, ChevronRight, Edit3, Plus, Trash2, Users, X } from 'lucide-react';
import { FormEvent, useEffect, useMemo, useState } from 'react';
import { createDepartment, deleteDepartment, listDepartments, updateDepartment, type Department } from '../shared/api/departments';

type Form = { name: string; parent_id: string };
const emptyForm: Form = { name: '', parent_id: '' };

export function DepartmentsPage() {
  const [tree, setTree] = useState<Department[]>([]);
  const [form, setForm] = useState<Form>(emptyForm);
  const [editing, setEditing] = useState<Department | null>(null);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const flat = useMemo(() => flatten(tree), [tree]);
  const employees = flat.reduce((sum, item) => sum + item.employees.length, 0);

  async function load() { setBusy(true); setError(null); try { const result = await listDepartments(); setTree(result); setExpanded((current) => current.size ? current : new Set(result.map((item) => item.id))); } catch { setError('Не удалось загрузить подразделения'); } finally { setBusy(false); } }
  useEffect(() => { void load(); }, []);

  async function submit(event: FormEvent) { event.preventDefault(); setBusy(true); setError(null); try { const input = { name: form.name.trim(), parent_id: form.parent_id || undefined }; if (editing) await updateDepartment(editing.id, input); else await createDepartment(input); setEditing(null); setForm(emptyForm); await load(); } catch { setError('Не удалось сохранить подразделение. Проверьте иерархию и глубину дерева.'); } finally { setBusy(false); } }
  function startEdit(item: Department) { setEditing(item); setForm({ name: item.name, parent_id: item.parent_id ?? '' }); }
  function startChild(item: Department) { setEditing(null); setForm({ name: '', parent_id: item.id }); document.querySelector('.department-form')?.scrollIntoView({ behavior: 'smooth' }); }
  async function remove(item: Department) { if (!window.confirm(`Удалить подразделение «${item.name}»?`)) return; setError(null); try { await deleteDepartment(item.id); await load(); } catch { setError('Нельзя удалить подразделение с сотрудниками или дочерними подразделениями.'); } }
  function toggle(id: string) { setExpanded((current) => { const next = new Set(current); if (next.has(id)) next.delete(id); else next.add(id); return next; }); }

  return <div className="departments-page">
    <section className="section-header"><div><span>Организация</span><h2>Подразделения</h2></div><div className="summary-strip"><strong>{flat.length}</strong><span>Подразделений</span><strong>{employees}</strong><span>Сотрудников</span></div></section>
    {error && <div className="form-error">{error}</div>}
    <section className="departments-layout">
      <div className="panel"><div className="panel-header"><div><h2>Структура организации</h2><p>Иерархия до пяти уровней</p></div><Building2 size={20} /></div>
        <div className="department-tree" aria-busy={busy}>{!busy && tree.length === 0 && <div className="empty-state">Создайте первое подразделение</div>}{tree.map((item) => <DepartmentNode key={item.id} item={item} expanded={expanded} onToggle={toggle} onEdit={startEdit} onChild={startChild} onDelete={remove} />)}</div>
      </div>
      <form className="panel department-form" onSubmit={submit}><div className="panel-header"><div><h2>{editing ? 'Редактирование' : 'Новое подразделение'}</h2><p>{editing ? editing.name : 'Добавьте корневой или дочерний узел'}</p></div>{editing && <button className="icon-button" type="button" onClick={() => { setEditing(null); setForm(emptyForm); }} aria-label="Отмена"><X size={18} /></button>}</div>
        <label className="form-field"><span>Название</span><input required maxLength={200} value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} /></label>
        <label className="form-field"><span>Родительское подразделение</span><select value={form.parent_id} onChange={(event) => setForm({ ...form, parent_id: event.target.value })}><option value="">Корневой уровень</option>{flat.filter((item) => item.id !== editing?.id && item.depth < 5).map((item) => <option key={item.id} value={item.id}>{'— '.repeat(item.depth - 1)}{item.name}</option>)}</select></label>
        <button className="primary-button" disabled={busy} type="submit"><Plus size={18} />{editing ? 'Сохранить' : 'Создать'}</button>
      </form>
    </section>
  </div>;
}

function DepartmentNode({ item, expanded, onToggle, onEdit, onChild, onDelete }: { item: Department; expanded: Set<string>; onToggle: (id: string) => void; onEdit: (item: Department) => void; onChild: (item: Department) => void; onDelete: (item: Department) => void }) {
  const open = expanded.has(item.id); const hasContent = item.children.length > 0 || item.employees.length > 0;
  return <div className="department-branch"><div className="department-row"><button className="tree-toggle" disabled={!hasContent} onClick={() => onToggle(item.id)} type="button" aria-label={open ? 'Свернуть' : 'Развернуть'}>{hasContent ? open ? <ChevronDown size={18} /> : <ChevronRight size={18} /> : <span />}</button><Building2 size={19} /><div className="department-name"><strong>{item.name}</strong><span>{item.employees.length} сотрудников · уровень {item.depth}</span></div><div className="row-actions">{item.depth < 5 && <button className="icon-button" onClick={() => onChild(item)} title="Добавить дочернее" type="button"><Plus size={17} /></button>}<button className="icon-button" onClick={() => onEdit(item)} title="Редактировать" type="button"><Edit3 size={17} /></button><button className="icon-button danger" onClick={() => void onDelete(item)} title="Удалить" type="button"><Trash2 size={17} /></button></div></div>{open && <div className="department-children">{item.employees.map((employee) => <a className="department-employee" href={`/employees/${employee.id}`} key={employee.id}><Users size={16} /><span><strong>{employee.name}</strong><small>{employee.position}</small></span></a>)}{item.children.map((child) => <DepartmentNode key={child.id} item={child} expanded={expanded} onToggle={onToggle} onEdit={onEdit} onChild={onChild} onDelete={onDelete} />)}</div>}</div>;
}
function flatten(items: Department[]): Department[] { return items.flatMap((item) => [item, ...flatten(item.children)]); }
