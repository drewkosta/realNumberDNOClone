import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import axios from 'axios';
import { dnoApi } from '../api';
import { Plus, Trash2, Download, Search, ChevronLeft, ChevronRight } from 'lucide-react';

export default function NumbersPage() {
  const queryClient = useQueryClient();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [dataset, setDataset] = useState('');
  const [channel, setChannel] = useState('');
  const [statusFilter, setStatusFilter] = useState('active');

  // Add number form
  const [showAdd, setShowAdd] = useState(false);
  const [newPhone, setNewPhone] = useState('');
  const [newType, setNewType] = useState('local');
  const [newChannel, setNewChannel] = useState('voice');
  const [newReason, setNewReason] = useState('');

  const { data, isLoading } = useQuery({
    queryKey: ['dno-numbers', page, search, dataset, channel, statusFilter],
    queryFn: () =>
      dnoApi.listNumbers({ page, pageSize: 25, search, dataset, channel, status: statusFilter }),
  });

  const addMutation = useMutation({
    mutationFn: () =>
      dnoApi.addNumber({
        phoneNumber: newPhone,
        numberType: newType,
        channel: newChannel,
        reason: newReason,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['dno-numbers'] });
      setShowAdd(false);
      setNewPhone('');
      setNewReason('');
    },
  });

  const removeMutation = useMutation({
    mutationFn: ({ phone, ch }: { phone: string; ch: string }) => dnoApi.removeNumber(phone, ch),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['dno-numbers'] }),
  });

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">DNO Number List</h1>
          <p className="text-gray-500 mt-1">Manage the Do Not Originate subscriber list</p>
        </div>
        <div className="flex gap-3">
          <button
            onClick={() => void dnoApi.exportCSV()}
            className="flex items-center gap-2 px-4 py-2 bg-white border border-gray-300 rounded-lg text-sm font-medium text-gray-700 hover:bg-gray-50"
          >
            <Download className="w-4 h-4" />
            Export CSV
          </button>
          <button
            onClick={() => setShowAdd(!showAdd)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700"
          >
            <Plus className="w-4 h-4" />
            Add Number
          </button>
        </div>
      </div>

      {/* Add number form */}
      {showAdd && (
        <div className="bg-white rounded-xl p-6 shadow-sm border border-gray-100 mb-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Add Number to DNO List</h2>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              addMutation.mutate();
            }}
            className="grid grid-cols-1 md:grid-cols-4 gap-4"
          >
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Phone Number</label>
              <input
                type="text"
                value={newPhone}
                onChange={(e) => setNewPhone(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 outline-none text-sm"
                placeholder="5551234567"
                required
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Number Type</label>
              <select
                value={newType}
                onChange={(e) => setNewType(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
              >
                <option value="local">Local (10-digit)</option>
                <option value="toll_free">Toll-Free</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Channel</label>
              <select
                value={newChannel}
                onChange={(e) => setNewChannel(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
              >
                <option value="voice">Voice</option>
                <option value="text">Text</option>
                <option value="both">Both</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Reason</label>
              <input
                type="text"
                value={newReason}
                onChange={(e) => setNewReason(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
                placeholder="Inbound only number"
              />
            </div>
            <div className="md:col-span-4 flex gap-3">
              <button
                type="submit"
                disabled={addMutation.isPending}
                className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
              >
                {addMutation.isPending ? 'Adding...' : 'Add to DNO List'}
              </button>
              <button
                type="button"
                onClick={() => setShowAdd(false)}
                className="px-4 py-2 bg-gray-100 text-gray-700 rounded-lg text-sm font-medium hover:bg-gray-200"
              >
                Cancel
              </button>
              {addMutation.isError && (
                <span className="text-red-600 text-sm self-center">
                  {axios.isAxiosError(addMutation.error) ? (addMutation.error.response?.data as { error?: string })?.error ?? 'Failed to add number' : 'Failed to add number'}
                </span>
              )}
            </div>
          </form>
        </div>
      )}

      {/* Filters */}
      <div className="bg-white rounded-xl p-4 shadow-sm border border-gray-100 mb-6">
        <div className="flex gap-4 items-end flex-wrap">
          <div className="flex-1 min-w-[200px]">
            <label className="block text-xs font-medium text-gray-500 mb-1">Search</label>
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
              <input
                type="text"
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value);
                  setPage(1);
                }}
                className="w-full pl-10 pr-3 py-2 border border-gray-300 rounded-lg text-sm"
                placeholder="Search phone numbers..."
              />
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1">Dataset</label>
            <select
              value={dataset}
              onChange={(e) => {
                setDataset(e.target.value);
                setPage(1);
              }}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm"
            >
              <option value="">All Datasets</option>
              <option value="auto">Auto Set</option>
              <option value="subscriber">Subscriber Set</option>
              <option value="itg">ITG Set</option>
              <option value="tss_registry">TSS Registry</option>
            </select>
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1">Channel</label>
            <select
              value={channel}
              onChange={(e) => {
                setChannel(e.target.value);
                setPage(1);
              }}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm"
            >
              <option value="">All Channels</option>
              <option value="voice">Voice</option>
              <option value="text">Text</option>
            </select>
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1">Status</label>
            <select
              value={statusFilter}
              onChange={(e) => {
                setStatusFilter(e.target.value);
                setPage(1);
              }}
              className="px-3 py-2 border border-gray-300 rounded-lg text-sm"
            >
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
              <option value="">All</option>
            </select>
          </div>
        </div>
      </div>

      {/* Table */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
        {isLoading ? (
          <div className="flex items-center justify-center h-48">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
          </div>
        ) : (
          <>
            <table className="w-full text-sm">
              <thead className="bg-gray-50 border-b border-gray-100">
                <tr>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Phone Number</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Dataset</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Type</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Channel</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Reason</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Updated</th>
                  <th className="text-left px-4 py-3 font-medium text-gray-600">Actions</th>
                </tr>
              </thead>
              <tbody>
                {data?.data && data.data.length > 0 ? (
                  data.data.map((n) => (
                    <tr key={n.id} className="border-b border-gray-50 hover:bg-gray-50">
                      <td className="px-4 py-3 font-mono font-medium">{n.phoneNumber}</td>
                      <td className="px-4 py-3">
                        <span className="px-2 py-1 rounded-full text-xs font-medium bg-blue-100 text-blue-700">
                          {n.dataset}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-gray-600">{n.numberType}</td>
                      <td className="px-4 py-3 text-gray-600">{n.channel}</td>
                      <td className="px-4 py-3">
                        <span
                          className={`px-2 py-1 rounded-full text-xs font-medium ${
                            n.status === 'active'
                              ? 'bg-green-100 text-green-700'
                              : 'bg-gray-100 text-gray-600'
                          }`}
                        >
                          {n.status}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-gray-500 max-w-[200px] truncate">
                        {n.reason ?? '-'}
                      </td>
                      <td className="px-4 py-3 text-gray-500">
                        {new Date(n.updatedAt).toLocaleDateString()}
                      </td>
                      <td className="px-4 py-3">
                        {n.dataset === 'subscriber' && n.status === 'active' && (
                          <button
                            onClick={() =>
                              removeMutation.mutate({ phone: n.phoneNumber, ch: n.channel })
                            }
                            className="p-1 text-red-500 hover:bg-red-50 rounded"
                            title="Remove from DNO list"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        )}
                      </td>
                    </tr>
                  ))
                ) : (
                  <tr>
                    <td colSpan={8} className="px-4 py-12 text-center text-gray-400">
                      No numbers found
                    </td>
                  </tr>
                )}
              </tbody>
            </table>

            {/* Pagination */}
            {data && data.totalPages > 1 && (
              <div className="flex items-center justify-between px-4 py-3 border-t border-gray-100">
                <p className="text-sm text-gray-500">
                  Showing {(data.page - 1) * data.pageSize + 1} to{' '}
                  {Math.min(data.page * data.pageSize, data.total)} of {data.total}
                </p>
                <div className="flex gap-2">
                  <button
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={page === 1}
                    className="p-2 border border-gray-300 rounded-lg disabled:opacity-50 hover:bg-gray-50"
                  >
                    <ChevronLeft className="w-4 h-4" />
                  </button>
                  <span className="px-3 py-2 text-sm text-gray-600">
                    Page {data.page} of {data.totalPages}
                  </span>
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
        )}
      </div>
    </div>
  );
}
