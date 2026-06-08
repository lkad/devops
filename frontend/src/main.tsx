import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import './styles/global.css';
import { AppShell } from './components/layout/AppShell';
import { ToastProvider } from './components/common/Toast';
import { K8sClusters } from './pages/K8sClusters';
import { Pipelines } from './pages/Pipelines';
import { Discovery } from './pages/Discovery';

function Login() {
  return <div>TODO</div>;
}
function Dashboard() {
  return <div>TODO</div>;
}
function Projects() {
  return <div>TODO</div>;
}
function Devices() {
  return <div>TODO</div>;
}
function PhysicalHosts() {
  return <div>TODO</div>;
}
function Logs() {
  return <div>TODO</div>;
}
function Metrics() {
  return <div>TODO</div>;
}
function Alerts() {
  return <div>TODO</div>;
}
function Audit() {
  return <div>TODO</div>;
}

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
  </StrictMode>,
);
