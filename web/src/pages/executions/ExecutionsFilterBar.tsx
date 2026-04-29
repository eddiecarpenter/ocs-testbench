/**
 * Filter row above the run table — status chips on the left.
 * Drives the `?status=` URL param.
 *
 * Counts come from the parent (single source of truth: the
 * `countByStatusFilter` selector applied to the same execution slice
 * the table is rendering).
 *
 * Note: the previous peer-filter dropdown was removed because peers
 * are bound to scenarios (see Position 3 from #94 scoping); filtering
 * by peer at this level is redundant. The Peer column on the table
 * itself surfaces the value, and column-sort is the user-driven way
 * to organise by peer.
 */
import { Badge, Button, Group } from '@mantine/core';

import type { StatusFilter } from './selectors';

interface ExecutionsFilterBarProps {
  status: StatusFilter;
  counts: Record<StatusFilter, number>;
  onStatusChange(next: StatusFilter): void;
}

const CHIPS: { value: StatusFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running' },
  { value: 'completed', label: 'Completed' },
  { value: 'failed', label: 'Failed' },
];

export function ExecutionsFilterBar({
  status,
  counts,
  onStatusChange,
}: ExecutionsFilterBarProps) {
  return (
    <Group gap="xs" data-testid="executions-status-chips">
      {CHIPS.map((chip) => {
        const active = chip.value === status;
        return (
          <Button
            key={chip.value}
            variant={active ? 'filled' : 'default'}
            size="xs"
            onClick={() => onStatusChange(chip.value)}
            data-active={active || undefined}
            data-testid={`executions-status-${chip.value}`}
            rightSection={
              <Badge
                variant={active ? 'white' : 'light'}
                size="xs"
                color={active ? 'blue' : 'gray'}
              >
                {counts[chip.value]}
              </Badge>
            }
          >
            {chip.label}
          </Button>
        );
      })}
    </Group>
  );
}
