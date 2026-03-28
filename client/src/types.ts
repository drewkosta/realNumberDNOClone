export interface User {
  id: number;
  email: string;
  firstName: string;
  lastName: string;
  role: 'admin' | 'org_admin' | 'operator' | 'viewer';
  orgId?: number;
  active: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

export interface DNONumber {
  id: number;
  phoneNumber: string;
  dataset: 'auto' | 'subscriber' | 'itg' | 'tss_registry';
  numberType: 'toll_free' | 'local';
  channel: 'voice' | 'text' | 'both';
  status: 'active' | 'inactive' | 'pending';
  reason?: string;
  addedByOrgId?: number;
  addedByUserId?: number;
  createdAt: string;
  updatedAt: string;
}

export interface DNOQueryResponse {
  phoneNumber: string;
  isDno: boolean;
  dataset?: string;
  channel: string;
  status?: string;
  lastUpdated?: string;
}

export interface BulkQueryResponse {
  results: DNOQueryResponse[];
  total: number;
  hits: number;
  misses: number;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
}

export interface AnalyticsSummary {
  totalDnoNumbers: number;
  activeNumbers: number;
  byDataset: Record<string, number>;
  byChannel: Record<string, number>;
  byNumberType: Record<string, number>;
  totalQueries24h: number;
  hitRate24h: number;
  queriesByHour: { hour: string; count: number }[];
  recentAdditions: number;
  recentRemovals: number;
}

export interface AuditLogEntry {
  id: number;
  userId?: number;
  orgId?: number;
  action: string;
  entityType: string;
  entityId?: number;
  details?: string;
  createdAt: string;
}

export interface BulkUploadResult {
  total: number;
  success: number;
  errors: number;
  details: string[];
}
