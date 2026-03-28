import { Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './auth';
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
        <Route path="bulk" element={<BulkPage />} />
        <Route path="analyzer" element={<AnalyzerPage />} />
        <Route path="compliance" element={<CompliancePage />} />
        <Route path="webhooks" element={<WebhooksPage />} />
        <Route path="roi" element={<ROIPage />} />
        <Route path="audit" element={<AuditPage />} />
        <Route path="admin" element={<AdminPage />} />
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
