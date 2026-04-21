import { Card, Stack, Text } from '@mantine/core';
import type { ElementType } from 'react';
import { NavLink as RouterLink } from 'react-router';

import type { KpiStat } from './kpis';
import classes from './KpiCard.module.css';

interface KpiCardProps {
  stat: KpiStat;
}

export function KpiCard({ stat }: KpiCardProps) {
  const isLink = Boolean(stat.to);
  // Mantine's polymorphic `component` prop doesn't narrow well on a ternary,
  // so we widen to ElementType to let both 'div' and NavLink through.
  const component: ElementType = isLink ? RouterLink : 'div';
  const linkProps = isLink
    ? { to: stat.to!, 'aria-label': `Go to ${stat.label}` }
    : {};

  return (
    <Card
      padding="lg"
      withBorder
      shadow="xs"
      component={component as 'div'}
      {...linkProps}
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
