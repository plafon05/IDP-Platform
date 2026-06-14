import { KeyRound, Mail } from 'lucide-react';
import axios from 'axios';
import { FormEvent, useMemo, useState } from 'react';
import { forgotPassword, resetPassword, type ForgotPasswordResponse } from '../shared/api/auth';

export function ResetPasswordPage() {
  const token = useMemo(() => new URLSearchParams(window.location.search).get('token') ?? '', []);
  const [email, setEmail] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [status, setStatus] = useState<'idle' | 'saving' | 'sent' | 'done'>('idle');
  const [response, setResponse] = useState<ForgotPasswordResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function handleForgot(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setStatus('saving');
    setError(null);
    try {
      const result = await forgotPassword(email);
      setResponse(result);
      setStatus('sent');
    } catch {
      setError('Не удалось создать запрос восстановления');
      setStatus('idle');
    }
  }

  async function handleReset(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setStatus('saving');
    setError(null);
    try {
      await resetPassword(token, newPassword);
      setStatus('done');
    } catch (err) {
      setError(resetErrorMessage(err));
      setStatus('idle');
    }
  }

  return (
    <main className="login-screen">
      <section className="login-panel" aria-labelledby="reset-title">
        <div className="login-brand">
          <div className="brand-mark">IDP</div>
          <div>
            <strong>Platform</strong>
            <span>Individual development plans</span>
          </div>
        </div>

        {token ? (
          <form className="login-form" onSubmit={handleReset}>
            <div>
              <h1 id="reset-title">Новый пароль</h1>
            </div>
            <label className="form-field">
              <span>Новый пароль</span>
              <input
                autoComplete="new-password"
                onChange={(event) => setNewPassword(event.target.value)}
                required
                type="password"
                value={newPassword}
              />
            </label>
            {error && <div className="form-error">{error}</div>}
            {status === 'done' && (
              <div className="form-success">
                Пароль изменён
                <a href="/">Перейти ко входу</a>
              </div>
            )}
            {status !== 'done' && (
              <button className="primary-button" disabled={status === 'saving'} type="submit">
                <KeyRound size={18} />
                Сменить пароль
              </button>
            )}
          </form>
        ) : (
          <form className="login-form" onSubmit={handleForgot}>
            <div>
              <h1 id="reset-title">Восстановление</h1>
            </div>
            <label className="form-field">
              <span>Email</span>
              <input
                autoComplete="email"
                inputMode="email"
                onChange={(event) => setEmail(event.target.value)}
                required
                type="email"
                value={email}
              />
            </label>
            {error && <div className="form-error">{error}</div>}
            {status === 'sent' && (
              <div className="form-success">
                Запрос обработан
                {response?.dev_reset_url && (
                  <a href={response.dev_reset_url}>Открыть dev-ссылку восстановления</a>
                )}
              </div>
            )}
            <button className="primary-button" disabled={status === 'saving'} type="submit">
              <Mail size={18} />
              Отправить
            </button>
          </form>
        )}
      </section>
    </main>
  );
}

function resetErrorMessage(err: unknown) {
  if (axios.isAxiosError(err)) {
    const code = err.response?.data?.error?.code;
    if (code === 'SAME_PASSWORD') {
      return 'Новый пароль должен отличаться от старого';
    }
    if (code === 'WEAK_PASSWORD') {
      return 'Пароль должен быть не короче 8 символов, с заглавной буквой и цифрой';
    }
    if (code === 'INVALID_RESET_TOKEN') {
      return 'Ссылка недействительна или устарела';
    }
  }

  return 'Не удалось сменить пароль';
}
