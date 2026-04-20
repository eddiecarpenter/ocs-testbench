import type { Meta, StoryObj } from '@storybook/react-vite';
import { SimpleGrid } from '@mantine/core';
import { KpiCard } from './KpiCard';
import { mockKpis } from '../../mock/dashboard';

const meta = {
  title: 'Dashboard/KpiCard',
  component: KpiCard,
  tags: ['autodocs'],
} satisfies Meta<typeof KpiCard>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    stat: mockKpis[0],
  },
};

export const SubscribersStat: Story = {
  args: {
    stat: mockKpis[1],
  },
};

export const AllFive: Story = {
  args: {
    stat: mockKpis[0],
  },
  render: () => (
    <SimpleGrid cols={5} spacing="md" maw={1100}>
      {mockKpis.map((stat) => (
        <KpiCard key={stat.label} stat={stat} />
      ))}
    </SimpleGrid>
  ),
};
