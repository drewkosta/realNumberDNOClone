import { describe, it, expect } from 'vitest';
import type { User, DNONumber, DNOQueryResponse, ComplianceReport, ROICalculation } from '../types';

describe('TypeScript types', () => {
  it('User type has expected shape', () => {
    const user: User = {
      id: 1, email: 'test@test.com', firstName: 'Test', lastName: 'User',
      role: 'admin', active: true, createdAt: '2026-01-01', updatedAt: '2026-01-01',
    };
    expect(user.role).toBe('admin');
    expect(user.orgId).toBeUndefined();
  });

  it('DNONumber type supports all datasets', () => {
    const datasets: DNONumber['dataset'][] = ['auto', 'subscriber', 'itg', 'tss_registry'];
    expect(datasets).toHaveLength(4);
  });

  it('DNONumber type supports all channels', () => {
    const channels: DNONumber['channel'][] = ['voice', 'text', 'both'];
    expect(channels).toHaveLength(3);
  });

  it('DNOQueryResponse isDno field', () => {
    const hit: DNOQueryResponse = { phoneNumber: '5551234567', isDno: true, channel: 'voice', dataset: 'subscriber' };
    const miss: DNOQueryResponse = { phoneNumber: '5551234567', isDno: false, channel: 'voice' };
    expect(hit.isDno).toBe(true);
    expect(miss.isDno).toBe(false);
  });

  it('ComplianceReport status types', () => {
    const statuses: ComplianceReport['complianceStatus'][] = ['compliant', 'at_risk', 'non_compliant'];
    expect(statuses).toHaveLength(3);
  });

  it('ROICalculation has expected fields', () => {
    const roi: ROICalculation = {
      dailyCallVolume: 50000, estHitRate: 17, estDailyBlocked: 8500,
      estMonthlyBlocked: 255000, estAnnualBlocked: 3102500,
      avgComplaintCost: 4, estAnnualSavings: 12410000, complianceRiskLevel: 'medium',
    };
    expect(roi.estAnnualSavings).toBeGreaterThan(0);
  });
});
