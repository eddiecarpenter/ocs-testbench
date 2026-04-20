import type { Meta, StoryObj } from '@storybook/react-vite';
import { ResponseTimeCard } from './ResponseTimeCard';
import { mockResponseTime } from '../../mock/dashboard';

const meta = {
  title: 'Dashboard/ResponseTimeCard',
  component: ResponseTimeCard,
  tags: ['autodocs'],
  parameters: { layout: 'padded' },
} satisfies Meta<typeof ResponseTimeCard>;

export default meta;

type Story = StoryObj<typeof meta>;

export const LastHour: Story = {
  args: {
    data: mockResponseTime(30),
    rangeLabel: 'Last hour',
  },
};

export const FullRun: Story = {
  args: {
    data: mockResponseTime(60),
    rangeLabel: 'Full run',
  },
};
