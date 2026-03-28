import { useState } from 'react';
import axios from 'axios';
import { analyzerApi } from '../api';
import type { DNOAnalyzerReport } from '../types';
import { Scan, AlertTriangle, ShieldAlert, TrendingUp } from 'lucide-react';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts';

const COLORS = ['#ef4444', '#f59e0b', '#3b82f6', '#10b981', '#8b5cf6'];

export default function AnalyzerPage() {
  const [input, setInput] = useState('');
  const [channel, setChannel] = useState('voice');
  const [loading, setLoading] = useState(false);
  const [report, setReport] = useState<DNOAnalyzerReport | null>(null);
  const [error, setError] = useState('');

  const handleAnalyze = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setReport(null);
    setLoading(true);
    try {
      const records = input
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean)
        .map((callerId) => ({ callerId }));
      const result = await analyzerApi.analyze(records, channel);
      setReport(result);
    } catch (err: unknown) {
      setError(axios.isAxiosError(err) ? (err.response?.data as { error?: string })?.error ?? 'Analysis failed' : 'Analysis failed');
    } finally {
      setLoading(false);
    }
  };

  const datasetData = report ? Object.entries(report.byDataset).map(([name, value]) => ({ name, value })) : [];
  const threatData = report ? Object.entries(report.byThreatCategory).map(([name, value]) => ({ name: name.replace(/_/g, ' '), value })) : [];

  return (
    <div>
      <div className="mb-8 animate-fade-up">
        <h1 className="text-2xl font-bold text-gray-900">DNO Analyzer</h1>
        <p className="text-gray-500 mt-1">Analyze your traffic against the DNO database to assess fraud exposure</p>
      </div>

      <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 mb-6 animate-fade-up stagger-1">
        <form onSubmit={(e) => void handleAnalyze(e)} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Caller IDs (one per line, up to 100,000)
            </label>
            <textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              rows={8}
              className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none font-mono text-sm"
              placeholder={"5551234567\n8001234567\n2125559999"}
              required
            />
          </div>
          <div className="flex gap-4 items-end">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Channel</label>
              <select value={channel} onChange={(e) => setChannel(e.target.value)} className="px-3 py-2 border border-gray-300 rounded-lg text-sm">
                <option value="voice">Voice</option>
                <option value="text">Text</option>
              </select>
            </div>
            <button type="submit" disabled={loading} className="flex items-center gap-2 px-6 py-2.5 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 font-medium">
              <Scan className="w-4 h-4" />
              {loading ? 'Analyzing...' : 'Analyze Traffic'}
            </button>
          </div>
          {error && <div className="alert-enter bg-red-50 text-red-700 px-4 py-3 rounded-lg text-sm">{error}</div>}
        </form>
      </div>

      {report && (
        <div className="animate-fade-up">
          {/* Summary stats */}
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
            <div className="card-hover bg-white rounded-xl p-5 shadow-sm border border-gray-100 animate-fade-up stagger-1">
              <p className="text-sm text-gray-500">Records Analyzed</p>
              <p className="text-2xl font-bold text-gray-900 animate-count-up">{report.totalRecords.toLocaleString()}</p>
            </div>
            <div className="card-hover bg-red-50 rounded-xl p-5 shadow-sm border border-red-100 animate-fade-up stagger-2">
              <div className="flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 text-red-500" />
                <p className="text-sm text-red-600">DNO Hits (Spoofed)</p>
              </div>
              <p className="text-2xl font-bold text-red-700 animate-count-up">{report.dnoHits.toLocaleString()}</p>
            </div>
            <div className="card-hover bg-amber-50 rounded-xl p-5 shadow-sm border border-amber-100 animate-fade-up stagger-3">
              <div className="flex items-center gap-2">
                <ShieldAlert className="w-4 h-4 text-amber-500" />
                <p className="text-sm text-amber-600">Hit Rate</p>
              </div>
              <p className="text-2xl font-bold text-amber-700 animate-count-up">{report.hitRate.toFixed(1)}%</p>
            </div>
            <div className="card-hover bg-blue-50 rounded-xl p-5 shadow-sm border border-blue-100 animate-fade-up stagger-4">
              <div className="flex items-center gap-2">
                <TrendingUp className="w-4 h-4 text-blue-500" />
                <p className="text-sm text-blue-600">Est. Blocked/Day</p>
              </div>
              <p className="text-2xl font-bold text-blue-700 animate-count-up">{report.estBlockedPerDay.toLocaleString()}</p>
            </div>
          </div>

          {/* Charts */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
            {datasetData.length > 0 && (
              <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-4">
                <h3 className="text-lg font-semibold text-gray-900 mb-4">Hits by Dataset</h3>
                <ResponsiveContainer width="100%" height={250}>
                  <PieChart>
                    <Pie data={datasetData} cx="50%" cy="50%" outerRadius={80} innerRadius={40} dataKey="value"
                      label={({ name, percent }: { name?: string; percent?: number }) => `${name ?? ''} ${((percent ?? 0) * 100).toFixed(0)}%`}>
                      {datasetData.map((entry, i) => <Cell key={entry.name} fill={COLORS[i % COLORS.length]} />)}
                    </Pie>
                    <Tooltip />
                  </PieChart>
                </ResponsiveContainer>
              </div>
            )}
            {threatData.length > 0 && (
              <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-5">
                <h3 className="text-lg font-semibold text-gray-900 mb-4">Hits by Threat Category</h3>
                <ResponsiveContainer width="100%" height={250}>
                  <BarChart data={threatData} layout="vertical">
                    <XAxis type="number" fontSize={12} />
                    <YAxis type="category" dataKey="name" fontSize={11} width={120} />
                    <Tooltip />
                    <Bar dataKey="value" fill="#ef4444" radius={[0, 6, 6, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            )}
          </div>

          {/* Top spoofed numbers */}
          {report.topSpoofed.length > 0 && (
            <div className="card-hover bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden animate-fade-up stagger-5">
              <div className="px-6 py-4 border-b border-gray-100">
                <h3 className="text-lg font-semibold text-gray-900">Top Spoofed Numbers</h3>
              </div>
              <table className="w-full text-sm">
                <thead className="bg-gray-50 border-b border-gray-100">
                  <tr>
                    <th className="text-left px-4 py-3 font-medium text-gray-600">Phone Number</th>
                    <th className="text-left px-4 py-3 font-medium text-gray-600">Dataset</th>
                    <th className="text-left px-4 py-3 font-medium text-gray-600">Threat Category</th>
                    <th className="text-left px-4 py-3 font-medium text-gray-600">Occurrences</th>
                  </tr>
                </thead>
                <tbody>
                  {report.topSpoofed.map((n) => (
                    <tr key={n.phoneNumber} className="border-b border-gray-50 row-hover">
                      <td className="px-4 py-3 font-mono">{n.phoneNumber}</td>
                      <td className="px-4 py-3"><span className="px-2 py-1 rounded-full text-xs font-medium bg-red-100 text-red-700">{n.dataset}</span></td>
                      <td className="px-4 py-3 text-gray-600">{n.threatCategory?.replace(/_/g, ' ') ?? '-'}</td>
                      <td className="px-4 py-3 font-medium">{n.count}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
