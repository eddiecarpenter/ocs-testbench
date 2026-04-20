import { Card, Stack, Text } from '@mantine/core';
import { NavLink as RouterLink } from 'react-router';
import type { KpiStat } from '../../types/dashboard';
import classes from './KpiCard.module.css';

interface KpiCardProps {
  stat: KpiStat;
}

export function KpiCard({ stat }: KpiCardProps) {
  const isLink = Boolean(stat.to);

  return (
    <Card
      padding="lg"
      withBorder
      shadow="xs"
      component={isLink ? RouterLink : 'div'}
      {...(isLink
        ? {
            to: stat.to!,
            'aria-label': `Go to ${stat.label}`,
          }
        : {})}
      className={isLink ? classes.clickable : undefined}
    >
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
