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
  refreshToken: string;
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
  investigationId?: string;
  threatCategory?: string;
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

// ── New feature types ───────────────────────────────────────

export interface DNOAnalyzerReport {
  totalRecords: number;
  dnoHits: number;
  dnoMisses: number;
  hitRate: number;
  byDataset: Record<string, number>;
  byThreatCategory: Record<string, number>;
  topSpoofed: SpoofedNumber[];
  estBlockedPerDay: number;
}

export interface SpoofedNumber {
  phoneNumber: string;
  dataset: string;
  threatCategory?: string;
  count: number;
}

export interface ComplianceReport {
  generatedAt: string;
  orgName?: string;
  totalDnoNumbers: number;
  datasetCoverage: Record<string, number>;
  channelCoverage: Record<string, number>;
  updateFrequency: string;
  enforcementMethod: string;
  last30DaysQueries: number;
  last30DaysHitRate: number;
  last30DaysBlocked: number;
  complianceStatus: 'compliant' | 'at_risk' | 'non_compliant';
  recommendations: string[];
}

export interface ROICalculation {
  dailyCallVolume: number;
  estHitRate: number;
  estDailyBlocked: number;
  estMonthlyBlocked: number;
  estAnnualBlocked: number;
  avgComplaintCost: number;
  estAnnualSavings: number;
  complianceRiskLevel: string;
}

export interface WebhookSubscription {
  id: number;
  orgId: number;
  url: string;
  events: string;
  active: boolean;
  createdAt: string;
}

export interface OwnershipValidation {
  phoneNumber: string;
  valid: boolean;
  reason: string;
}
