import { useState } from 'react';
import axios from 'axios';
import { dnoApi } from '../api';
import type { DNOQueryResponse, BulkQueryResponse } from '../types';
import { Search, Phone, CheckCircle, XCircle } from 'lucide-react';

export default function QueryPage() {
  const [mode, setMode] = useState<'single' | 'bulk'>('single');
  const [phone, setPhone] = useState('');
  const [channel, setChannel] = useState('voice');
  const [bulkInput, setBulkInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [singleResult, setSingleResult] = useState<DNOQueryResponse | null>(null);
  const [bulkResult, setBulkResult] = useState<BulkQueryResponse | null>(null);
  const [error, setError] = useState('');

  const handleSingleQuery = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSingleResult(null);
    setLoading(true);
    try {
      const result = await dnoApi.query(phone, channel);
      setSingleResult(result);
    } catch (err: unknown) {
      setError(axios.isAxiosError(err) ? (err.response?.data as { error?: string })?.error ?? 'Query failed' : 'Query failed');
    } finally {
      setLoading(false);
    }
  };

  const handleBulkQuery = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setBulkResult(null);
    setLoading(true);
    try {
      const numbers = bulkInput
        .split('\n')
        .map((n) => n.trim())
        .filter(Boolean);
      const result = await dnoApi.bulkQuery(numbers, channel);
      setBulkResult(result);
    } catch (err: unknown) {
      setError(axios.isAxiosError(err) ? (err.response?.data as { error?: string })?.error ?? 'Bulk query failed' : 'Bulk query failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-gray-900">Query DNO Database</h1>
        <p className="text-gray-500 mt-1">Check if phone numbers are on the Do Not Originate list</p>
      </div>

      {/* Mode toggle */}
      <div className="flex gap-2 mb-6">
        <button
          onClick={() => setMode('single')}
          className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
            mode === 'single' ? 'bg-blue-600 text-white' : 'bg-white text-gray-600 border border-gray-300'
          }`}
        >
          Single Query
        </button>
        <button
          onClick={() => setMode('bulk')}
          className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
            mode === 'bulk' ? 'bg-blue-600 text-white' : 'bg-white text-gray-600 border border-gray-300'
          }`}
        >
          Bulk Query
        </button>
      </div>

      {error && <div className="bg-red-50 text-red-700 px-4 py-3 rounded-lg text-sm mb-6">{error}</div>}

      {mode === 'single' ? (
        <div className="bg-white rounded-xl p-6 shadow-sm border border-gray-100 mb-6">
          <form onSubmit={(e) => void handleSingleQuery(e)} className="flex gap-4 items-end">
            <div className="flex-1">
              <label className="block text-sm font-medium text-gray-700 mb-1">Phone Number</label>
              <div className="relative">
                <Phone className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
                <input
                  type="text"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  className="w-full pl-10 pr-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
                  placeholder="(555) 123-4567 or 5551234567"
                  required
                />
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Channel</label>
              <select
                value={channel}
                onChange={(e) => setChannel(e.target.value)}
                className="px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
              >
                <option value="voice">Voice</option>
                <option value="text">Text</option>
              </select>
            </div>
            <button
              type="submit"
              disabled={loading}
              className="flex items-center gap-2 px-6 py-2.5 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 font-medium"
            >
              <Search className="w-4 h-4" />
              Query
            </button>
          </form>
        </div>
      ) : (
        <div className="bg-white rounded-xl p-6 shadow-sm border border-gray-100 mb-6">
          <form onSubmit={(e) => void handleBulkQuery(e)} className="space-y-4">
            <div className="flex gap-4 items-end">
              <div className="flex-1">
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Phone Numbers (one per line, max 1000)
                </label>
                <textarea
                  value={bulkInput}
                  onChange={(e) => setBulkInput(e.target.value)}
                  rows={8}
                  className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none font-mono text-sm"
                  placeholder={"5551234567\n5559876543\n8001234567"}
                  required
                />
              </div>
              <div className="flex flex-col gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Channel</label>
                  <select
                    value={channel}
                    onChange={(e) => setChannel(e.target.value)}
                    className="w-full px-4 py-2.5 border border-gray-300 rounded-lg"
                  >
                    <option value="voice">Voice</option>
                    <option value="text">Text</option>
                  </select>
                </div>
                <button
                  type="submit"
                  disabled={loading}
                  className="flex items-center gap-2 px-6 py-2.5 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 font-medium"
                >
                  <Search className="w-4 h-4" />
                  Bulk Query
                </button>
              </div>
            </div>
          </form>
        </div>
      )}

      {/* Single result */}
      {singleResult && (
        <div
          className={`rounded-xl p-6 shadow-sm border ${
            singleResult.isDno
              ? 'bg-red-50 border-red-200'
              : 'bg-green-50 border-green-200'
          }`}
        >
          <div className="flex items-center gap-3">
            {singleResult.isDno ? (
              <XCircle className="w-8 h-8 text-red-500" />
            ) : (
              <CheckCircle className="w-8 h-8 text-green-500" />
            )}
            <div>
              <p className="text-lg font-bold text-gray-900">{singleResult.phoneNumber}</p>
              <p className={`text-sm font-medium ${singleResult.isDno ? 'text-red-700' : 'text-green-700'}`}>
                {singleResult.isDno ? 'ON DNO LIST - Do Not Originate' : 'NOT on DNO list'}
              </p>
            </div>
          </div>
          {singleResult.isDno && (
            <div className="mt-4 grid grid-cols-3 gap-4 text-sm">
              <div>
                <span className="text-gray-500">Dataset:</span>{' '}
                <span className="font-medium text-gray-900">{singleResult.dataset}</span>
              </div>
              <div>
                <span className="text-gray-500">Channel:</span>{' '}
                <span className="font-medium text-gray-900">{singleResult.channel}</span>
              </div>
              <div>
                <span className="text-gray-500">Last Updated:</span>{' '}
                <span className="font-medium text-gray-900">
                  {singleResult.lastUpdated
                    ? new Date(singleResult.lastUpdated).toLocaleDateString()
                    : 'N/A'}
                </span>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Bulk results */}
      {bulkResult && (
        <div>
          <div className="grid grid-cols-3 gap-4 mb-6">
            <div className="bg-white rounded-xl p-4 shadow-sm border border-gray-100 text-center">
              <p className="text-2xl font-bold text-gray-900">{bulkResult.total}</p>
              <p className="text-sm text-gray-500">Total Queried</p>
            </div>
            <div className="bg-red-50 rounded-xl p-4 shadow-sm border border-red-100 text-center">
              <p className="text-2xl font-bold text-red-700">{bulkResult.hits}</p>
              <p className="text-sm text-red-600">DNO Hits</p>
            </div>
            <div className="bg-green-50 rounded-xl p-4 shadow-sm border border-green-100 text-center">
              <p className="text-2xl font-bold text-green-700">{bulkResult.misses}</p>
              <p className="text-sm text-green-600">Clear</p>
            </div>
          </div>

          <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-100">
                <tr>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Phone Number</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Dataset</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Channel</th>
                </tr>
              </thead>
              <tbody>
                {bulkResult.results.map((r) => (
                  <tr key={r.phoneNumber} className="border-b border-gray-50 hover:bg-gray-50">
                    <td className="px-4 py-3 font-mono">{r.phoneNumber}</td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium ${
                          r.isDno ? 'bg-red-100 text-red-700' : 'bg-green-100 text-green-700'
                        }`}
                      >
                        {r.isDno ? 'DNO' : 'Clear'}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-gray-600">{r.dataset ?? '-'}</td>
                    <td className="px-4 py-3 text-gray-600">{r.channel}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
