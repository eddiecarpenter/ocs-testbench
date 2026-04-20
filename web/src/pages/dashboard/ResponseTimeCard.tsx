import { Card, Group, Stack, Text, Title } from '@mantine/core';
import { LineChart } from '@mantine/charts';
import type { ResponseTimePoint } from '../../types/dashboard';

interface ResponseTimeCardProps {
  data: ResponseTimePoint[];
  rangeLabel?: string;
}

export function ResponseTimeCard({
  data,
  rangeLabel = 'Last hour',
}: ResponseTimeCardProps) {
  return (
    <Card padding="lg" withBorder shadow="xs">
      <Stack gap="md">
        <Group justify="space-between" align="center">
          <Title order={5} fw={600}>
            Response time
          </Title>
          <Text size="xs" c="dimmed">
            {rangeLabel}
          </Text>
        </Group>
        <LineChart
          h={200}
          data={data}
          dataKey="t"
          series={[
            { name: 'p50', color: 'teal.6' },
            { name: 'p95', color: 'yellow.6' },
            { name: 'p99', color: 'red.6' },
          ]}
          curveType="monotone"
          withDots={false}
          withLegend
          legendProps={{ verticalAlign: 'top', height: 40 }}
          yAxisProps={{ tickFormatter: (v) => `${v}ms` }}
          gridAxis="y"
        />
      </Stack>
    </Card>
  );
}
