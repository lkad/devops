import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import './styles/global.css';
import { AppShell } from './components/layout/AppShell';
import { ToastProvider } from './components/common/Toast';
import { Logs } from './pages/Logs';
import { Metrics } from './pages/Metrics';
import { Alerts } from './pages/Alerts';
import { Audit } from './pages/Audit';

// Stubs — owned by other agents. Will be replaced when those branches merge.
const Login = () => <div>TODO: Login</div>;
const Dashboard = () => <div>TODO: Dashboard</div>;
const Projects = () => <div>TODO: Projects</div>;
const Devices = () => <div>TODO: Devices</div>;
const PhysicalHosts = () => <div>TODO: Physical Hosts</div>;
const K8sClusters = () => <div>TODO: K8s Clusters</div>;
const Discovery = () => <div>TODO: Discovery</div>;
const Pipelines = () => <div>TODO: Pipelines</div>;

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
