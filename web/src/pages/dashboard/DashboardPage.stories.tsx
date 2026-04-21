import { QueryClientProvider } from '@tanstack/react-query';
import type { Meta, StoryObj } from '@storybook/react-vite';

import { createQueryClient } from '../../api/query-client';
import { DashboardPage } from './DashboardPage';

// Load the mock REST layer so the dashboard's queries resolve inside Storybook.
// Dynamic import kept out of prod; Storybook always exercises the mocks.
await import('../../mocks');

const queryClient = createQueryClient();

const meta = {
  title: 'Pages/Dashboard',
  component: DashboardPage,
  parameters: { layout: 'fullscreen' },
  decorators: [
    (Story) => (
      <QueryClientProvider client={queryClient}>
        <Story />
      </QueryClientProvider>
    ),
  ],
} satisfies Meta<typeof DashboardPage>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
