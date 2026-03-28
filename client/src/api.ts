import axios from 'axios';
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
} from './types';

const api = axios.create({ baseURL: '/api' });

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      window.location.href = '/login';
    }
    return Promise.reject(err);
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
    api.delete('/dno/numbers', { params: { phoneNumber, channel } }).then((r) => r.data),

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
    api.get('/dno/export', { responseType: 'blob' }).then((r) => {
      const url = window.URL.createObjectURL(new Blob([r.data]));
      const a = document.createElement('a');
      a.href = url;
      a.download = 'dno_export.csv';
      a.click();
      window.URL.revokeObjectURL(url);
    }),
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
};
