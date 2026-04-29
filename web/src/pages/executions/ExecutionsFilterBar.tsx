/**
 * Filter row above the run table — status chips on the left, peer
 * dropdown on the right. Drives `?status=` and `?peer=` URL params.
 *
 * Counts come from the parent (single source of truth: the
 * `countByStatusFilter` selector applied to the same execution slice
 * the table is rendering).
 */
import { Badge, Button, Group, Select } from '@mantine/core';

import type { StatusFilter } from './selectors';

interface ExecutionsFilterBarProps {
  status: StatusFilter;
  counts: Record<StatusFilter, number>;
  onStatusChange(next: StatusFilter): void;

  peerOptions: { value: string; label: string }[];
  peerId: string | null;
  onPeerChange(next: string | null): void;
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
  peerOptions,
  peerId,
  onPeerChange,
}: ExecutionsFilterBarProps) {
  return (
    <Group justify="space-between" align="center" wrap="wrap">
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

      <Select
        placeholder="All peers"
        data={peerOptions}
        value={peerId}
        onChange={onPeerChange}
        clearable
        w={220}
        data-testid="executions-peer-filter"
      />
    </Group>
  );
}
