import { Box } from '@mantine/core';
import { QueryClientProvider } from '@tanstack/react-query';
import type { Meta, StoryObj } from '@storybook/react-vite';
import { MemoryRouter, Route, Routes } from 'react-router';

import { createQueryClient } from '../api/query-client';
import { DashboardPage } from '../pages/dashboard/DashboardPage';
import { PlaceholderPage } from '../pages/PlaceholderPage';
import { AppShell } from './AppShell';

// Load the mock REST layer so DashboardPage queries resolve in Storybook.
await import('../mocks');

const queryClient = createQueryClient();

const meta = {
  title: 'Layout/AppShell',
  component: AppShell,
  parameters: { layout: 'fullscreen' },
} satisfies Meta<typeof AppShell>;

export default meta;

type Story = StoryObj<typeof meta>;

// NOTE: the decorator already wraps in MemoryRouter, but we add Routes here so
// the <Outlet /> inside AppShell renders the Dashboard. Placeholder routes are
// included for all sidebar destinations so navigation works end-to-end in
// Storybook without blank white screens.
export const WithDashboard: Story = {
  render: () => (
    <QueryClientProvider client={queryClient}>
      <Box>
        <Routes>
          <Route element={<AppShell />}>
            <Route index element={<DashboardPage />} />
            <Route path="peers" element={<PlaceholderPage title="Peers" />} />
            <Route
              path="subscribers"
              element={<PlaceholderPage title="Subscribers" />}
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
      </Box>
    </QueryClientProvider>
  ),
};

// Blank shell (outlet empty) — useful to review chrome in isolation
export const Empty: Story = {
  render: () => (
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route element={<AppShell />}>
          <Route index element={<div style={{ padding: 24 }}>Content area</div>} />
        </Route>
      </Routes>
    </MemoryRouter>
  ),
};
