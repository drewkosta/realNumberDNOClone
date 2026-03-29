import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import ErrorBoundary from '../components/ErrorBoundary';
import { loginAsUser, createWrapper } from './helpers';

function ThrowingComponent(): never {
  throw new Error('Test explosion');
}

function GoodComponent() {
  return <div data-testid="good">Hello</div>;
}

describe('ErrorBoundary', () => {
  it('renders children when no error', () => {
    render(
      <ErrorBoundary>
        <GoodComponent />
      </ErrorBoundary>
    );
    expect(screen.getByTestId('good').textContent).toBe('Hello');
  });

  it('renders error UI when child throws', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {});
    render(
      <ErrorBoundary>
        <ThrowingComponent />
      </ErrorBoundary>
    );
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    expect(screen.getByText('Test explosion')).toBeInTheDocument();
    expect(screen.getByText('Reload Page')).toBeInTheDocument();
    spy.mockRestore();
  });
});

describe('Layout', () => {
  beforeEach(() => localStorage.clear());

  it('renders sidebar with nav items for admin', async () => {
    loginAsUser({ role: 'admin' });
    const { default: Layout } = await import('../components/Layout');
    render(
      <div>
        <Layout />
      </div>,
      { wrapper: createWrapper() },
    );
    expect(screen.getByText('FakeNumber')).toBeInTheDocument();
    expect(screen.getByText('Dashboard')).toBeInTheDocument();
    expect(screen.getByText('Query Numbers')).toBeInTheDocument();
    expect(screen.getByText('DNO List')).toBeInTheDocument();
    expect(screen.getByText('Admin')).toBeInTheDocument();
  });

  it('hides admin nav for viewer', async () => {
    loginAsUser({ role: 'viewer' });
    const { default: Layout } = await import('../components/Layout');
    render(<Layout />, { wrapper: createWrapper() });
    expect(screen.queryByText('Admin')).not.toBeInTheDocument();
    expect(screen.queryByText('Webhooks')).not.toBeInTheDocument();
    expect(screen.queryByText('Bulk Operations')).not.toBeInTheDocument();
  });

  it('hides webhooks for operator', async () => {
    loginAsUser({ role: 'operator' });
    const { default: Layout } = await import('../components/Layout');
    render(<Layout />, { wrapper: createWrapper() });
    expect(screen.getByText('Bulk Operations')).toBeInTheDocument();
    expect(screen.queryByText('Webhooks')).not.toBeInTheDocument();
    expect(screen.queryByText('Admin')).not.toBeInTheDocument();
  });

  it('shows user name in sidebar', async () => {
    loginAsUser({ firstName: 'John', lastName: 'Doe' });
    const { default: Layout } = await import('../components/Layout');
    render(<Layout />, { wrapper: createWrapper() });
    expect(screen.getByText('John Doe')).toBeInTheDocument();
  });
});

describe('ServiceStatus', () => {
  it('renders nothing when online and backend healthy', async () => {
    // Mock fetch for /health
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{"status":"ok"}', { status: 200 }));
    const { default: ServiceStatus } = await import('../components/ServiceStatus');
    const { container } = render(<ServiceStatus />);
    await waitFor(() => {
      expect(container.textContent).toBe('');
    });
    fetchSpy.mockRestore();
  });
});
