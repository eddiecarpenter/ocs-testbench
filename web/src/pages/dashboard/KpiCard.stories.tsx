import type { Meta, StoryObj } from '@storybook/react-vite';
import { SimpleGrid } from '@mantine/core';

import { KpiCard } from './KpiCard';
import { toKpiStats } from './kpis';

const sampleStats = toKpiStats({
  peers: { connected: 3, total: 5 },
  subscribers: 142,
  scenarios: 24,
  activeRuns: 2,
});

const meta = {
  title: 'Dashboard/KpiCard',
  component: KpiCard,
  tags: ['autodocs'],
} satisfies Meta<typeof KpiCard>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    stat: sampleStats[0],
  },
};

export const SubscribersStat: Story = {
  args: {
    stat: sampleStats[1],
  },
};

export const AllFive: Story = {
  args: {
    stat: sampleStats[0],
  },
  render: () => (
    <SimpleGrid cols={5} spacing="md" maw={1100}>
      {sampleStats.map((stat) => (
        <KpiCard key={stat.label} stat={stat} />
      ))}
    </SimpleGrid>
  ),
};
