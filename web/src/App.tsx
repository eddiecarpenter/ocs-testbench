import { MantineProvider } from '@mantine/core';
import { Notifications } from '@mantine/notifications';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Route, Routes } from 'react-router';

import { theme } from './theme/theme';
import { AppShell } from './layout/AppShell';
import { DashboardPage } from './pages/dashboard/DashboardPage';
import { PlaceholderPage } from './pages/PlaceholderPage';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  },
});

export function App() {
  return (
    <MantineProvider theme={theme} defaultColorScheme="auto">
      <QueryClientProvider client={queryClient}>
        <Notifications position="top-right" />
        <Routes>
          <Route element={<AppShell />}>
            <Route index element={<DashboardPage />} />
            <Route path="peers" element={<PlaceholderPage title="Peers" />} />
            <Route
              path="subscribers"
              element={<PlaceholderPage title="Subscribers" />}
            />
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
      </QueryClientProvider>
    </MantineProvider>
  );
}
