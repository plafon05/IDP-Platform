import { BellOff, Check } from 'lucide-react';
import { useState } from 'react';
import { unsubscribeFromNotifications } from '../shared/api/notifications';

export function UnsubscribePage() {
  const token = new URLSearchParams(window.location.search).get('token') ?? '';
  const [status, setStatus] = useState<'idle' | 'saving' | 'done' | 'error'>('idle');

  async function unsubscribe() {
    setStatus('saving');
    try {
      await unsubscribeFromNotifications(token);
      setStatus('done');
    } catch {
      setStatus('error');
    }
  }

  return (
    <main className="login-screen">
      <section className="login-panel unsubscribe-panel">
        <div className="brand"><div className="brand-mark">IDP</div><strong>Platform</strong></div>
        {status === 'done' ? (
          <><Check size={32} /><h1>Уведомления отключены</h1><p>Сброс пароля по email продолжит работать.</p><a className="primary-button" href="/">Вернуться в систему</a></>
        ) : (
          <><BellOff size={32} /><h1>Отключить email-уведомления?</h1><p>Письма об ИПР, задачах, комментариях и сроках больше не будут приходить.</p>{status === 'error' && <div className="form-error">Ссылка недействительна</div>}<button className="primary-button" disabled={!token || status === 'saving'} onClick={() => void unsubscribe()} type="button">Отключить уведомления</button></>
        )}
      </section>
    </main>
  );
}
