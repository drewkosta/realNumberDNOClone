import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { auditApi } from '../api';
import { ChevronLeft, ChevronRight, ScrollText } from 'lucide-react';

export default function AuditPage() {
  const [page, setPage] = useState(1);

  const { data, isLoading } = useQuery({
    queryKey: ['audit-log', page],
    queryFn: () => auditApi.getLog(page, 25),
  });

  const actionColors: Record<string, string> = {
    add: 'bg-green-100 text-green-700',
    remove: 'bg-red-100 text-red-700',
    update: 'bg-blue-100 text-blue-700',
    login: 'bg-purple-100 text-purple-700',
  };

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-gray-900">Audit Log</h1>
        <p className="text-gray-500 mt-1">Track all changes and actions in the DNO system</p>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        {isLoading ? (
          <div className="flex items-center justify-center h-48">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
          </div>
        ) : data?.data && data.data.length > 0 ? (
          <>
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-100">
                <tr>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Timestamp</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Action</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Entity Type</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">User ID</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Details</th>
                </tr>
              </thead>
              <tbody>
                {data.data.map((entry) => (
                  <tr key={entry.id} className="border-b border-gray-50 hover:bg-gray-50">
                    <td className="px-4 py-3 text-gray-500">
                      {new Date(entry.createdAt).toLocaleString()}
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`px-2 py-1 rounded-full text-xs font-medium ${
                          actionColors[entry.action] ?? 'bg-gray-100 text-gray-600'
                        }`}
                      >
                        {entry.action}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-gray-600">{entry.entityType}</td>
                    <td className="px-4 py-3 text-gray-600">{entry.userId ?? '-'}</td>
                    <td className="px-4 py-3 text-gray-500 max-w-[400px] truncate">
                      {entry.details ?? '-'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>

            {data.totalPages > 1 && (
              <div className="flex items-center justify-between px-4 py-3 border-t border-gray-100">
                <p className="text-sm text-gray-500">
                  Page {data.page} of {data.totalPages} ({data.total} entries)
                </p>
                <div className="flex gap-2">
                  <button
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={page === 1}
                    className="p-2 border border-gray-300 rounded-lg disabled:opacity-50 hover:bg-gray-50"
                  >
                    <ChevronLeft className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => setPage((p) => Math.min(data.totalPages, p + 1))}
                    disabled={page >= data.totalPages}
                    className="p-2 border border-gray-300 rounded-lg disabled:opacity-50 hover:bg-gray-50"
                  >
                    <ChevronRight className="w-4 h-4" />
                  </button>
                </div>
              </div>
            )}
          </>
        ) : (
          <div className="flex flex-col items-center justify-center h-48 text-gray-400">
            <ScrollText className="w-10 h-10 mb-2" />
            <p>No audit log entries yet</p>
          </div>
        )}
      </div>
    </div>
  );
}
