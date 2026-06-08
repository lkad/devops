import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import './styles/global.css';
import { AppShell } from './components/layout/AppShell';
import { ToastProvider } from './components/common/Toast';
import { Projects } from './pages/Projects';
import { Devices } from './pages/Devices';
import { PhysicalHosts } from './pages/PhysicalHosts';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <ToastProvider>
        <Routes>
          <Route path="/login" element={<div>TODO</div>} />
          <Route element={<AppShell />}>
            <Route path="/" element={<div>TODO</div>} />
            <Route path="/projects" element={<Projects />} />
            <Route path="/devices" element={<Devices />} />
            <Route path="/physical-hosts" element={<PhysicalHosts />} />
            <Route path="/k8s" element={<div>TODO</div>} />
            <Route path="/discovery" element={<div>TODO</div>} />
            <Route path="/pipelines" element={<div>TODO</div>} />
            <Route path="/logs" element={<div>TODO</div>} />
            <Route path="/metrics" element={<div>TODO</div>} />
            <Route path="/alerts" element={<div>TODO</div>} />
            <Route path="/audit" element={<div>TODO</div>} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </ToastProvider>
    </BrowserRouter>
  </StrictMode>
);
