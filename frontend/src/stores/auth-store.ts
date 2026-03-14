import { create } from 'zustand';
import { apiClient, setToken, getToken, clearToken } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { LoginResponse } from '@shared/index';

interface AuthState {
  token: string | null;
  isAuthenticated: boolean;
  login: (password: string, rememberMe?: boolean) => Promise<void>;
  logout: () => void;
  checkSession: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  token: getToken(),
  isAuthenticated: !!getToken(),

  login: async (password: string, rememberMe = true) => {
    try {
      await apiClient.post(API_ROUTES.system.timeSync, { client_time_ms: Date.now() });
    } catch {
      // best-effort clock sync — don't fail login if this fails
    }
    const response = await apiClient.post<LoginResponse>(API_ROUTES.auth.login, { password });
    setToken(response.token, rememberMe);
    set({ token: response.token, isAuthenticated: true });
  },

  logout: () => {
    clearToken();
    set({ token: null, isAuthenticated: false });
  },

  checkSession: () => {
    const token = getToken();
    set({ token, isAuthenticated: !!token });
  },
}));
