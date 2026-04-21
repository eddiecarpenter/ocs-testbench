import type { Meta, StoryObj } from '@storybook/react-vite';

import { buildResponseTimeSeries } from '../../mocks/data/metrics';
import { ResponseTimeCard } from './ResponseTimeCard';

const HOUR_MS = 60 * 60 * 1000;

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
    data: buildResponseTimeSeries('PT1H', HOUR_MS, 30).points,
    rangeLabel: 'Last hour',
  },
};

export const FullRun: Story = {
  args: {
    data: buildResponseTimeSeries('PT1H', HOUR_MS, 60).points,
    rangeLabel: 'Full run',
  },
};
