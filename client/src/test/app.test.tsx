import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AuthProvider } from '../auth';

// Minimal App routing test -- just checks the login page renders when unauthenticated
function TestApp({ route = '/' }: { route?: string }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  // Lazy import to avoid full app mount issues
  return (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[route]}>
        <AuthProvider>
          <TestRoutes />
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

// Simplified routes for testing
import { Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from '../auth';

function TestRoutes() {
  const { isAuthenticated } = useAuth();
  return (
    <Routes>
      <Route path="/login" element={<div data-testid="login-page">Login</div>} />
      <Route path="/" element={isAuthenticated ? <div data-testid="dashboard">Dashboard</div> : <Navigate to="/login" replace />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

describe('App routing', () => {
  beforeEach(() => localStorage.clear());

  it('redirects to login when unauthenticated', () => {
    render(<TestApp route="/" />);
    expect(screen.getByTestId('login-page')).toBeInTheDocument();
  });

  it('shows dashboard when authenticated', () => {
    localStorage.setItem('token', 'test-token');
    localStorage.setItem('user', JSON.stringify({ id: 1, email: 'a@b.com', firstName: 'A', lastName: 'B', role: 'admin', active: true, createdAt: '', updatedAt: '' }));
    render(<TestApp route="/" />);
    expect(screen.getByTestId('dashboard')).toBeInTheDocument();
  });

  it('redirects unknown routes to login when unauthenticated', () => {
    render(<TestApp route="/unknown" />);
    expect(screen.getByTestId('login-page')).toBeInTheDocument();
  });
});
