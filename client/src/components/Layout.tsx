import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import { useAuth } from '../auth';
import {
  LayoutDashboard,
  Search,
  List,
  Upload,
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
      <aside className="w-64 bg-slate-900 text-white flex flex-col">
        <div className="p-6 border-b border-slate-700">
          <div className="flex items-center gap-3">
            <Phone className="w-8 h-8 text-blue-400" />
            <div>
              <h1 className="text-lg font-bold">FakeNumber</h1>
              <p className="text-xs text-slate-400">Do Not Originate</p>
            </div>
          </div>
        </div>

        <nav className="flex-1 p-4 space-y-1">
          {navItems.map((item) => {
            if (item.adminOnly && user?.role !== 'admin') return null;
            return (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  `flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors ${
                    isActive
                      ? 'bg-blue-600 text-white'
                      : 'text-slate-300 hover:bg-slate-800 hover:text-white'
                  }`
                }
              >
                <item.icon className="w-5 h-5" />
                {item.label}
              </NavLink>
            );
          })}
        </nav>

        <div className="p-4 border-t border-slate-700">
          <div className="flex items-center justify-between">
            <div className="text-sm">
              <p className="text-white font-medium">
                {user?.firstName} {user?.lastName}
              </p>
              <p className="text-slate-400 text-xs">{user?.role}</p>
            </div>
            <button
              onClick={handleLogout}
              className="p-2 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition-colors"
              title="Logout"
            >
              <LogOut className="w-5 h-5" />
            </button>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <div className="p-8">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
