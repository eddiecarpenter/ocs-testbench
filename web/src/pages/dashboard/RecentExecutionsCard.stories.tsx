import type { Meta, StoryObj } from '@storybook/react-vite';

import { executionFixtures } from '../../mocks/data/executions';
import { RecentExecutionsCard } from './RecentExecutionsCard';

const recent = executionFixtures.slice(0, 5);

const meta = {
  title: 'Dashboard/RecentExecutionsCard',
  component: RecentExecutionsCard,
  tags: ['autodocs'],
  parameters: { layout: 'padded' },
} satisfies Meta<typeof RecentExecutionsCard>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { executions: recent },
};

export const AllSuccess: Story = {
  args: {
    executions: recent.map((e) => ({ ...e, result: 'success' as const })),
  },
};

export const Empty: Story = {
  args: { executions: [] },
};
