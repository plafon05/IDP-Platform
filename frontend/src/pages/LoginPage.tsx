import { Eye, EyeOff, LogIn } from 'lucide-react';
import { FormEvent, useState } from 'react';
import { useSessionStore } from '../entities/session/model';

export function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const login = useSessionStore((state) => state.login);
  const error = useSessionStore((state) => state.error);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSubmitting(true);
    try {
      await login(email, password);
    } catch {
      // Error text is owned by the session store.
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <main className="login-screen">
      <section className="login-panel" aria-labelledby="login-title">
        <div className="login-brand">
          <div className="brand-mark">IDP</div>
          <div>
            <strong>Platform</strong>
            <span>Individual development plans</span>
          </div>
        </div>

        <form className="login-form" onSubmit={handleSubmit}>
          <div>
            <h1 id="login-title">Вход</h1>
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

          <label className="form-field">
            <span>Пароль</span>
            <div className="password-field">
              <input
                autoComplete="current-password"
                onChange={(event) => setPassword(event.target.value)}
                required
                type={showPassword ? 'text' : 'password'}
                value={password}
              />
              <button
                aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}
                onClick={() => setShowPassword((value) => !value)}
                type="button"
              >
                {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
              </button>
            </div>
          </label>

          {error && <div className="form-error">{error}</div>}

          <button className="primary-button" disabled={isSubmitting} type="submit">
            <LogIn size={18} />
            {isSubmitting ? 'Вход...' : 'Войти'}
          </button>

          <a className="form-link" href="/reset-password">
            Забыли пароль?
          </a>
        </form>
      </section>
    </main>
  );
}
