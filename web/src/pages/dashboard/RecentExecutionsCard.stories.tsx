import type { Meta, StoryObj } from '@storybook/react-vite';
import { RecentExecutionsCard } from './RecentExecutionsCard';
import { mockExecutions } from '../../mock/dashboard';

const meta = {
  title: 'Dashboard/RecentExecutionsCard',
  component: RecentExecutionsCard,
  tags: ['autodocs'],
  parameters: { layout: 'padded' },
} satisfies Meta<typeof RecentExecutionsCard>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { executions: mockExecutions },
};

export const AllSuccess: Story = {
  args: {
    executions: mockExecutions.map((e) => ({ ...e, result: 'success' as const })),
  },
};

export const Empty: Story = {
  args: { executions: [] },
};
