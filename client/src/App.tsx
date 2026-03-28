import { Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './auth';
import type { User } from './types';
import Layout from './components/Layout';
import LoginPage from './pages/LoginPage';
import DashboardPage from './pages/DashboardPage';
import QueryPage from './pages/QueryPage';
import NumbersPage from './pages/NumbersPage';
import BulkPage from './pages/BulkPage';
import AnalyzerPage from './pages/AnalyzerPage';
import CompliancePage from './pages/CompliancePage';
import WebhooksPage from './pages/WebhooksPage';
import ROIPage from './pages/ROIPage';
import AuditPage from './pages/AuditPage';
import AdminPage from './pages/AdminPage';

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuth();
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function RoleGuard({ allowed, children }: { allowed: User['role'][]; children: React.ReactNode }) {
  const { user } = useAuth();
  if (!user || !allowed.includes(user.role)) return <Navigate to="/" replace />;
  return <>{children}</>;
}

function AppRoutes() {
  const { isAuthenticated } = useAuth();

  return (
    <Routes>
      <Route path="/login" element={isAuthenticated ? <Navigate to="/" replace /> : <LoginPage />} />
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }
      >
        <Route index element={<DashboardPage />} />
        <Route path="query" element={<QueryPage />} />
        <Route path="numbers" element={<NumbersPage />} />
        <Route path="bulk" element={<RoleGuard allowed={['admin', 'org_admin', 'operator']}><BulkPage /></RoleGuard>} />
        <Route path="analyzer" element={<AnalyzerPage />} />
        <Route path="compliance" element={<CompliancePage />} />
        <Route path="webhooks" element={<RoleGuard allowed={['admin', 'org_admin']}><WebhooksPage /></RoleGuard>} />
        <Route path="roi" element={<ROIPage />} />
        <Route path="audit" element={<AuditPage />} />
        <Route path="admin" element={<RoleGuard allowed={['admin']}><AdminPage /></RoleGuard>} />
      </Route>
    </Routes>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <AppRoutes />
    </AuthProvider>
  );
}
