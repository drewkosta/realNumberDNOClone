import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import axios from 'axios';
import { webhookApi } from '../api';
import { Webhook, Plus, Trash2, CheckCircle } from 'lucide-react';

export default function WebhooksPage() {
  const queryClient = useQueryClient();
  const [showAdd, setShowAdd] = useState(false);
  const [url, setUrl] = useState('');
  const [secret, setSecret] = useState('');
  const [events, setEvents] = useState('dno.added,dno.removed');
  const [error, setError] = useState('');

  const { data: webhooks, isLoading } = useQuery({
    queryKey: ['webhooks'],
    queryFn: webhookApi.list,
  });

  const createMutation = useMutation({
    mutationFn: () => webhookApi.create({ url, secret, events }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['webhooks'] });
      setShowAdd(false);
      setUrl('');
      setSecret('');
      setError('');
    },
    onError: (err: unknown) => {
      setError(axios.isAxiosError(err) ? (err.response?.data as { error?: string })?.error ?? 'Failed' : 'Failed');
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => webhookApi.remove(id),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['webhooks'] }),
  });

  return (
    <div>
      <div className="flex items-center justify-between mb-8 animate-fade-up">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Webhooks</h1>
          <p className="text-gray-500 mt-1">Receive real-time push notifications when DNO records change</p>
        </div>
        <button onClick={() => setShowAdd(!showAdd)} className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700">
          <Plus className="w-4 h-4" />
          Add Webhook
        </button>
      </div>

      {showAdd && (
        <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 mb-6 animate-fade-down">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">New Webhook Subscription</h2>
          <form onSubmit={(e) => { e.preventDefault(); createMutation.mutate(); }} className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Endpoint URL</label>
                <input type="url" value={url} onChange={(e) => setUrl(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="https://your-service.com/webhook" required />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Signing Secret</label>
                <input type="text" value={secret} onChange={(e) => setSecret(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="whsec_..." required />
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Events</label>
              <select value={events} onChange={(e) => setEvents(e.target.value)} className="px-3 py-2 border border-gray-300 rounded-lg text-sm">
                <option value="dno.added,dno.removed">All events (added + removed)</option>
                <option value="dno.added">Number added only</option>
                <option value="dno.removed">Number removed only</option>
              </select>
            </div>
            {error && <div className="alert-enter bg-red-50 text-red-700 px-4 py-3 rounded-lg text-sm">{error}</div>}
            <div className="flex gap-3">
              <button type="submit" disabled={createMutation.isPending}
                className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-50">
                {createMutation.isPending ? 'Creating...' : 'Create Webhook'}
              </button>
              <button type="button" onClick={() => setShowAdd(false)} className="px-4 py-2 bg-gray-100 text-gray-700 rounded-lg text-sm hover:bg-gray-200">Cancel</button>
            </div>
          </form>
        </div>
      )}

      <div className="card-hover bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden animate-fade-up stagger-1">
        {isLoading ? (
          <div className="p-8 text-center"><div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto" /></div>
        ) : webhooks && webhooks.length > 0 ? (
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Endpoint</th>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Events</th>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Created</th>
                <th className="text-left px-4 py-3 font-medium text-gray-600">Actions</th>
              </tr>
            </thead>
            <tbody>
              {webhooks.map((wh) => (
                <tr key={wh.id} className="border-b border-gray-50 row-hover">
                  <td className="px-4 py-3 font-mono text-xs max-w-[300px] truncate">{wh.url}</td>
                  <td className="px-4 py-3">
                    {wh.events.split(',').map((e) => (
                      <span key={e} className="inline-block px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-700 mr-1">{e}</span>
                    ))}
                  </td>
                  <td className="px-4 py-3">
                    <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-700">
                      <CheckCircle className="w-3 h-3" /> Active
                    </span>
                  </td>
                  <td className="px-4 py-3 text-gray-500">{new Date(wh.createdAt).toLocaleDateString()}</td>
                  <td className="px-4 py-3">
                    <button onClick={() => deleteMutation.mutate(wh.id)} className="p-1 text-red-500 hover:bg-red-50 rounded" title="Delete">
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <div className="flex flex-col items-center justify-center h-48 text-gray-400">
            <Webhook className="w-10 h-10 mb-2" />
            <p>No webhook subscriptions yet</p>
            <p className="text-xs mt-1">Add a webhook to receive real-time DNO change notifications</p>
          </div>
        )}
      </div>

      <div className="mt-6 bg-gray-50 rounded-xl p-6 animate-fade-up stagger-2">
        <h3 className="text-sm font-semibold text-gray-700 mb-2">Webhook Payload Format</h3>
        <pre className="bg-gray-900 text-green-400 p-4 rounded-lg text-xs font-mono overflow-x-auto">
{`POST https://your-service.com/webhook
X-Webhook-Signature: <hmac-sha256-hex>
Content-Type: application/json

{
  "event": "dno.added",
  "phoneNumber": "5551234567",
  "dataset": "subscriber",
  "channel": "voice",
  "timestamp": "2026-03-28T18:00:00Z",
  "orgId": 2
}`}
        </pre>
      </div>
    </div>
  );
}
