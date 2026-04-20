import { Card, Stack, Text } from '@mantine/core';
import type { KpiStat } from '../../types/dashboard';

interface KpiCardProps {
  stat: KpiStat;
}

export function KpiCard({ stat }: KpiCardProps) {
  return (
    <Card padding="lg" withBorder shadow="xs">
      <Stack gap={4} align="center">
        <Text size="sm" c="dimmed" fw={500}>
          {stat.label}
        </Text>
        <Text fz={28} fw={600} lh={1.1}>
          {stat.value}
        </Text>
        {stat.subtitle && (
          <Text size="xs" c="dimmed">
            {stat.subtitle}
          </Text>
        )}
      </Stack>
    </Card>
  );
}
