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
      <div className="animate-fade-in">
        <div className="mb-8">
          <div className="skeleton h-8 w-40 mb-2" />
          <div className="skeleton h-5 w-72" />
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="bg-white rounded-xl p-6 shadow-sm border border-gray-100">
              <div className="skeleton h-10 w-10 rounded-lg mb-3" />
              <div className="skeleton h-4 w-24 mb-2" />
              <div className="skeleton h-7 w-20" />
            </div>
          ))}
        </div>
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
      <div className="mb-8 animate-fade-up">
        <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
        <p className="text-gray-500 mt-1">FakeNumber DNO Analytics Overview</p>
      </div>

      {/* Stats cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        <StatCard icon={Database} label="Total DNO Numbers" value={analytics.totalDnoNumbers.toLocaleString()} color="blue" delay={0} />
        <StatCard icon={Activity} label="Queries (24h)" value={analytics.totalQueries24h.toLocaleString()} color="green" delay={1} />
        <StatCard icon={ShieldCheck} label="Hit Rate (24h)" value={`${analytics.hitRate24h.toFixed(1)}%`} color="amber" delay={2} />
        <StatCard icon={TrendingUp} label="Active Numbers" value={analytics.activeNumbers.toLocaleString()} color="purple" delay={3} />
      </div>

      {/* Recent activity cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
        <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-4">
          <div className="flex items-center gap-2 mb-1">
            <Plus className="w-4 h-4 text-green-500" />
            <span className="text-sm text-gray-500">Additions (7 days)</span>
          </div>
          <p className="text-3xl font-bold text-gray-900 animate-count-up">{analytics.recentAdditions}</p>
        </div>
        <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-5">
          <div className="flex items-center gap-2 mb-1">
            <Minus className="w-4 h-4 text-red-500" />
            <span className="text-sm text-gray-500">Removals (7 days)</span>
          </div>
          <p className="text-3xl font-bold text-gray-900 animate-count-up">{analytics.recentRemovals}</p>
        </div>
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-5">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Queries by Hour (24h)</h2>
          {analytics.queriesByHour && analytics.queriesByHour.length > 0 ? (
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={analytics.queriesByHour}>
                <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
                <XAxis dataKey="hour" tickFormatter={(v: string) => v.split(' ')[1] ?? v} fontSize={12} />
                <YAxis fontSize={12} />
                <Tooltip
                  contentStyle={{ borderRadius: '0.75rem', border: '1px solid #e2e8f0', boxShadow: '0 4px 12px rgba(0,0,0,0.08)' }}
                />
                <Bar dataKey="count" fill="#3b82f6" radius={[6, 6, 0, 0]} animationDuration={800} animationEasing="ease-out" />
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <div className="flex items-center justify-center h-[300px] text-gray-400">No query data yet</div>
          )}
        </div>

        <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Distribution by Dataset</h2>
          {datasetData.length > 0 ? (
            <ResponsiveContainer width="100%" height={300}>
              <PieChart>
                <Pie
                  data={datasetData}
                  cx="50%"
                  cy="50%"
                  outerRadius={100}
                  innerRadius={50}
                  dataKey="value"
                  animationDuration={800}
                  animationEasing="ease-out"
                  label={({ name, percent }: { name?: string; percent?: number }) => `${name ?? ''} ${((percent ?? 0) * 100).toFixed(0)}%`}
                >
                  {datasetData.map((entry, index) => (
                    <Cell key={entry.name} fill={COLORS[index % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip contentStyle={{ borderRadius: '0.75rem', border: '1px solid #e2e8f0', boxShadow: '0 4px 12px rgba(0,0,0,0.08)' }} />
              </PieChart>
            </ResponsiveContainer>
          ) : (
            <div className="flex items-center justify-center h-[300px] text-gray-400">No data yet</div>
          )}
        </div>
      </div>

      {/* Channel distribution */}
      <div className="card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up stagger-6">
        <h2 className="text-lg font-semibold text-gray-900 mb-4">Distribution by Channel</h2>
        <div className="flex gap-8 justify-center">
          {channelData.map((item, i) => (
            <div key={item.name} className="text-center animate-count-up" style={{ animationDelay: `${0.3 + i * 0.1}s` }}>
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
  delay,
}: {
  icon: typeof Database;
  label: string;
  value: string;
  color: string;
  delay: number;
}) {
  const colors: Record<string, string> = {
    blue: 'bg-blue-50 text-blue-600',
    green: 'bg-green-50 text-green-600',
    amber: 'bg-amber-50 text-amber-600',
    purple: 'bg-purple-50 text-purple-600',
  };

  const glowColors: Record<string, string> = {
    blue: 'hover:shadow-blue-100',
    green: 'hover:shadow-green-100',
    amber: 'hover:shadow-amber-100',
    purple: 'hover:shadow-purple-100',
  };

  return (
    <div
      className={`card-hover bg-white rounded-xl p-6 shadow-sm border border-gray-100 animate-fade-up ${glowColors[color]}`}
      style={{ animationDelay: `${delay * 0.08}s` }}
    >
      <div className={`inline-flex p-2.5 rounded-xl ${colors[color]} mb-3 transition-transform duration-300 hover:scale-110`}>
        <Icon className="w-5 h-5" />
      </div>
      <p className="text-sm text-gray-500">{label}</p>
      <p className="text-2xl font-bold text-gray-900 mt-1 animate-count-up" style={{ animationDelay: `${delay * 0.08 + 0.15}s` }}>
        {value}
      </p>
    </div>
  );
}
