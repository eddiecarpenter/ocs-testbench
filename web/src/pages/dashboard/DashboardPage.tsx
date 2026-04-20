import { Grid, SimpleGrid, Stack, Text, Title } from '@mantine/core';
import {
  mockExecutions,
  mockKpis,
  mockPeers,
  mockResponseTime,
} from '../../mock/dashboard';
import { KpiCard } from './KpiCard';
import { PeerStatusCard } from './PeerStatusCard';
import { RecentExecutionsCard } from './RecentExecutionsCard';
import { ResponseTimeCard } from './ResponseTimeCard';

export function DashboardPage() {
  const responseTime = mockResponseTime();

  return (
    <Stack gap="lg" p="md">
      <Stack gap={4}>
        <Title order={2} fw={600}>
          Dashboard
        </Title>
        <Text c="dimmed" size="sm">
          Overview of testbench state and recent activity
        </Text>
      </Stack>

      <SimpleGrid cols={{ base: 1, xs: 2, md: 5 }} spacing="md">
        {mockKpis.map((stat) => (
          <KpiCard key={stat.label} stat={stat} />
        ))}
      </SimpleGrid>

      <Grid>
        <Grid.Col span={{ base: 12, md: 6 }}>
          <PeerStatusCard peers={mockPeers} />
        </Grid.Col>
        <Grid.Col span={{ base: 12, md: 6 }}>
          <RecentExecutionsCard executions={mockExecutions} />
        </Grid.Col>
      </Grid>

      <ResponseTimeCard data={responseTime} />
    </Stack>
  );
}
