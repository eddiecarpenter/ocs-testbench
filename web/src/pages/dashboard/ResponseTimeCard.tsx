import { Card, Group, Stack, Text, Title } from '@mantine/core';
import { LineChart } from '@mantine/charts';

import type { ResponseTimePoint } from '../../api/resources/metrics';

interface ResponseTimeCardProps {
  data: ResponseTimePoint[];
  rangeLabel?: string;
}

/** Render the bucket timestamp as a short "-12m"/"now" label. */
function formatBucketLabel(points: ResponseTimePoint[]): (t: string) => string {
  if (points.length === 0) return () => '';
  const last = Date.parse(points[points.length - 1].t);
  return (t: string) => {
    const ms = Date.parse(t);
    if (Number.isNaN(ms)) return t;
    const diffMin = Math.round((last - ms) / 60_000);
    if (diffMin <= 0) return 'now';
    if (diffMin < 60) return `-${diffMin}m`;
    const h = Math.round(diffMin / 60);
    return `-${h}h`;
  };
}

export function ResponseTimeCard({
  data,
  rangeLabel = 'Last hour',
}: ResponseTimeCardProps) {
  const tickFormatter = formatBucketLabel(data);

  // Attach a pre-computed display label so Recharts renders stable ticks
  // regardless of how many buckets are present.
  const rows = data.map((p) => ({ ...p, label: tickFormatter(p.t) }));

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
          data={rows}
          dataKey="label"
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
