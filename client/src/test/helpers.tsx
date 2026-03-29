import { type ReactNode } from 'react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AuthProvider } from '../auth';
import type { User } from '../types';

const testUser: User = {
  id: 1, email: 'admin@test.com', firstName: 'Admin', lastName: 'User',
  role: 'admin', orgId: 1, active: true, createdAt: '2026-01-01', updatedAt: '2026-01-01',
};

export function loginAsUser(user: Partial<User> = {}) {
  const u = { ...testUser, ...user };
  localStorage.setItem('token', 'test-token');
  localStorage.setItem('refreshToken', 'test-refresh');
  localStorage.setItem('user', JSON.stringify(u));
}

export function createWrapper(route = '/') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={[route]}>
          <AuthProvider>{children}</AuthProvider>
        </MemoryRouter>
      </QueryClientProvider>
    );
  };
}
