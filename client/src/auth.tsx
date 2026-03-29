import { createContext, use, useState, useCallback, type ReactNode } from 'react';
import type { User } from './types';

interface AuthContextType {
  user: User | null;
  token: string | null;
  login: (token: string, refreshToken: string, user: User) => void;
  logout: () => void;
  isAuthenticated: boolean;
}

const AuthContext = createContext<AuthContextType>(null!);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('token'));
  const [user, setUser] = useState<User | null>(() => {
    const stored = localStorage.getItem('user');
    return stored ? (JSON.parse(stored) as User) : null;
  });

  const login = useCallback((token: string, refreshToken: string, user: User) => {
    localStorage.setItem('token', token);
    localStorage.setItem('refreshToken', refreshToken);
    localStorage.setItem('user', JSON.stringify(user));
    setToken(token);
    setUser(user);
  }, []);

  const logout = useCallback(() => {
    localStorage.removeItem('token');
    localStorage.removeItem('refreshToken');
    localStorage.removeItem('user');
    setToken(null);
    setUser(null);
  }, []);

  return (
    <AuthContext value={{ user, token, login, logout, isAuthenticated: !!token }}>
      {children}
    </AuthContext>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth() {
  return use(AuthContext);
}
