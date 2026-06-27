import { Bold, Eye, Italic, Link, List, Pencil } from 'lucide-react';
import { useRef, useState } from 'react';
import Markdown from 'react-markdown';

type Props = {
  value: string;
  onChange: (value: string) => void;
  maxLength?: number;
};

export function MarkdownEditor({ value, onChange, maxLength = 10000 }: Props) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [preview, setPreview] = useState(false);

  function replaceSelection(before: string, after: string, placeholder: string) {
    const textarea = textareaRef.current;
    if (!textarea) return;
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const selected = value.slice(start, end) || placeholder;
    const next = `${value.slice(0, start)}${before}${selected}${after}${value.slice(end)}`;
    if (next.length > maxLength) return;
    onChange(next);
    requestAnimationFrame(() => {
      textarea.focus();
      textarea.setSelectionRange(start + before.length, start + before.length + selected.length);
    });
  }

  function makeList() {
    const textarea = textareaRef.current;
    if (!textarea) return;
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const selected = value.slice(start, end) || 'Пункт списка';
    const replacement = selected.split('\n').map((line) => `- ${line}`).join('\n');
    const next = `${value.slice(0, start)}${replacement}${value.slice(end)}`;
    if (next.length <= maxLength) onChange(next);
  }

  return <div className="markdown-editor">
    <div className="markdown-toolbar">
      <button className="icon-button" type="button" title="Жирный" aria-label="Жирный" onClick={() => replaceSelection('**', '**', 'текст')}><Bold size={16} /></button>
      <button className="icon-button" type="button" title="Курсив" aria-label="Курсив" onClick={() => replaceSelection('*', '*', 'текст')}><Italic size={16} /></button>
      <button className="icon-button" type="button" title="Список" aria-label="Список" onClick={makeList}><List size={16} /></button>
      <button className="icon-button" type="button" title="Ссылка" aria-label="Ссылка" onClick={() => replaceSelection('[', '](https://)', 'название')}><Link size={16} /></button>
      <span />
      <button className={`icon-button ${preview ? 'active' : ''}`} type="button" title={preview ? 'Редактор' : 'Предпросмотр'} aria-label={preview ? 'Редактор' : 'Предпросмотр'} onClick={() => setPreview((current) => !current)}>{preview ? <Pencil size={16} /> : <Eye size={16} />}</button>
    </div>
    {preview ? <div className="markdown-preview"><MarkdownContent value={value} /></div> : <textarea ref={textareaRef} maxLength={maxLength} value={value} onChange={(event) => onChange(event.target.value)} />}
  </div>;
}

export function MarkdownContent({ value }: { value: string }) {
  return <Markdown components={{ a: ({ children, ...props }) => <a {...props} target="_blank" rel="noreferrer">{children}</a> }}>{value}</Markdown>;
}
