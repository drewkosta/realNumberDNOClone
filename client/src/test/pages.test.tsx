import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { loginAsUser, createWrapper } from './helpers';

// Mock all API modules
vi.mock('../api', () => ({
  authApi: {
    login: vi.fn().mockResolvedValue({
      token: 'tok', refreshToken: 'ref',
      user: { id: 1, email: 'admin@test.com', firstName: 'A', lastName: 'U', role: 'admin', active: true, createdAt: '', updatedAt: '' },
    }),
    me: vi.fn().mockResolvedValue({ id: 1, email: 'admin@test.com', firstName: 'A', lastName: 'U', role: 'admin', active: true }),
  },
  dnoApi: {
    query: vi.fn().mockResolvedValue({ phoneNumber: '5551234567', isDno: false, channel: 'voice' }),
    bulkQuery: vi.fn().mockResolvedValue({ results: [], total: 0, hits: 0, misses: 0 }),
    addNumber: vi.fn().mockResolvedValue({ id: 1, phoneNumber: '5551234567', dataset: 'subscriber', status: 'active' }),
    removeNumber: vi.fn().mockResolvedValue({ message: 'removed' }),
    listNumbers: vi.fn().mockResolvedValue({ data: [], total: 0, page: 1, pageSize: 25, totalPages: 0 }),
    bulkUpload: vi.fn().mockResolvedValue({ total: 0, success: 0, errors: 0, details: [] }),
    exportCSV: vi.fn().mockResolvedValue(undefined),
    validateOwnership: vi.fn().mockResolvedValue({ phoneNumber: '5551234567', valid: true, reason: 'ok' }),
  },
  analyticsApi: {
    getSummary: vi.fn().mockResolvedValue({
      totalDnoNumbers: 1300, activeNumbers: 1300,
      byDataset: { auto: 800, subscriber: 300, itg: 50, tss_registry: 150 },
      byChannel: { voice: 1100, text: 150, both: 50 },
      byNumberType: { local: 750, toll_free: 550 },
      totalQueries24h: 1005, hitRate24h: 15.2,
      queriesByHour: [{ hour: '2026-03-28 14:00', count: 42 }],
      recentAdditions: 12, recentRemovals: 3,
    }),
  },
  auditApi: {
    getLog: vi.fn().mockResolvedValue({
      data: [{ id: 1, action: 'add', entityType: 'dno_number', createdAt: '2026-01-01T00:00:00Z' }],
      total: 1, page: 1, pageSize: 25, totalPages: 1,
    }),
  },
  adminApi: {
    createUser: vi.fn().mockResolvedValue({ id: 2, email: 'new@test.com', role: 'viewer' }),
    generateApiKey: vi.fn().mockResolvedValue({ orgId: 1, apiKey: 'dno_test123', note: 'Store securely' }),
    revokeApiKey: vi.fn().mockResolvedValue({ message: 'revoked' }),
    ingestITG: vi.fn().mockResolvedValue({ id: 1, dataset: 'itg', phoneNumber: '5551234567' }),
    npacEvent: vi.fn().mockResolvedValue({ message: 'processed' }),
    tssSync: vi.fn().mockResolvedValue({ message: 'synced', numbersAdded: 5 }),
  },
  analyzerApi: {
    analyze: vi.fn().mockResolvedValue({
      totalRecords: 3, dnoHits: 1, dnoMisses: 2, hitRate: 33.3,
      byDataset: { itg: 1 }, byThreatCategory: {}, topSpoofed: [], estBlockedPerDay: 1,
    }),
  },
  complianceApi: {
    getReport: vi.fn().mockResolvedValue({
      generatedAt: '2026-01-01', complianceStatus: 'compliant', totalDnoNumbers: 1300,
      datasetCoverage: { auto: 800, subscriber: 300, itg: 50, tss_registry: 150 },
      channelCoverage: { voice: 1100, text: 150 },
      updateFrequency: 'Near real-time', enforcementMethod: 'API query',
      last30DaysQueries: 5000, last30DaysHitRate: 15.2, last30DaysBlocked: 760,
      recommendations: ['All checks passed.'],
    }),
  },
  roiApi: {
    calculate: vi.fn().mockResolvedValue({
      dailyCallVolume: 50000, estHitRate: 17, estDailyBlocked: 8500,
      estMonthlyBlocked: 255000, estAnnualBlocked: 3102500,
      avgComplaintCost: 4, estAnnualSavings: 12410000, complianceRiskLevel: 'medium',
    }),
  },
  webhookApi: {
    create: vi.fn().mockResolvedValue({ id: 1, url: 'https://example.com', events: 'dno.added', active: true, createdAt: '2026-01-01' }),
    list: vi.fn().mockResolvedValue([]),
    remove: vi.fn().mockResolvedValue({ message: 'deleted' }),
  },
}));

beforeEach(() => {
  localStorage.clear();
  vi.clearAllMocks();
});

// ── LoginPage ──────────────────────────────────────────────────────────────

describe('LoginPage', () => {
  it('renders login form', async () => {
    const { default: LoginPage } = await import('../pages/LoginPage');
    render(<LoginPage />, { wrapper: createWrapper('/login') });
    expect(screen.getByText('FakeNumber DNO')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('admin@realnumber.local')).toBeInTheDocument();
    expect(screen.getByText('Sign In')).toBeInTheDocument();
  });

  it('shows demo credentials', async () => {
    const { default: LoginPage } = await import('../pages/LoginPage');
    render(<LoginPage />, { wrapper: createWrapper('/login') });
    expect(screen.getByText(/admin@realnumber.local/)).toBeInTheDocument();
    expect(screen.getAllByText(/viewer/).length).toBeGreaterThan(0);
  });
});

// ── DashboardPage ──────────────────────────────────────────────────────────

describe('DashboardPage', () => {
  it('renders analytics data', async () => {
    loginAsUser();
    const { default: DashboardPage } = await import('../pages/DashboardPage');
    render(<DashboardPage />, { wrapper: createWrapper() });
    await waitFor(() => {
      expect(screen.getByText('Dashboard')).toBeInTheDocument();
      expect(screen.getAllByText('1,300').length).toBeGreaterThan(0);
    });
  });
});

// ── QueryPage ──────────────────────────────────────────────────────────────

describe('QueryPage', () => {
  it('renders query form with single/bulk toggle', async () => {
    loginAsUser();
    const { default: QueryPage } = await import('../pages/QueryPage');
    render(<QueryPage />, { wrapper: createWrapper() });
    expect(screen.getByText('Query DNO Database')).toBeInTheDocument();
    expect(screen.getByText('Single Query')).toBeInTheDocument();
    expect(screen.getByText('Bulk Query')).toBeInTheDocument();
  });
});

// ── NumbersPage ────────────────────────────────────────────────────────────

describe('NumbersPage', () => {
  it('renders number list', async () => {
    loginAsUser();
    const { default: NumbersPage } = await import('../pages/NumbersPage');
    render(<NumbersPage />, { wrapper: createWrapper() });
    await waitFor(() => {
      expect(screen.getByText('DNO Number List')).toBeInTheDocument();
    });
  });

  it('hides add button for viewer role', async () => {
    loginAsUser({ role: 'viewer' });
    const { default: NumbersPage } = await import('../pages/NumbersPage');
    render(<NumbersPage />, { wrapper: createWrapper() });
    await waitFor(() => {
      expect(screen.queryByText('Add Number')).not.toBeInTheDocument();
    });
  });

  it('shows add button for operator role', async () => {
    loginAsUser({ role: 'operator' });
    const { default: NumbersPage } = await import('../pages/NumbersPage');
    render(<NumbersPage />, { wrapper: createWrapper() });
    await waitFor(() => {
      expect(screen.getByText('Add Number')).toBeInTheDocument();
    });
  });
});

// ── BulkPage ───────────────────────────────────────────────────────────────

describe('BulkPage', () => {
  it('renders upload and export sections', async () => {
    loginAsUser();
    const { default: BulkPage } = await import('../pages/BulkPage');
    render(<BulkPage />, { wrapper: createWrapper() });
    expect(screen.getByText('Bulk Upload')).toBeInTheDocument();
    expect(screen.getByText('Flat File Export')).toBeInTheDocument();
  });
});

// ── AnalyzerPage ───────────────────────────────────────────────────────────

describe('AnalyzerPage', () => {
  it('renders analyzer form', async () => {
    loginAsUser();
    const { default: AnalyzerPage } = await import('../pages/AnalyzerPage');
    render(<AnalyzerPage />, { wrapper: createWrapper() });
    expect(screen.getByText('DNO Analyzer')).toBeInTheDocument();
    expect(screen.getByText('Analyze Traffic')).toBeInTheDocument();
  });
});

// ── CompliancePage ─────────────────────────────────────────────────────────

describe('CompliancePage', () => {
  it('renders compliance report', async () => {
    loginAsUser();
    const { default: CompliancePage } = await import('../pages/CompliancePage');
    render(<CompliancePage />, { wrapper: createWrapper() });
    await waitFor(() => {
      expect(screen.getByText('Compliance Report')).toBeInTheDocument();
      expect(screen.getByText('Compliant')).toBeInTheDocument();
    });
  });
});

// ── WebhooksPage ───────────────────────────────────────────────────────────

describe('WebhooksPage', () => {
  it('renders webhooks page with add button', async () => {
    loginAsUser();
    const { default: WebhooksPage } = await import('../pages/WebhooksPage');
    render(<WebhooksPage />, { wrapper: createWrapper() });
    await waitFor(() => {
      expect(screen.getByText('Webhooks')).toBeInTheDocument();
      expect(screen.getByText('Add Webhook')).toBeInTheDocument();
    });
  });
});

// ── ROIPage ────────────────────────────────────────────────────────────────

describe('ROIPage', () => {
  it('renders calculator with slider', async () => {
    loginAsUser();
    const { default: ROIPage } = await import('../pages/ROIPage');
    render(<ROIPage />, { wrapper: createWrapper() });
    expect(screen.getByText('ROI Calculator')).toBeInTheDocument();
    expect(screen.getByText('Calculate ROI')).toBeInTheDocument();
  });

  it('shows results after calculation', async () => {
    loginAsUser();
    const { default: ROIPage } = await import('../pages/ROIPage');
    render(<ROIPage />, { wrapper: createWrapper() });
    const user = userEvent.setup();
    await user.click(screen.getByText('Calculate ROI'));
    await waitFor(() => {
      expect(screen.getByText('8,500')).toBeInTheDocument(); // daily blocked
    });
  });
});

// ── AuditPage ──────────────────────────────────────────────────────────────

describe('AuditPage', () => {
  it('renders audit log', async () => {
    loginAsUser();
    const { default: AuditPage } = await import('../pages/AuditPage');
    render(<AuditPage />, { wrapper: createWrapper() });
    await waitFor(() => {
      expect(screen.getByText('Audit Log')).toBeInTheDocument();
    });
  });
});

// ── AdminPage ──────────────────────────────────────────────────────────────

describe('AdminPage', () => {
  it('renders admin panel for admin role', async () => {
    loginAsUser({ role: 'admin' });
    const { default: AdminPage } = await import('../pages/AdminPage');
    render(<AdminPage />, { wrapper: createWrapper() });
    expect(screen.getByText('Administration')).toBeInTheDocument();
    expect(screen.getAllByText('Create User').length).toBeGreaterThan(0);
    expect(screen.getByText('API Keys')).toBeInTheDocument();
    expect(screen.getByText('ITG Traceback Ingest')).toBeInTheDocument();
  });

  it('shows access denied for non-admin', async () => {
    loginAsUser({ role: 'viewer' });
    const { default: AdminPage } = await import('../pages/AdminPage');
    render(<AdminPage />, { wrapper: createWrapper() });
    expect(screen.getByText('Admin access required')).toBeInTheDocument();
  });
});
