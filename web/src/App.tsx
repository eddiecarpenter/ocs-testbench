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
import { DashboardPage } from './pages/dashboard/DashboardPage';
import { PeersPage } from './pages/peers/PeersPage';
import { PlaceholderPage } from './pages/PlaceholderPage';
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
              <Route
                path="templates"
                element={<PlaceholderPage title="AVP Templates" />}
              />
              <Route
                path="scenarios"
                element={<PlaceholderPage title="Scenarios" />}
              />
              <Route
                path="execution"
                element={<PlaceholderPage title="Executions" />}
              />
              <Route
                path="settings"
                element={<PlaceholderPage title="Settings" />}
              />
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
