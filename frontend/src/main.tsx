import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import './styles/global.css';
import { AppShell } from './components/layout/AppShell';
import { ToastProvider } from './components/common/Toast';
import { Login } from './pages/Login';
import { Dashboard } from './pages/Dashboard';
import { Projects } from './pages/Projects';
import { Devices } from './pages/Devices';
import { PhysicalHosts } from './pages/PhysicalHosts';
import { K8sClusters } from './pages/K8sClusters';
import { Discovery } from './pages/Discovery';
import { Pipelines } from './pages/Pipelines';
import { Services } from './pages/Services';
import { Logs } from './pages/Logs';
import { Metrics } from './pages/Metrics';
import { Alerts } from './pages/Alerts';
import { Audit } from './pages/Audit';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <ToastProvider>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route element={<AppShell />}>
            <Route path="/" element={<Dashboard />} />
            <Route path="/projects" element={<Projects />} />
            <Route path="/devices" element={<Devices />} />
            <Route path="/physical-hosts" element={<PhysicalHosts />} />
            <Route path="/k8s" element={<K8sClusters />} />
            <Route path="/discovery" element={<Discovery />} />
            <Route path="/pipelines" element={<Pipelines />} />
            <Route path="/services" element={<Services />} />
            <Route path="/logs" element={<Logs />} />
            <Route path="/metrics" element={<Metrics />} />
            <Route path="/alerts" element={<Alerts />} />
            <Route path="/audit" element={<Audit />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </ToastProvider>
    </BrowserRouter>
  </StrictMode>
);
