import { useState } from 'react';
import { adminApi } from '../api';
import { useAuth } from '../auth';
import { Shield, UserPlus, CheckCircle } from 'lucide-react';

export default function AdminPage() {
  const { user } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  const [role, setRole] = useState('operator');
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState('');
  const [error, setError] = useState('');

  if (user?.role !== 'admin') {
    return (
      <div className="flex flex-col items-center justify-center h-64 text-gray-400">
        <Shield className="w-12 h-12 mb-3" />
        <p className="text-lg font-medium">Admin access required</p>
      </div>
    );
  }

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess('');
    setLoading(true);
    try {
      const newUser = await adminApi.createUser({
        email,
        password,
        firstName,
        lastName,
        role,
      });
      setSuccess(`User ${newUser.email} created successfully`);
      setEmail('');
      setPassword('');
      setFirstName('');
      setLastName('');
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to create user');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-gray-900">Administration</h1>
        <p className="text-gray-500 mt-1">Manage users and system configuration</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Create user */}
        <div className="bg-white rounded-xl p-6 shadow-sm border border-gray-100">
          <div className="flex items-center gap-3 mb-6">
            <UserPlus className="w-5 h-5 text-blue-600" />
            <h2 className="text-lg font-semibold text-gray-900">Create User</h2>
          </div>

          <form onSubmit={handleCreateUser} className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">First Name</label>
                <input
                  type="text"
                  value={firstName}
                  onChange={(e) => setFirstName(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Last Name</label>
                <input
                  type="text"
                  value={lastName}
                  onChange={(e) => setLastName(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
                  required
                />
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
                required
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 outline-none"
                minLength={6}
                required
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Role</label>
              <select
                value={role}
                onChange={(e) => setRole(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm"
              >
                <option value="viewer">Viewer - Read-only access</option>
                <option value="operator">Operator - Can manage numbers</option>
                <option value="org_admin">Org Admin - Can manage organization</option>
                <option value="admin">Admin - Full system access</option>
              </select>
            </div>

            {error && (
              <div className="bg-red-50 text-red-700 px-4 py-3 rounded-lg text-sm">{error}</div>
            )}
            {success && (
              <div className="bg-green-50 text-green-700 px-4 py-3 rounded-lg text-sm flex items-center gap-2">
                <CheckCircle className="w-4 h-4" />
                {success}
              </div>
            )}

            <button
              type="submit"
              disabled={loading}
              className="w-full px-4 py-2.5 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700 disabled:opacity-50"
            >
              {loading ? 'Creating...' : 'Create User'}
            </button>
          </form>
        </div>

        {/* System info */}
        <div className="bg-white rounded-xl p-6 shadow-sm border border-gray-100">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">System Information</h2>
          <div className="space-y-4">
            <div className="flex justify-between py-3 border-b border-gray-100">
              <span className="text-sm text-gray-500">System</span>
              <span className="text-sm font-medium text-gray-900">RealNumber DNO Clone</span>
            </div>
            <div className="flex justify-between py-3 border-b border-gray-100">
              <span className="text-sm text-gray-500">Backend</span>
              <span className="text-sm font-medium text-gray-900">Go + Chi Router</span>
            </div>
            <div className="flex justify-between py-3 border-b border-gray-100">
              <span className="text-sm text-gray-500">Frontend</span>
              <span className="text-sm font-medium text-gray-900">React + TypeScript + Tailwind</span>
            </div>
            <div className="flex justify-between py-3 border-b border-gray-100">
              <span className="text-sm text-gray-500">Database</span>
              <span className="text-sm font-medium text-gray-900">SQLite (WAL mode)</span>
            </div>
            <div className="flex justify-between py-3 border-b border-gray-100">
              <span className="text-sm text-gray-500">Data Sets</span>
              <span className="text-sm font-medium text-gray-900">Auto, Subscriber, ITG, TSS Registry</span>
            </div>
            <div className="flex justify-between py-3">
              <span className="text-sm text-gray-500">Channels</span>
              <span className="text-sm font-medium text-gray-900">Voice, Text, Both</span>
            </div>
          </div>

          <div className="mt-6 p-4 bg-amber-50 rounded-lg">
            <p className="text-sm text-amber-800 font-medium">FCC Compliance Note</p>
            <p className="text-xs text-amber-600 mt-1">
              As of December 15, 2025, FCC enforcement requires service providers to use a
              "reasonable" DNO list. Gateway providers are required to block foreign-originated
              illegal calls referencing DNO numbers.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
