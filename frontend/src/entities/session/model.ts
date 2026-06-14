import { create } from 'zustand';
import { getCurrentUser, login, logout, refreshSession, type User } from '../../shared/api/auth';
import { setAccessToken } from '../../shared/api/client';

type SessionStatus = 'checking' | 'authenticated' | 'anonymous';

type SessionState = {
  status: SessionStatus;
  user: User | null;
  error: string | null;
  bootstrap: () => Promise<void>;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  setUser: (user: User) => void;
};

export const useSessionStore = create<SessionState>((set) => ({
  status: 'checking',
  user: null,
  error: null,

  async bootstrap() {
    try {
      const session = await refreshSession();
      setAccessToken(session.access_token);
      set({ status: 'authenticated', user: session.user, error: null });
    } catch {
      setAccessToken(null);
      set({ status: 'anonymous', user: null, error: null });
    }
  },

  async login(email, password) {
    set({ error: null });
    try {
      const session = await login(email, password);
      setAccessToken(session.access_token);
      set({ status: 'authenticated', user: session.user, error: null });
    } catch {
      setAccessToken(null);
      set({ status: 'anonymous', user: null, error: 'Неверный email или пароль' });
      throw new Error('login_failed');
    }
  },

  async logout() {
    try {
      await logout();
    } finally {
      setAccessToken(null);
      set({ status: 'anonymous', user: null, error: null });
    }
  },

  setUser(user) {
    set({ user });
  },
}));
