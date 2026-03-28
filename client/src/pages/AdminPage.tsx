import { useState } from 'react';
import axios from 'axios';
import { adminApi } from '../api';
import { useAuth } from '../auth';
import { Shield, UserPlus, CheckCircle, Key, AlertTriangle, Database, RefreshCw } from 'lucide-react';

export default function AdminPage() {
  const { user } = useAuth();

  if (user?.role !== 'admin') {
    return (
      <div className="flex flex-col items-center justify-center h-64 text-gray-400">
        <Shield className="w-12 h-12 mb-3" />
        <p className="text-lg font-medium">Admin access required</p>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-8 animate-fade-up">
        <h1 className="text-2xl font-bold text-gray-900">Administration</h1>
        <p className="text-gray-500 mt-1">Manage users, API keys, and system integrations</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
        <CreateUserCard />
        <APIKeyCard />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
        <ITGIngestCard />
        <IntegrationsCard />
      </div>
    </div>
  );
}

function CreateUserCard() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  const [role, setRole] = useState('operator');
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState('');
  const [error, setError] = useState('');

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(''); setSuccess(''); setLoading(true);
    try {
      const newUser = await adminApi.createUser({ email, password, firstName, lastName, role });
      setSuccess(`User ${newUser.email} created successfully`);
      setEmail(''); setPassword(''); setFirstName(''); setLastName('');
    } catch (err: unknown) {
      setError(axios.isAxiosError(err) ? (err.response?.data as { error?: string })?.error ?? 'Failed to create user' : 'Failed to create user');
    } finally { setLoading(false); }
  };

  return (
    <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-1">
      <div className="flex items-center gap-3 mb-6">
        <UserPlus className="w-5 h-5 text-blue-600" />
        <h2 className="text-lg font-semibold text-gray-900">Create User</h2>
      </div>
      <form onSubmit={(e) => void handleCreateUser(e)} className="space-y-4">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">First Name</label>
            <input type="text" value={firstName} onChange={(e) => setFirstName(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" required />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Last Name</label>
            <input type="text" value={lastName} onChange={(e) => setLastName(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" required />
          </div>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" required />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" minLength={8} required />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Role</label>
          <select value={role} onChange={(e) => setRole(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm">
            <option value="viewer">Viewer</option>
            <option value="operator">Operator</option>
            <option value="org_admin">Org Admin</option>
            <option value="admin">Admin</option>
          </select>
        </div>
        {error && <div className="alert-enter bg-red-50 text-red-700 px-4 py-3 rounded-lg text-sm">{error}</div>}
        {success && <div className="alert-enter bg-green-50 text-green-700 px-4 py-3 rounded-lg text-sm flex items-center gap-2"><CheckCircle className="w-4 h-4" />{success}</div>}
        <button type="submit" disabled={loading} className="w-full px-4 py-2.5 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700 disabled:opacity-50">
          {loading ? 'Creating...' : 'Create User'}
        </button>
      </form>
    </div>
  );
}

function APIKeyCard() {
  const [orgId, setOrgId] = useState('');
  const [generatedKey, setGeneratedKey] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleGenerate = async () => {
    setError(''); setGeneratedKey(''); setLoading(true);
    try {
      const result = await adminApi.generateApiKey(Number(orgId));
      setGeneratedKey(result.apiKey);
    } catch (err: unknown) {
      setError(axios.isAxiosError(err) ? (err.response?.data as { error?: string })?.error ?? 'Failed' : 'Failed');
    } finally { setLoading(false); }
  };

  const handleRevoke = async () => {
    setError(''); setLoading(true);
    try {
      await adminApi.revokeApiKey(Number(orgId));
      setGeneratedKey('');
      setError('');
    } catch (err: unknown) {
      setError(axios.isAxiosError(err) ? (err.response?.data as { error?: string })?.error ?? 'Failed' : 'Failed');
    } finally { setLoading(false); }
  };

  return (
    <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-2">
      <div className="flex items-center gap-3 mb-6">
        <Key className="w-5 h-5 text-blue-600" />
        <h2 className="text-lg font-semibold text-gray-900">API Keys</h2>
      </div>
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Organization ID</label>
          <input type="number" value={orgId} onChange={(e) => setOrgId(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="e.g., 2" />
        </div>
        {error && <div className="alert-enter bg-red-50 text-red-700 px-4 py-3 rounded-lg text-sm">{error}</div>}
        {generatedKey && (
          <div className="alert-enter bg-green-50 border border-green-200 p-4 rounded-lg">
            <p className="text-xs text-green-600 mb-1 font-medium">API Key (copy now -- it won't be shown again):</p>
            <p className="font-mono text-sm text-green-800 break-all select-all">{generatedKey}</p>
          </div>
        )}
        <div className="flex gap-3">
          <button onClick={() => void handleGenerate()} disabled={loading || !orgId}
            className="flex-1 px-4 py-2.5 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700 disabled:opacity-50 text-sm">
            Generate Key
          </button>
          <button onClick={() => void handleRevoke()} disabled={loading || !orgId}
            className="flex-1 px-4 py-2.5 bg-red-600 text-white rounded-lg font-medium hover:bg-red-700 disabled:opacity-50 text-sm">
            Revoke Key
          </button>
        </div>
      </div>
    </div>
  );
}

function ITGIngestCard() {
  const [phone, setPhone] = useState('');
  const [investigationId, setInvestigationId] = useState('');
  const [threatCategory, setThreatCategory] = useState('auto_warranty_scam');
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState('');
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(''); setSuccess(''); setLoading(true);
    try {
      const result = await adminApi.ingestITG({ phoneNumber: phone, investigationId, threatCategory });
      setSuccess(`Added ${result.phoneNumber} to ITG set (${investigationId})`);
      setPhone(''); setInvestigationId('');
    } catch (err: unknown) {
      setError(axios.isAxiosError(err) ? (err.response?.data as { error?: string })?.error ?? 'Failed' : 'Failed');
    } finally { setLoading(false); }
  };

  return (
    <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-3">
      <div className="flex items-center gap-3 mb-6">
        <AlertTriangle className="w-5 h-5 text-red-500" />
        <h2 className="text-lg font-semibold text-gray-900">ITG Traceback Ingest</h2>
      </div>
      <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Phone Number</label>
          <input type="text" value={phone} onChange={(e) => setPhone(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="5551234567" required />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Investigation ID</label>
          <input type="text" value={investigationId} onChange={(e) => setInvestigationId(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="ITG-2026-0042" required />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Threat Category</label>
          <select value={threatCategory} onChange={(e) => setThreatCategory(e.target.value)} className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm">
            <option value="auto_warranty_scam">Auto Warranty Scam</option>
            <option value="irs_impersonation">IRS Impersonation</option>
            <option value="medicare_fraud">Medicare Fraud</option>
            <option value="student_loan_scam">Student Loan Scam</option>
            <option value="tech_support_scam">Tech Support Scam</option>
            <option value="utility_impersonation">Utility Impersonation</option>
            <option value="bank_fraud">Bank Fraud Spoofing</option>
          </select>
        </div>
        {error && <div className="alert-enter bg-red-50 text-red-700 px-4 py-3 rounded-lg text-sm">{error}</div>}
        {success && <div className="alert-enter bg-green-50 text-green-700 px-4 py-3 rounded-lg text-sm flex items-center gap-2"><CheckCircle className="w-4 h-4" />{success}</div>}
        <button type="submit" disabled={loading} className="w-full px-4 py-2.5 bg-red-600 text-white rounded-lg font-medium hover:bg-red-700 disabled:opacity-50">
          {loading ? 'Ingesting...' : 'Add to ITG Set'}
        </button>
      </form>
    </div>
  );
}

function IntegrationsCard() {
  const [tssResult, setTssResult] = useState('');
  const [npacPhone, setNpacPhone] = useState('');
  const [npacStatus, setNpacStatus] = useState('disconnected');
  const [npacResult, setNpacResult] = useState('');
  const [loading, setLoading] = useState('');

  const handleTSSSync = async () => {
    setLoading('tss'); setTssResult('');
    try {
      const result = await adminApi.tssSync();
      setTssResult(`Synced: ${result.numbersAdded} numbers added to text DNO`);
    } catch {
      setTssResult('Sync failed');
    } finally { setLoading(''); }
  };

  const handleNPAC = async () => {
    setLoading('npac'); setNpacResult('');
    try {
      await adminApi.npacEvent({ phoneNumber: npacPhone, newStatus: npacStatus });
      setNpacResult(`Processed: ${npacPhone} -> ${npacStatus}`);
      setNpacPhone('');
    } catch {
      setNpacResult('Failed');
    } finally { setLoading(''); }
  };

  return (
    <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-4">
      <div className="flex items-center gap-3 mb-6">
        <Database className="w-5 h-5 text-blue-600" />
        <h2 className="text-lg font-semibold text-gray-900">Mock Integrations</h2>
      </div>

      {/* TSS Sync */}
      <div className="mb-6 pb-6 border-b border-gray-100">
        <h3 className="text-sm font-semibold text-gray-700 mb-2">TSS Registry Sync</h3>
        <p className="text-xs text-gray-500 mb-3">Sync non-text-enabled toll-free numbers into the text DNO dataset</p>
        <button onClick={() => void handleTSSSync()} disabled={loading === 'tss'}
          className="flex items-center gap-2 px-4 py-2 bg-slate-800 text-white rounded-lg text-sm font-medium hover:bg-slate-900 disabled:opacity-50">
          <RefreshCw className={`w-4 h-4 ${loading === 'tss' ? 'animate-spin' : ''}`} />
          {loading === 'tss' ? 'Syncing...' : 'Run TSS Sync'}
        </button>
        {tssResult && <p className="text-sm text-green-700 mt-2 animate-fade-in">{tssResult}</p>}
      </div>

      {/* NPAC Event */}
      <div>
        <h3 className="text-sm font-semibold text-gray-700 mb-2">NPAC Porting Event</h3>
        <p className="text-xs text-gray-500 mb-3">Simulate a number porting/disconnect event</p>
        <div className="flex gap-2 mb-3">
          <input type="text" value={npacPhone} onChange={(e) => setNpacPhone(e.target.value)}
            className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm" placeholder="5551234567" />
          <select value={npacStatus} onChange={(e) => setNpacStatus(e.target.value)} className="px-3 py-2 border border-gray-300 rounded-lg text-sm">
            <option value="disconnected">Disconnected</option>
            <option value="unassigned">Unassigned</option>
            <option value="ported">Ported</option>
            <option value="assigned">Assigned</option>
          </select>
        </div>
        <button onClick={() => void handleNPAC()} disabled={loading === 'npac' || !npacPhone}
          className="flex items-center gap-2 px-4 py-2 bg-slate-800 text-white rounded-lg text-sm font-medium hover:bg-slate-900 disabled:opacity-50">
          {loading === 'npac' ? 'Processing...' : 'Submit Event'}
        </button>
        {npacResult && <p className="text-sm text-green-700 mt-2 animate-fade-in">{npacResult}</p>}
      </div>
    </div>
  );
}
