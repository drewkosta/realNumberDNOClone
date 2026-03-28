import { useQuery } from '@tanstack/react-query';
import { analyticsApi } from '../api';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
} from 'recharts';
import { Database, Activity, ShieldCheck, TrendingUp, Plus, Minus } from 'lucide-react';

const COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6'];

export default function DashboardPage() {
  const { data: analytics, isLoading } = useQuery({
    queryKey: ['analytics'],
    queryFn: analyticsApi.getSummary,
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
      </div>
    );
  }

  if (!analytics) return null;

  const datasetData = Object.entries(analytics.byDataset).map(([name, value]) => ({
    name: name.replace('_', ' ').toUpperCase(),
    value,
  }));

  const channelData = Object.entries(analytics.byChannel).map(([name, value]) => ({
    name: name.charAt(0).toUpperCase() + name.slice(1),
    value,
  }));

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
        <p className="text-gray-500 mt-1">RealNumber DNO Analytics Overview</p>
      </div>

      {/* Stats cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        <StatCard
          icon={Database}
          label="Total DNO Numbers"
          value={analytics.totalDnoNumbers.toLocaleString()}
          color="blue"
        />
        <StatCard
          icon={Activity}
          label="Queries (24h)"
          value={analytics.totalQueries24h.toLocaleString()}
          color="green"
        />
        <StatCard
          icon={ShieldCheck}
          label="Hit Rate (24h)"
          value={`${analytics.hitRate24h.toFixed(1)}%`}
          color="amber"
        />
        <StatCard
          icon={TrendingUp}
          label="Active Numbers"
          value={analytics.activeNumbers.toLocaleString()}
          color="purple"
        />
      </div>

      {/* Recent activity cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
        <div className="bg-white rounded-xl p-6 shadow-sm border border-gray-100">
          <div className="flex items-center gap-2 mb-1">
            <Plus className="w-4 h-4 text-green-500" />
            <span className="text-sm text-gray-500">Additions (7 days)</span>
          </div>
          <p className="text-3xl font-bold text-gray-900">{analytics.recentAdditions}</p>
        </div>
        <div className="bg-white rounded-xl p-6 shadow-sm border border-gray-100">
          <div className="flex items-center gap-2 mb-1">
            <Minus className="w-4 h-4 text-red-500" />
            <span className="text-sm text-gray-500">Removals (7 days)</span>
          </div>
          <p className="text-3xl font-bold text-gray-900">{analytics.recentRemovals}</p>
        </div>
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        {/* Queries by hour */}
        <div className="bg-white rounded-xl p-6 shadow-sm border border-gray-100">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Queries by Hour (24h)</h2>
          {analytics.queriesByHour && analytics.queriesByHour.length > 0 ? (
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={analytics.queriesByHour}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis
                  dataKey="hour"
                  tickFormatter={(v) => v.split(' ')[1] || v}
                  fontSize={12}
                />
                <YAxis fontSize={12} />
                <Tooltip />
                <Bar dataKey="count" fill="#3b82f6" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <div className="flex items-center justify-center h-[300px] text-gray-400">
              No query data yet
            </div>
          )}
        </div>

        {/* Distribution by dataset */}
        <div className="bg-white rounded-xl p-6 shadow-sm border border-gray-100">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Distribution by Dataset</h2>
          {datasetData.length > 0 ? (
            <ResponsiveContainer width="100%" height={300}>
              <PieChart>
                <Pie
                  data={datasetData}
                  cx="50%"
                  cy="50%"
                  outerRadius={100}
                  dataKey="value"
                  label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                >
                  {datasetData.map((_, index) => (
                    <Cell key={index} fill={COLORS[index % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          ) : (
            <div className="flex items-center justify-center h-[300px] text-gray-400">
              No data yet
            </div>
          )}
        </div>
      </div>

      {/* Channel distribution */}
      <div className="bg-white rounded-xl p-6 shadow-sm border border-gray-100">
        <h2 className="text-lg font-semibold text-gray-900 mb-4">Distribution by Channel</h2>
        <div className="flex gap-8 justify-center">
          {channelData.map((item) => (
            <div key={item.name} className="text-center">
              <p className="text-3xl font-bold text-gray-900">{item.value.toLocaleString()}</p>
              <p className="text-sm text-gray-500">{item.name}</p>
            </div>
          ))}
          {channelData.length === 0 && <p className="text-gray-400">No data yet</p>}
        </div>
      </div>
    </div>
  );
}

function StatCard({
  icon: Icon,
  label,
  value,
  color,
}: {
  icon: typeof Database;
  label: string;
  value: string;
  color: string;
}) {
  const colors: Record<string, string> = {
    blue: 'bg-blue-50 text-blue-600',
    green: 'bg-green-50 text-green-600',
    amber: 'bg-amber-50 text-amber-600',
    purple: 'bg-purple-50 text-purple-600',
  };

  return (
    <div className="bg-white rounded-xl p-6 shadow-sm border border-gray-100">
      <div className={`inline-flex p-2 rounded-lg ${colors[color]} mb-3`}>
        <Icon className="w-5 h-5" />
      </div>
      <p className="text-sm text-gray-500">{label}</p>
      <p className="text-2xl font-bold text-gray-900 mt-1">{value}</p>
    </div>
  );
}
