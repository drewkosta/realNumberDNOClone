import { useState, useRef } from 'react';
import axios from 'axios';
import { dnoApi } from '../api';
import type { BulkUploadResult } from '../types';
import { Upload, FileText, Download, CheckCircle, AlertCircle } from 'lucide-react';

export default function BulkPage() {
  const fileRef = useRef<HTMLInputElement>(null);
  const [channel, setChannel] = useState('voice');
  const [numberType, setNumberType] = useState('local');
  const [file, setFile] = useState<File | null>(null);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<BulkUploadResult | null>(null);
  const [error, setError] = useState('');

  const handleUpload = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!file) return;
    setError('');
    setResult(null);
    setLoading(true);
    try {
      const res = await dnoApi.bulkUpload(file, channel, numberType);
      setResult(res);
    } catch (err: unknown) {
      setError(axios.isAxiosError(err) ? (err.response?.data as { error?: string })?.error ?? 'Upload failed' : 'Upload failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Bulk Operations</h1>
          <p className="text-gray-500 mt-1">Upload CSV files to add numbers or export the DNO database</p>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Upload */}
        <div className="bg-white rounded-xl p-6 shadow-sm border border-gray-100">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Bulk Upload</h2>
          <p className="text-sm text-gray-500 mb-4">
            Upload a CSV file with phone numbers to add to the DNO subscriber set. The CSV should
            have phone numbers in the first column and an optional reason in the second column.
          </p>

          <form onSubmit={(e) => void handleUpload(e)} className="space-y-4">
            <div
              className="border-2 border-dashed border-gray-300 rounded-lg p-8 text-center cursor-pointer hover:border-blue-400 transition-colors"
              onClick={() => fileRef.current?.click()}
            >
              {file ? (
                <div className="flex items-center justify-center gap-3">
                  <FileText className="w-8 h-8 text-blue-500" />
                  <div className="text-left">
                    <p className="text-sm font-medium text-gray-900">{file.name}</p>
                    <p className="text-xs text-gray-500">
                      {(file.size / 1024).toFixed(1)} KB
                    </p>
                  </div>
                </div>
              ) : (
                <>
                  <Upload className="w-10 h-10 text-gray-400 mx-auto mb-3" />
                  <p className="text-sm text-gray-600 mb-1">Click to select a CSV file</p>
                  <p className="text-xs text-gray-400">Max 10MB</p>
                </>
              )}
              <input
                ref={fileRef}
                type="file"
                accept=".csv"
                className="hidden"
                onChange={(e) => setFile(e.target.files?.[0] ?? null)}
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Channel</label>
                <select
                  value={channel}
                  onChange={(e) => setChannel(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
                >
                  <option value="voice">Voice</option>
                  <option value="text">Text</option>
                  <option value="both">Both</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Number Type</label>
                <select
                  value={numberType}
                  onChange={(e) => setNumberType(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
                >
                  <option value="local">Local (10-digit)</option>
                  <option value="toll_free">Toll-Free</option>
                </select>
              </div>
            </div>

            {error && (
              <div className="bg-red-50 text-red-700 px-4 py-3 rounded-lg text-sm">{error}</div>
            )}

            <button
              type="submit"
              disabled={!file || loading}
              className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700 disabled:opacity-50"
            >
              <Upload className="w-4 h-4" />
              {loading ? 'Uploading...' : 'Upload & Process'}
            </button>
          </form>

          {/* Upload result */}
          {result && (
            <div className="mt-6 border-t border-gray-100 pt-4">
              <h3 className="font-medium text-gray-900 mb-3">Upload Results</h3>
              <div className="grid grid-cols-3 gap-3 mb-4">
                <div className="text-center p-3 bg-gray-50 rounded-lg">
                  <p className="text-xl font-bold text-gray-900">{result.total}</p>
                  <p className="text-xs text-gray-500">Total</p>
                </div>
                <div className="text-center p-3 bg-green-50 rounded-lg">
                  <p className="text-xl font-bold text-green-700">{result.success}</p>
                  <p className="text-xs text-green-600">Success</p>
                </div>
                <div className="text-center p-3 bg-red-50 rounded-lg">
                  <p className="text-xl font-bold text-red-700">{result.errors}</p>
                  <p className="text-xs text-red-600">Errors</p>
                </div>
              </div>
              {result.details && result.details.length > 0 && (
                <div className="bg-red-50 rounded-lg p-3 max-h-40 overflow-auto">
                  {result.details.map((d) => (
                    <div key={d} className="flex items-start gap-2 text-xs text-red-700 mb-1">
                      <AlertCircle className="w-3 h-3 mt-0.5 shrink-0" />
                      {d}
                    </div>
                  ))}
                </div>
              )}
              {result.errors === 0 && (
                <div className="flex items-center gap-2 text-green-700 text-sm">
                  <CheckCircle className="w-4 h-4" />
                  All numbers processed successfully
                </div>
              )}
            </div>
          )}
        </div>

        {/* Export */}
        <div className="bg-white rounded-xl p-6 shadow-sm border border-gray-100">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Flat File Export</h2>
          <p className="text-sm text-gray-500 mb-6">
            Download the complete DNO database as a CSV flat file. The export includes all active
            numbers with their dataset, channel, and last update date. This is compatible with the
            FakeNumber DNO flat file format used by carriers and gateway providers for batch
            processing.
          </p>

          <div className="bg-gray-50 rounded-lg p-4 mb-6">
            <h3 className="text-sm font-medium text-gray-700 mb-2">Export Format</h3>
            <div className="text-xs text-gray-500 space-y-1 font-mono">
              <p>phone_number, last_update_date, status_flag, dataset, channel, number_type</p>
              <p className="text-gray-400">---</p>
              <p>5551234567, 2024-01-15T10:30:00Z, 1, subscriber, voice, local</p>
              <p>8001234567, 2024-01-14T08:00:00Z, 0, auto, voice, toll_free</p>
            </div>
            <p className="text-xs text-gray-400 mt-2">
              Status flag: 0 = Auto Set (system), 1 = Subscriber Set (manual)
            </p>
          </div>

          <button
            onClick={() => void dnoApi.exportCSV()}
            className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-slate-800 text-white rounded-lg font-medium hover:bg-slate-900"
          >
            <Download className="w-4 h-4" />
            Download DNO Flat File
          </button>
        </div>
      </div>

      {/* CSV template */}
      <div className="mt-6 bg-white rounded-xl p-6 shadow-sm border border-gray-100">
        <h2 className="text-lg font-semibold text-gray-900 mb-2">CSV Upload Template</h2>
        <p className="text-sm text-gray-500 mb-4">
          Your CSV file should follow this format. The header row is optional and will be
          automatically skipped.
        </p>
        <div className="bg-gray-50 rounded-lg p-4 font-mono text-sm text-gray-700">
          <p className="text-gray-400">phone_number,reason</p>
          <p>5551234567,Customer service inbound only</p>
          <p>5559876543,IVR number - never originates</p>
          <p>8001234567,Toll-free advertising number</p>
        </div>
      </div>
    </div>
  );
}
