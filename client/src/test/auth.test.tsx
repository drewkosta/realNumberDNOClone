import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { AuthProvider, useAuth } from '../auth';

function TestConsumer() {
  const { user, isAuthenticated, login, logout } = useAuth();
  return (
    <div>
      <span data-testid="auth">{isAuthenticated ? 'yes' : 'no'}</span>
      <span data-testid="user">{user?.email ?? 'none'}</span>
      <button onClick={() => login('tok', 'ref', { id: 1, email: 'a@b.com', firstName: 'A', lastName: 'B', role: 'admin', active: true, createdAt: '', updatedAt: '' })}>
        login
      </button>
      <button onClick={logout}>logout</button>
    </div>
  );
}

describe('AuthContext', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('starts unauthenticated', () => {
    render(<AuthProvider><TestConsumer /></AuthProvider>);
    expect(screen.getByTestId('auth').textContent).toBe('no');
    expect(screen.getByTestId('user').textContent).toBe('none');
  });

  it('login sets user and token', () => {
    render(<AuthProvider><TestConsumer /></AuthProvider>);
    act(() => screen.getByText('login').click());
    expect(screen.getByTestId('auth').textContent).toBe('yes');
    expect(screen.getByTestId('user').textContent).toBe('a@b.com');
    expect(localStorage.getItem('token')).toBe('tok');
    expect(localStorage.getItem('refreshToken')).toBe('ref');
  });

  it('logout clears state', () => {
    render(<AuthProvider><TestConsumer /></AuthProvider>);
    act(() => screen.getByText('login').click());
    act(() => screen.getByText('logout').click());
    expect(screen.getByTestId('auth').textContent).toBe('no');
    expect(localStorage.getItem('token')).toBeNull();
    expect(localStorage.getItem('refreshToken')).toBeNull();
  });

  it('restores from localStorage', () => {
    localStorage.setItem('token', 'saved');
    localStorage.setItem('user', JSON.stringify({ id: 1, email: 'saved@test.com', firstName: 'S', lastName: 'T', role: 'viewer', active: true, createdAt: '', updatedAt: '' }));
    render(<AuthProvider><TestConsumer /></AuthProvider>);
    expect(screen.getByTestId('auth').textContent).toBe('yes');
    expect(screen.getByTestId('user').textContent).toBe('saved@test.com');
  });
});
