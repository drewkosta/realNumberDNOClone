import { useQuery } from '@tanstack/react-query';
import { complianceApi } from '../api';
import { ShieldCheck, AlertTriangle, XCircle, CheckCircle, FileText, Download } from 'lucide-react';

const statusConfig = {
  compliant: { icon: CheckCircle, color: 'text-green-600', bg: 'bg-green-50 border-green-200', label: 'Compliant' },
  at_risk: { icon: AlertTriangle, color: 'text-amber-600', bg: 'bg-amber-50 border-amber-200', label: 'At Risk' },
  non_compliant: { icon: XCircle, color: 'text-red-600', bg: 'bg-red-50 border-red-200', label: 'Non-Compliant' },
};

export default function CompliancePage() {
  const { data: report, isLoading } = useQuery({
    queryKey: ['compliance-report'],
    queryFn: complianceApi.getReport,
  });

  if (isLoading) {
    return (
      <div className="animate-fade-in">
        <div className="mb-8"><div className="skeleton h-8 w-48 mb-2" /><div className="skeleton h-5 w-80" /></div>
        <div className="skeleton h-40 w-full mb-6 rounded-xl" />
        <div className="grid grid-cols-2 gap-6"><div className="skeleton h-60 rounded-xl" /><div className="skeleton h-60 rounded-xl" /></div>
      </div>
    );
  }

  if (!report) return null;

  const cfg = statusConfig[report.complianceStatus];
  const StatusIcon = cfg.icon;

  return (
    <div>
      <div className="mb-8 animate-fade-up">
        <h1 className="text-2xl font-bold text-gray-900">Compliance Report</h1>
        <p className="text-gray-500 mt-1">FCC Do Not Originate compliance assessment for RMD filings</p>
      </div>

      {/* Status banner */}
      <div className={`result-enter rounded-xl p-6 shadow-sm border mb-8 ${cfg.bg}`}>
        <div className="flex items-center gap-4">
          <StatusIcon className={`w-10 h-10 ${cfg.color}`} />
          <div>
            <p className={`text-xl font-bold ${cfg.color}`}>{cfg.label}</p>
            <p className="text-sm text-gray-600 mt-1">
              Generated {new Date(report.generatedAt).toLocaleString()}
              {report.orgName ? ` for ${report.orgName}` : ' (system-wide)'}
            </p>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        {/* Dataset Coverage */}
        <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-1">
          <div className="flex items-center gap-2 mb-4">
            <ShieldCheck className="w-5 h-5 text-blue-600" />
            <h2 className="text-lg font-semibold text-gray-900">Dataset Coverage</h2>
          </div>
          <div className="space-y-3">
            {['auto', 'subscriber', 'itg', 'tss_registry'].map((ds) => {
              const count = report.datasetCoverage[ds] ?? 0;
              const active = count > 0;
              return (
                <div key={ds} className="flex items-center justify-between py-2 border-b border-gray-50">
                  <div className="flex items-center gap-2">
                    {active ? <CheckCircle className="w-4 h-4 text-green-500" /> : <XCircle className="w-4 h-4 text-gray-300" />}
                    <span className="text-sm font-medium text-gray-700">{ds.replace('_', ' ').toUpperCase()}</span>
                  </div>
                  <span className={`text-sm font-bold ${active ? 'text-gray-900' : 'text-gray-400'}`}>
                    {count.toLocaleString()} numbers
                  </span>
                </div>
              );
            })}
          </div>
          <p className="text-xs text-gray-400 mt-4">Total: {report.totalDnoNumbers.toLocaleString()} active DNO numbers</p>
        </div>

        {/* Enforcement Stats */}
        <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-2">
          <div className="flex items-center gap-2 mb-4">
            <FileText className="w-5 h-5 text-blue-600" />
            <h2 className="text-lg font-semibold text-gray-900">Enforcement (30 Days)</h2>
          </div>
          <div className="space-y-4">
            <div className="flex justify-between py-2 border-b border-gray-50">
              <span className="text-sm text-gray-500">Total Queries</span>
              <span className="text-sm font-bold text-gray-900">{report.last30DaysQueries.toLocaleString()}</span>
            </div>
            <div className="flex justify-between py-2 border-b border-gray-50">
              <span className="text-sm text-gray-500">Hit Rate</span>
              <span className="text-sm font-bold text-gray-900">{report.last30DaysHitRate.toFixed(1)}%</span>
            </div>
            <div className="flex justify-between py-2 border-b border-gray-50">
              <span className="text-sm text-gray-500">Calls Blocked</span>
              <span className="text-sm font-bold text-green-700">{report.last30DaysBlocked.toLocaleString()}</span>
            </div>
            <div className="flex justify-between py-2 border-b border-gray-50">
              <span className="text-sm text-gray-500">Update Frequency</span>
              <span className="text-sm text-gray-700">{report.updateFrequency}</span>
            </div>
            <div className="flex justify-between py-2">
              <span className="text-sm text-gray-500">Enforcement Method</span>
              <span className="text-sm text-gray-700 text-right max-w-[250px]">{report.enforcementMethod}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Recommendations */}
      <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 mb-6 animate-fade-up stagger-3">
        <h2 className="text-lg font-semibold text-gray-900 mb-4">Recommendations</h2>
        <div className="space-y-3">
          {report.recommendations.map((rec) => (
            <div key={rec} className="flex items-start gap-3 p-3 bg-gray-50 rounded-lg">
              <AlertTriangle className="w-4 h-4 text-amber-500 mt-0.5 shrink-0" />
              <p className="text-sm text-gray-700">{rec}</p>
            </div>
          ))}
        </div>
      </div>

      {/* Channel breakdown */}
      <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-4">
        <h2 className="text-lg font-semibold text-gray-900 mb-4">Channel Coverage</h2>
        <div className="flex gap-8 justify-center">
          {Object.entries(report.channelCoverage).map(([ch, count]) => (
            <div key={ch} className="text-center animate-count-up">
              <p className="text-3xl font-bold text-gray-900">{count.toLocaleString()}</p>
              <p className="text-sm text-gray-500">{ch.charAt(0).toUpperCase() + ch.slice(1)}</p>
            </div>
          ))}
        </div>
      </div>

      <div className="mt-6 text-center animate-fade-up stagger-5">
        <button
          onClick={() => {
            const blob = new Blob([JSON.stringify(report, null, 2)], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `compliance-report-${new Date().toISOString().split('T')[0]}.json`;
            a.click();
            URL.revokeObjectURL(url);
          }}
          className="inline-flex items-center gap-2 px-6 py-2.5 bg-slate-800 text-white rounded-lg font-medium hover:bg-slate-900"
        >
          <Download className="w-4 h-4" />
          Download Report (JSON)
        </button>
      </div>
    </div>
  );
}
