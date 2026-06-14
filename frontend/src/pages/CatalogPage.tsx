import { Archive, BookMarked, Edit3, Plus, RefreshCw, Save, Tags, Trash2, X } from 'lucide-react';
import type { Dispatch, SetStateAction } from 'react';
import { FormEvent, useEffect, useMemo, useState } from 'react';
import {
  archiveCompetency,
  createCompetency,
  createTag,
  createTaskCategory,
  deleteTag,
  deleteTaskCategory,
  listCompetencies,
  listTags,
  listTaskCategories,
  updateCompetency,
  updateTag,
  updateTaskCategory,
  type Competency,
  type CompetencyCategory,
  type CompetencyLevel,
  type NamedCatalogItem,
} from '../shared/api/catalog';

const categoryLabels: Record<CompetencyCategory, string> = {
  hard: 'Hard skills',
  soft: 'Soft skills',
  leadership: 'Лидерство',
  management: 'Управление',
  technical: 'Технические',
};

const emptyCompetencyForm = {
  name: '',
  description: '',
  category: 'hard' as CompetencyCategory,
  is_active: true,
  levels: [
    { level: 1, title: '', description: '' },
    { level: 2, title: '', description: '' },
    { level: 3, title: '', description: '' },
    { level: 4, title: '', description: '' },
  ],
};

type CompetencyForm = typeof emptyCompetencyForm;
type NamedKind = 'task-category' | 'tag';

export function CatalogPage() {
  const [competencies, setCompetencies] = useState<Competency[]>([]);
  const [taskCategories, setTaskCategories] = useState<NamedCatalogItem[]>([]);
  const [tags, setTags] = useState<NamedCatalogItem[]>([]);
  const [competencyForm, setCompetencyForm] = useState<CompetencyForm>(emptyCompetencyForm);
  const [editingCompetencyID, setEditingCompetencyID] = useState<string | null>(null);
  const [taskCategoryName, setTaskCategoryName] = useState('');
  const [tagName, setTagName] = useState('');
  const [editingNamed, setEditingNamed] = useState<{ kind: NamedKind; id: string; name: string } | null>(null);
  const [status, setStatus] = useState<'loading' | 'idle' | 'saving'>('loading');
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const activeCompetencies = useMemo(() => competencies.filter((item) => item.is_active).length, [competencies]);

  async function loadCatalogs() {
    setStatus('loading');
    setError(null);
    try {
      const [competenciesResult, taskCategoriesResult, tagsResult] = await Promise.all([
        listCompetencies(true),
        listTaskCategories(),
        listTags(),
      ]);
      setCompetencies(competenciesResult);
      setTaskCategories(taskCategoriesResult);
      setTags(tagsResult);
    } catch {
      setError('Не удалось загрузить справочники');
    } finally {
      setStatus('idle');
    }
  }

  useEffect(() => {
    void loadCatalogs();
  }, []);

  function startEditCompetency(item: Competency) {
    const levels = [1, 2, 3, 4].map((level) => {
      const existing = item.levels?.find((candidate) => candidate.level === level);
      return {
        level,
        title: existing?.title ?? '',
        description: existing?.description ?? '',
      };
    });

    setEditingCompetencyID(item.id);
    setCompetencyForm({
      name: item.name,
      description: item.description ?? '',
      category: item.category,
      is_active: item.is_active,
      levels,
    });
    setError(null);
    setNotice(null);
  }

  async function handleCompetencySubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setStatus('saving');
    setError(null);
    setNotice(null);

    try {
      const payload = {
        name: competencyForm.name.trim(),
        description: competencyForm.description.trim() || undefined,
        category: competencyForm.category,
        is_active: competencyForm.is_active,
        levels: normalizedLevels(competencyForm.levels),
      };

      if (editingCompetencyID) {
        await updateCompetency(editingCompetencyID, payload);
        setNotice('Компетенция обновлена');
      } else {
        await createCompetency(payload);
        setNotice('Компетенция создана');
      }

      resetCompetencyForm();
      await loadCatalogs();
    } catch {
      setError('Не удалось сохранить компетенцию');
      setStatus('idle');
    }
  }

  async function handleArchiveCompetency(item: Competency) {
    if (!window.confirm(`Архивировать компетенцию "${item.name}"? Она останется в истории ИПР.`)) {
      return;
    }

    setStatus('saving');
    setError(null);
    setNotice(null);
    try {
      await archiveCompetency(item.id);
      await loadCatalogs();
      setNotice('Компетенция архивирована');
    } catch {
      setError('Не удалось архивировать компетенцию');
      setStatus('idle');
    }
  }

  async function handleNamedSubmit(kind: NamedKind, event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const name = kind === 'task-category' ? taskCategoryName.trim() : tagName.trim();
    if (!name) {
      setError('Название обязательно');
      return;
    }

    setStatus('saving');
    setError(null);
    setNotice(null);
    try {
      if (editingNamed?.kind === kind) {
        if (kind === 'task-category') {
          await updateTaskCategory(editingNamed.id, name);
        } else {
          await updateTag(editingNamed.id, name);
        }
        setEditingNamed(null);
        setNotice('Элемент обновлён');
      } else if (kind === 'task-category') {
        await createTaskCategory(name);
        setNotice('Категория создана');
      } else {
        await createTag(name);
        setNotice('Тег создан');
      }

      setTaskCategoryName('');
      setTagName('');
      await loadCatalogs();
    } catch {
      setError('Не удалось сохранить элемент справочника');
      setStatus('idle');
    }
  }

  async function handleDeleteNamed(kind: NamedKind, item: NamedCatalogItem) {
    if (!window.confirm(`Удалить "${item.name}"? Если элемент уже используется, API не даст его удалить.`)) {
      return;
    }

    setStatus('saving');
    setError(null);
    setNotice(null);
    try {
      if (kind === 'task-category') {
        await deleteTaskCategory(item.id);
      } else {
        await deleteTag(item.id);
      }
      await loadCatalogs();
      setNotice('Элемент удалён');
    } catch {
      setError('Не удалось удалить элемент. Возможно, он уже используется.');
      setStatus('idle');
    }
  }

  function startEditNamed(kind: NamedKind, item: NamedCatalogItem) {
    setEditingNamed({ kind, id: item.id, name: item.name });
    if (kind === 'task-category') {
      setTaskCategoryName(item.name);
    } else {
      setTagName(item.name);
    }
    setError(null);
    setNotice(null);
  }

  function resetCompetencyForm() {
    setEditingCompetencyID(null);
    setCompetencyForm(emptyCompetencyForm);
  }

  return (
    <div className="catalog-page">
      <section className="section-header">
        <div>
          <span>Администрирование</span>
          <h2>Справочники</h2>
        </div>
        <div className="summary-strip" aria-label="Статистика справочников">
          <strong>{activeCompetencies}</strong>
          <span>Компетенции</span>
          <strong>{taskCategories.length + tags.length}</strong>
          <span>Категории и теги</span>
        </div>
      </section>

      {error && <div className="form-error">{error}</div>}
      {notice && <div className="form-success">{notice}</div>}

      <section className="catalog-layout">
        <div className="panel">
          <div className="panel-header">
            <div>
              <h2>Компетенции</h2>
              <p>Уровни используются при постановке целей в ИПР</p>
            </div>
            <button className="icon-button" onClick={() => void loadCatalogs()} type="button" aria-label="Обновить">
              <RefreshCw size={18} />
            </button>
          </div>

          <div className="catalog-list" aria-busy={status === 'loading'}>
            {competencies.map((item) => (
              <article className={`catalog-row ${item.is_active ? '' : 'muted'}`} key={item.id}>
                <div>
                  <strong>{item.name}</strong>
                  <span>{categoryLabels[item.category]}</span>
                  {item.description && <p>{item.description}</p>}
                  <div className="level-strip">
                    {(item.levels ?? []).map((level) => (
                      <span key={level.level}>
                        L{level.level}: {level.title}
                      </span>
                    ))}
                  </div>
                </div>
                <span className={`status-pill ${item.is_active ? 'online' : 'offline'}`}>
                  {item.is_active ? 'Активна' : 'Архив'}
                </span>
                <div className="row-actions">
                  <button
                    className="icon-button"
                    disabled={status === 'saving'}
                    onClick={() => startEditCompetency(item)}
                    title="Редактировать"
                    type="button"
                    aria-label="Редактировать"
                  >
                    <Edit3 size={18} />
                  </button>
                  <button
                    className="icon-button danger"
                    disabled={status === 'saving' || !item.is_active}
                    onClick={() => void handleArchiveCompetency(item)}
                    title="Архивировать"
                    type="button"
                    aria-label="Архивировать"
                  >
                    <Archive size={18} />
                  </button>
                </div>
              </article>
            ))}
          </div>
        </div>

        <form className="panel catalog-form" onSubmit={handleCompetencySubmit}>
          <div className="panel-header">
            <div>
              <h2>{editingCompetencyID ? 'Редактирование' : 'Новая компетенция'}</h2>
              <p>Название, категория и шкала уровней</p>
            </div>
            <BookMarked size={20} aria-hidden="true" />
          </div>

          <label className="form-field">
            <span>Название</span>
            <input
              onChange={(event) => setCompetencyForm((current) => ({ ...current, name: event.target.value }))}
              required
              value={competencyForm.name}
            />
          </label>
          <label className="form-field">
            <span>Категория</span>
            <select
              onChange={(event) =>
                setCompetencyForm((current) => ({
                  ...current,
                  category: event.target.value as CompetencyCategory,
                }))
              }
              value={competencyForm.category}
            >
              {Object.entries(categoryLabels).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </select>
          </label>
          <label className="form-field">
            <span>Описание</span>
            <textarea
              onChange={(event) => setCompetencyForm((current) => ({ ...current, description: event.target.value }))}
              value={competencyForm.description}
            />
          </label>

          <div className="level-editor">
            {competencyForm.levels.map((level, index) => (
              <div className="level-editor-row" key={level.level}>
                <strong>L{level.level}</strong>
                <input
                  onChange={(event) => updateLevel(index, 'title', event.target.value, setCompetencyForm)}
                  placeholder="Название уровня"
                  value={level.title}
                />
                <input
                  onChange={(event) => updateLevel(index, 'description', event.target.value, setCompetencyForm)}
                  placeholder="Описание"
                  value={level.description}
                />
              </div>
            ))}
          </div>

          <label className="checkbox-line">
            <input
              checked={competencyForm.is_active}
              onChange={(event) => setCompetencyForm((current) => ({ ...current, is_active: event.target.checked }))}
              type="checkbox"
            />
            Активна
          </label>

          <div className="button-row">
            <button className="primary-button" disabled={status === 'saving'} type="submit">
              <Save size={18} />
              Сохранить
            </button>
            {editingCompetencyID && (
              <button className="secondary-button" onClick={resetCompetencyForm} type="button">
                <X size={18} />
                Отмена
              </button>
            )}
          </div>
        </form>
      </section>

      <section className="catalog-named-grid">
        <NamedCatalogPanel
          icon={BookMarked}
          items={taskCategories}
          kind="task-category"
          name={taskCategoryName}
          onChange={setTaskCategoryName}
          onDelete={handleDeleteNamed}
          onEdit={startEditNamed}
          onSubmit={handleNamedSubmit}
          status={status}
          title="Категории задач"
        />
        <NamedCatalogPanel
          icon={Tags}
          items={tags}
          kind="tag"
          name={tagName}
          onChange={setTagName}
          onDelete={handleDeleteNamed}
          onEdit={startEditNamed}
          onSubmit={handleNamedSubmit}
          status={status}
          title="Теги"
        />
      </section>
    </div>
  );
}

function NamedCatalogPanel({
  icon: Icon,
  items,
  kind,
  name,
  onChange,
  onDelete,
  onEdit,
  onSubmit,
  status,
  title,
}: {
  icon: typeof BookMarked;
  items: NamedCatalogItem[];
  kind: NamedKind;
  name: string;
  onChange: (value: string) => void;
  onDelete: (kind: NamedKind, item: NamedCatalogItem) => Promise<void>;
  onEdit: (kind: NamedKind, item: NamedCatalogItem) => void;
  onSubmit: (kind: NamedKind, event: FormEvent<HTMLFormElement>) => Promise<void>;
  status: 'loading' | 'idle' | 'saving';
  title: string;
}) {
  return (
    <form className="panel named-catalog-panel" onSubmit={(event) => void onSubmit(kind, event)}>
      <div className="panel-header">
        <div>
          <h2>{title}</h2>
          <p>Короткий справочник для карточек задач</p>
        </div>
        <Icon size={20} aria-hidden="true" />
      </div>

      <div className="inline-form">
        <input onChange={(event) => onChange(event.target.value)} placeholder="Название" required value={name} />
        <button className="primary-button" disabled={status === 'saving'} type="submit" aria-label="Сохранить">
          <Plus size={18} />
        </button>
      </div>

      <div className="chip-list">
        {items.map((item) => (
          <div className="catalog-chip" key={item.id}>
            <span>{item.name}</span>
            <button
              className="icon-button"
              disabled={status === 'saving'}
              onClick={() => onEdit(kind, item)}
              title="Редактировать"
              type="button"
              aria-label="Редактировать"
            >
              <Edit3 size={16} />
            </button>
            <button
              className="icon-button danger"
              disabled={status === 'saving'}
              onClick={() => void onDelete(kind, item)}
              title="Удалить"
              type="button"
              aria-label="Удалить"
            >
              <Trash2 size={16} />
            </button>
          </div>
        ))}
      </div>
    </form>
  );
}

function normalizedLevels(levels: CompetencyForm['levels']): CompetencyLevel[] {
  return levels
    .filter((item) => item.title.trim())
    .map((item) => ({
      level: item.level,
      title: item.title.trim(),
      description: item.description.trim() || undefined,
    }));
}

function updateLevel(
  index: number,
  field: 'title' | 'description',
  value: string,
  setCompetencyForm: Dispatch<SetStateAction<CompetencyForm>>,
) {
  setCompetencyForm((current) => ({
    ...current,
    levels: current.levels.map((level, candidateIndex) =>
      candidateIndex === index ? { ...level, [field]: value } : level,
    ),
  }));
}
