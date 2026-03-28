import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import { useAuth } from '../auth';
import {
  LayoutDashboard,
  Search,
  List,
  Upload,
  Scan,
  ShieldCheck,
  Webhook,
  Calculator,
  ScrollText,
  Shield,
  LogOut,
  Phone,
} from 'lucide-react';

const navItems = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard', end: true },
  { to: '/query', icon: Search, label: 'Query Numbers' },
  { to: '/numbers', icon: List, label: 'DNO List' },
  { to: '/bulk', icon: Upload, label: 'Bulk Operations' },
  { to: '/analyzer', icon: Scan, label: 'DNO Analyzer' },
  { to: '/compliance', icon: ShieldCheck, label: 'Compliance' },
  { to: '/webhooks', icon: Webhook, label: 'Webhooks' },
  { to: '/roi', icon: Calculator, label: 'ROI Calculator' },
  { to: '/audit', icon: ScrollText, label: 'Audit Log' },
  { to: '/admin', icon: Shield, label: 'Admin', adminOnly: true },
];

export default function Layout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  const handleLogout = () => {
    logout();
    void navigate('/login');
  };

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Sidebar */}
      <aside className="w-64 bg-slate-900 text-white flex flex-col animate-slide-left">
        <div className="p-6 border-b border-slate-700">
          <div className="flex items-center gap-3 group cursor-default">
            <div className="transition-transform duration-300 group-hover:rotate-12 group-hover:scale-110">
              <Phone className="w-8 h-8 text-blue-400" />
            </div>
            <div>
              <h1 className="text-lg font-bold transition-colors duration-200 group-hover:text-blue-300">FakeNumber</h1>
              <p className="text-xs text-slate-400">Do Not Originate</p>
            </div>
          </div>
        </div>

        <nav className="flex-1 p-4 space-y-1 overflow-y-auto">
          {navItems.map((item, index) => {
            if (item.adminOnly && user?.role !== 'admin') return null;
            return (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  `nav-indicator flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-all duration-200 animate-fade-up ${
                    isActive
                      ? 'active bg-blue-600/90 text-white shadow-lg shadow-blue-600/20'
                      : 'text-slate-300 hover:bg-slate-800 hover:text-white hover:pl-4'
                  }`
                }
                style={{ animationDelay: `${index * 0.04 + 0.1}s` }}
              >
                <item.icon className="w-4 h-4 transition-transform duration-200" />
                {item.label}
              </NavLink>
            );
          })}
        </nav>

        <div className="p-4 border-t border-slate-700 animate-fade-in" style={{ animationDelay: '0.3s' }}>
          <div className="flex items-center justify-between">
            <div className="text-sm">
              <p className="text-white font-medium">
                {user?.firstName} {user?.lastName}
              </p>
              <p className="text-slate-400 text-xs">{user?.role}</p>
            </div>
            <button
              onClick={handleLogout}
              className="p-2 text-slate-400 hover:text-red-400 rounded-lg hover:bg-slate-800 transition-all duration-200 hover:rotate-6"
              title="Logout"
            >
              <LogOut className="w-5 h-5" />
            </button>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <div className="p-8 page-enter">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
