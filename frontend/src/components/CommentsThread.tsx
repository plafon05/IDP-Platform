import { Edit3, Send, Trash2, X } from 'lucide-react';
import { FormEvent, useEffect, useState } from 'react';
import Markdown from 'react-markdown';
import { createComment, deleteComment, listComments, updateComment, type Comment } from '../shared/api/comments';

type Props = { entityType: 'idp' | 'task'; entityID: string; title?: string };

export function CommentsThread({ entityType, entityID, title = 'Комментарии' }: Props) {
  const [comments, setComments] = useState<Comment[]>([]);
  const [content, setContent] = useState('');
  const [editing, setEditing] = useState<Comment | null>(null);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setBusy(true);
    setError(null);
    try { setComments(await listComments(entityType, entityID)); }
    catch { setError('Не удалось загрузить комментарии'); }
    finally { setBusy(false); }
  }

  useEffect(() => { void load(); }, [entityType, entityID]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    const value = content.trim();
    if (!value) return;
    setBusy(true);
    setError(null);
    try {
      if (editing) await updateComment(editing.id, value);
      else await createComment(entityType, entityID, value);
      setContent('');
      setEditing(null);
      await load();
    } catch { setError(editing ? 'Не удалось изменить комментарий. Возможно, прошло больше 10 минут.' : 'Не удалось добавить комментарий'); setBusy(false); }
  }

  function startEdit(comment: Comment) { setEditing(comment); setContent(comment.content); }
  function cancelEdit() { setEditing(null); setContent(''); }

  async function remove(comment: Comment) {
    if (!window.confirm('Удалить комментарий?')) return;
    setBusy(true);
    try { await deleteComment(comment.id); await load(); }
    catch { setError('Не удалось удалить комментарий'); setBusy(false); }
  }

  return <section className="comments-thread">
    <div className="comments-heading"><strong>{title}</strong><span>{comments.length}</span></div>
    {error && <div className="form-error">{error}</div>}
    <div className="comment-list" aria-busy={busy}>
      {!busy && comments.length === 0 && <span className="task-empty">Комментариев пока нет</span>}
      {comments.map((comment) => <article className={`comment-item ${comment.is_deleted ? 'deleted' : ''}`} key={comment.id}>
        <div className="comment-avatar">
          {comment.author_avatar ? <img alt="" src={comment.author_avatar} /> : initials(comment.author_name)}
        </div>
        <div className="comment-body">
          <div className="comment-meta"><strong>{comment.author_name}</strong><time dateTime={comment.created_at}>{formatDateTime(comment.created_at)}</time>{comment.updated_at !== comment.created_at && !comment.is_deleted && <span>изменён</span>}</div>
          <div className="comment-content"><Markdown components={{ a: ({ children, ...props }) => <a {...props} target="_blank" rel="noreferrer">{children}</a> }}>{comment.content}</Markdown></div>
        </div>
        {(comment.can_edit || comment.can_delete) && <div className="comment-actions">
          {comment.can_edit && <button className="icon-button" type="button" title="Редактировать" aria-label="Редактировать" onClick={() => startEdit(comment)}><Edit3 size={15} /></button>}
          {comment.can_delete && <button className="icon-button danger" type="button" title="Удалить" aria-label="Удалить" onClick={() => void remove(comment)}><Trash2 size={15} /></button>}
        </div>}
      </article>)}
    </div>
    <form className="comment-form" onSubmit={submit}>
      <textarea maxLength={5000} placeholder={editing ? 'Измените комментарий' : 'Напишите комментарий'} required value={content} onChange={(event) => setContent(event.target.value)} />
      <button className="primary-button" disabled={busy || !content.trim()} type="submit"><Send size={17} />{editing ? 'Сохранить' : 'Отправить'}</button>
      {editing && <button className="secondary-button" type="button" onClick={cancelEdit}><X size={17} />Отмена</button>}
    </form>
  </section>;
}

function initials(name: string) { return name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]).join('').toUpperCase(); }
function formatDateTime(value: string) { return new Intl.DateTimeFormat('ru-RU', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)); }
