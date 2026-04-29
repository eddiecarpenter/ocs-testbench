import { MantineProvider } from '@mantine/core';
import { Notifications } from '@mantine/notifications';
import { QueryClientProvider } from '@tanstack/react-query';
import { Route, Routes } from 'react-router';

import { createQueryClient } from './api/query-client';
import { usePeerStatusToasts } from './api/resources/usePeerStatusToasts';
import { SseProvider } from './api/sse/SseProvider';
import { ErrorProvider } from './context/error/ErrorProvider';
import { theme } from './theme/theme';
import { AppShell } from './layout/AppShell';
import { ScenariosListPage } from './pages/scenarios/ScenariosPage';
import { DashboardPage } from './pages/dashboard/DashboardPage';
import { ExecutionsPage } from './pages/executions/ExecutionsPage';
import { ExecutionDebuggerStubPage } from './pages/executions/ExecutionDebuggerStubPage';
import { PeersPage } from './pages/peers/PeersPage';
import { SettingsPage } from './pages/settings/SettingsPage';
import { SubscribersPage } from './pages/subscribers/SubscribersPage';

const queryClient = createQueryClient();

export function App() {
  return (
    <ErrorProvider>
      <MantineProvider theme={theme} defaultColorScheme="auto">
        <QueryClientProvider client={queryClient}>
          <SseProvider>
            <Notifications position="top-right" />
            <GlobalPeerToasts />
            <Routes>
            <Route element={<AppShell />}>
              <Route index element={<DashboardPage />} />
              <Route path="peers" element={<PeersPage />} />
              <Route path="subscribers" element={<SubscribersPage />} />
              <Route path="scenarios" element={<ScenariosListPage />} />
              <Route path="scenarios/new" element={<ScenariosListPage />} />
              <Route path="scenarios/:id" element={<ScenariosListPage />} />
              <Route path="executions" element={<ExecutionsPage />} />
              <Route
                path="executions/:id"
                element={<ExecutionDebuggerStubPage />}
              />
              <Route path="settings" element={<SettingsPage />} />
            </Route>
          </Routes>
          </SseProvider>
        </QueryClientProvider>
      </MantineProvider>
    </ErrorProvider>
  );
}

/**
 * Empty mount point for the global peer-status toast subscription. Has
 * to live inside `QueryClientProvider` — a plain hook call in `App`
 * would run above it.
 */
function GlobalPeerToasts() {
  usePeerStatusToasts();
  return null;
}
