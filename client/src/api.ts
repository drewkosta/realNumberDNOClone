import axios, { isAxiosError } from 'axios';
import type {
  LoginResponse,
  User,
  DNOQueryResponse,
  BulkQueryResponse,
  DNONumber,
  PaginatedResponse,
  AnalyticsSummary,
  AuditLogEntry,
  BulkUploadResult,
  DNOAnalyzerReport,
  ComplianceReport,
  ROICalculation,
  WebhookSubscription,
  OwnershipValidation,
} from './types';

const api = axios.create({ baseURL: '/api/v1' });

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

let isRefreshing = false;

api.interceptors.response.use(
  (res) => res,
  async (err: unknown) => {
    if (isAxiosError(err) && err.response?.status === 401 && err.config && !isRefreshing) {
      const refreshToken = localStorage.getItem('refreshToken');
      if (refreshToken) {
        isRefreshing = true;
        try {
          const resp = await axios.post<LoginResponse>('/api/v1/auth/refresh', { refreshToken });
          localStorage.setItem('token', resp.data.token);
          localStorage.setItem('refreshToken', resp.data.refreshToken);
          localStorage.setItem('user', JSON.stringify(resp.data.user));
          isRefreshing = false;
          // Retry the original request
          err.config.headers.Authorization = `Bearer ${resp.data.token}`;
          return api.request(err.config);
        } catch {
          isRefreshing = false;
        }
      }
      localStorage.removeItem('token');
      localStorage.removeItem('refreshToken');
      localStorage.removeItem('user');
      window.location.href = '/login';
    }
    return Promise.reject(err instanceof Error ? err : new Error(String(err)));
  }
);

export const authApi = {
  login: (email: string, password: string) =>
    api.post<LoginResponse>('/auth/login', { email, password }).then((r) => r.data),
  me: () => api.get<User>('/auth/me').then((r) => r.data),
};

export const dnoApi = {
  query: (phoneNumber: string, channel = 'voice') =>
    api.get<DNOQueryResponse>('/dno/query', { params: { phoneNumber, channel } }).then((r) => r.data),

  bulkQuery: (phoneNumbers: string[], channel = 'voice') =>
    api.post<BulkQueryResponse>('/dno/query/bulk', { phoneNumbers, channel }).then((r) => r.data),

  addNumber: (data: { phoneNumber: string; numberType: string; channel: string; reason?: string }) =>
    api.post<DNONumber>('/dno/numbers', data).then((r) => r.data),

  removeNumber: (phoneNumber: string, channel = 'voice') =>
    api.delete<{ message: string }>('/dno/numbers', { params: { phoneNumber, channel } }).then((r) => r.data),

  listNumbers: (params: {
    page?: number;
    pageSize?: number;
    dataset?: string;
    status?: string;
    channel?: string;
    search?: string;
  }) => api.get<PaginatedResponse<DNONumber>>('/dno/numbers', { params }).then((r) => r.data),

  bulkUpload: (file: File, channel: string, numberType: string) => {
    const form = new FormData();
    form.append('file', file);
    form.append('channel', channel);
    form.append('numberType', numberType);
    return api.post<BulkUploadResult>('/dno/bulk-upload', form).then((r) => r.data);
  },

  exportCSV: () =>
    api.get<Blob>('/dno/export', { responseType: 'blob' }).then((r) => {
      const url = window.URL.createObjectURL(new Blob([r.data]));
      const a = document.createElement('a');
      a.href = url;
      a.download = 'dno_export.csv';
      a.click();
      window.URL.revokeObjectURL(url);
    }),

  validateOwnership: (phoneNumber: string) =>
    api.get<OwnershipValidation>('/dno/validate-ownership', { params: { phoneNumber } }).then((r) => r.data),
};

export const analyticsApi = {
  getSummary: () => api.get<AnalyticsSummary>('/analytics').then((r) => r.data),
};

export const auditApi = {
  getLog: (page = 1, pageSize = 25) =>
    api.get<PaginatedResponse<AuditLogEntry>>('/audit-log', { params: { page, pageSize } }).then((r) => r.data),
};

export const adminApi = {
  createUser: (data: {
    email: string;
    password: string;
    firstName: string;
    lastName: string;
    role: string;
    orgId?: number;
  }) => api.post<User>('/admin/users', data).then((r) => r.data),

  generateApiKey: (orgId: number) =>
    api.post<{ orgId: number; apiKey: string; note: string }>('/admin/api-keys', null, { params: { orgId } }).then((r) => r.data),

  revokeApiKey: (orgId: number) =>
    api.delete<{ message: string }>('/admin/api-keys', { params: { orgId } }).then((r) => r.data),

  ingestITG: (data: { phoneNumber: string; investigationId: string; threatCategory: string; channel?: string }) =>
    api.post<DNONumber>('/admin/itg-ingest', data).then((r) => r.data),

  npacEvent: (data: { phoneNumber: string; newStatus: string; newOwnerOrgId?: number }) =>
    api.post<{ message: string }>('/admin/npac-event', data).then((r) => r.data),

  tssSync: () =>
    api.post<{ message: string; numbersAdded: number }>('/admin/tss-sync').then((r) => r.data),
};

export const analyzerApi = {
  analyze: (records: { callerId: string; timestamp?: string }[], channel = 'voice') =>
    api.post<DNOAnalyzerReport>('/analyzer', { records, channel }).then((r) => r.data),
};

export const complianceApi = {
  getReport: () => api.get<ComplianceReport>('/compliance-report').then((r) => r.data),
};

export const roiApi = {
  calculate: (dailyCallVolume: number) =>
    api.get<ROICalculation>('/roi-calculator', { params: { dailyCallVolume } }).then((r) => r.data),
};

export const webhookApi = {
  create: (data: { url: string; secret: string; events?: string }) =>
    api.post<WebhookSubscription>('/webhooks', data).then((r) => r.data),
  list: () => api.get<WebhookSubscription[]>('/webhooks').then((r) => r.data),
  remove: (id: number) =>
    api.delete<{ message: string }>('/webhooks', { params: { id } }).then((r) => r.data),
};
