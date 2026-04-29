/**
 * Run table for the Executions list page.
 *
 * Columns: # · [Scenario] · Status · Subscriber · Peer · Progress ·
 * Duration · Started · [Actions]. The Scenario column is only shown
 * in the "All runs" view (when no specific scenario is selected).
 * Every column header is clickable and toggles asc/desc; default
 * sort is Started descending. The kebab in the actions column fires
 * either View (always) and Re-run (terminal rows) or Stop (running
 * rows) — kebab clicks never bubble a navigation.
 *
 * The Mode column was dropped — scenario names are wide and the
 * Interactive/Continuous distinction is implicit in the Progress
 * format (`1 / 1` vs `n / m`). Surface mode in the row's hover-card
 * if it ever needs to be visible again.
 *
 * Pure presentation surface; the parent feeds in a pre-filtered
 * `executions` array. Sorting and progress formatting come from
 * `runTableHelpers.ts`, where they're unit-tested.
 */
import {
  ActionIcon,
  Badge,
  Group,
  Menu,
  Table,
  Text,
  UnstyledButton,
} from '@mantine/core';
import {
  IconAdjustments,
  IconArrowDown,
  IconArrowUp,
  IconArrowsSort,
  IconDots,
  IconEye,
  IconPlayerStop,
} from '@tabler/icons-react';
import { useMemo, useState } from 'react';

import type { ExecutionSummary } from '../../api/resources/executions';
import { relativeTime } from '../../utils/relativeTime';

import {
  STATE_COLOR,
  STATE_LABEL,
  type SortDir,
  type SortKey,
  formatDuration,
  formatProgress,
  groupByBatch,
  sortRows,
} from './runTableHelpers';

interface ExecutionsTableProps {
  executions: readonly ExecutionSummary[];
  /** When true, render the additional "Scenario" column ("All runs" view). */
  showScenarioColumn: boolean;
  /**
   * Fired when the user picks "View" — opens the Debugger route for
   * the row. Replaces the previous row-onClick navigation; matches the
   * kebab-driven pattern used on Peers / Subscribers / Scenarios.
   */
  onViewRow(row: ExecutionSummary): void;
  /**
   * Fired when the user picks "Re-run" — opens the Start-Run dialog
   * pre-filled from the row. There is no separate silent re-run path:
   * every re-run goes through the dialog so the user can confirm or
   * tweak parameters (matches the "dialog always" run convention).
   */
  onRerunRow(row: ExecutionSummary): void;
  /**
   * Fired when the user picks "Stop" on a running row. The parent
   * shows a confirmation modal before firing
   * `POST /executions/:id/abort`.
   */
  onStopRow(row: ExecutionSummary): void;
}

interface SortState {
  key: SortKey;
  dir: SortDir;
}

export function ExecutionsTable({
  executions,
  showScenarioColumn,
  onViewRow,
  onRerunRow,
  onStopRow,
}: ExecutionsTableProps) {
  const [sort, setSort] = useState<SortState>({
    key: 'startedAt',
    dir: 'desc',
  });

  // Group by batch first using natural order so every row sees its
  // siblings regardless of the active sort.
  const runsByBatch = useMemo(() => groupByBatch(executions), [executions]);
  const sorted = useMemo(
    () => sortRows(executions, sort.key, sort.dir, runsByBatch),
    [executions, sort, runsByBatch],
  );

  const toggleSort = (key: SortKey) => {
    setSort((prev) =>
      prev.key === key
        ? { key, dir: prev.dir === 'asc' ? 'desc' : 'asc' }
        : { key, dir: key === 'startedAt' ? 'desc' : 'asc' },
    );
  };

  if (sorted.length === 0) {
    return (
      <Text c="dimmed" size="sm" ta="center" py="lg">
        No runs match the current filters.
      </Text>
    );
  }

  const sortIndicator = (key: SortKey) => {
    if (sort.key !== key) {
      return <IconArrowsSort size={12} style={{ opacity: 0.35 }} />;
    }
    return sort.dir === 'asc' ? (
      <IconArrowUp size={12} />
    ) : (
      <IconArrowDown size={12} />
    );
  };

  const sortableHeader = (key: SortKey, label: string, width?: number) => (
    <Table.Th style={width ? { width } : undefined}>
      <UnstyledButton
        onClick={() => toggleSort(key)}
        data-testid={`executions-sort-${key}`}
        aria-label={`Sort by ${label}`}
        style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}
      >
        <Text size="sm" fw={600}>
          {label}
        </Text>
        {sortIndicator(key)}
      </UnstyledButton>
    </Table.Th>
  );

  return (
    <Table highlightOnHover data-testid="executions-table-grid">
      <Table.Thead>
        <Table.Tr>
          {sortableHeader('id', '#', 56)}
          {showScenarioColumn && sortableHeader('scenarioName', 'Scenario')}
          {sortableHeader('state', 'Status')}
          {sortableHeader('subscriber', 'Subscriber')}
          {sortableHeader('peer', 'Peer')}
          {sortableHeader('progress', 'Progress')}
          {sortableHeader('duration', 'Duration')}
          {sortableHeader('startedAt', 'Started')}
          <Table.Th aria-label="Actions" style={{ width: 48 }} />
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {sorted.map((row) => (
          <Table.Tr key={row.id} data-testid={`executions-row-${row.id}`}>
            <Table.Td>
              <Text size="sm" fw={500}>
                {row.id}
              </Text>
            </Table.Td>
            {showScenarioColumn && (
              <Table.Td>
                <Text size="sm">{row.scenarioName}</Text>
              </Table.Td>
            )}
            <Table.Td>
              <Badge
                variant="light"
                color={STATE_COLOR[row.state]}
                data-testid={`executions-row-${row.id}-status`}
              >
                {STATE_LABEL[row.state]}
              </Badge>
            </Table.Td>
            <Table.Td>
              <Text size="sm" c="dimmed">
                {row.subscriberMsisdn ?? row.subscriberId ?? '–'}
              </Text>
            </Table.Td>
            <Table.Td>
              <Text
                size="sm"
                c="dimmed"
                data-testid={`executions-row-${row.id}-peer`}
              >
                {row.peerName ?? row.peerId ?? '–'}
              </Text>
            </Table.Td>
            <Table.Td>
              <Text
                size="sm"
                ff="monospace"
                data-testid={`executions-row-${row.id}-progress`}
              >
                {formatProgress(row, runsByBatch)}
              </Text>
            </Table.Td>
            <Table.Td>
              <Text size="sm" ff="monospace">
                {formatDuration(row)}
              </Text>
            </Table.Td>
            <Table.Td>
              <Group gap={4} wrap="nowrap">
                <Text size="sm">{relativeTime(row.startedAt)}</Text>
              </Group>
            </Table.Td>
            <Table.Td>
              <Menu position="bottom-end" withinPortal>
                <Menu.Target>
                  <ActionIcon
                    variant="subtle"
                    color="gray"
                    aria-label={`Run actions for #${row.id}`}
                    data-testid={`executions-row-${row.id}-kebab`}
                    onClick={(e) => e.stopPropagation()}
                  >
                    <IconDots size={16} />
                  </ActionIcon>
                </Menu.Target>
                <Menu.Dropdown>
                  {/* View — always available */}
                  <Menu.Item
                    leftSection={<IconEye size={14} />}
                    data-testid={`executions-row-${row.id}-view`}
                    onClick={(e) => {
                      e.stopPropagation();
                      onViewRow(row);
                    }}
                  >
                    View
                  </Menu.Item>
                  <Menu.Divider />
                  {row.state === 'running' ? (
                    /* Running rows: only Stop is meaningful — re-running
                       an already-running execution doesn't make sense. */
                    <Menu.Item
                      color="red"
                      leftSection={<IconPlayerStop size={14} />}
                      data-testid={`executions-row-${row.id}-stop`}
                      onClick={(e) => {
                        e.stopPropagation();
                        onStopRow(row);
                      }}
                    >
                      Stop
                    </Menu.Item>
                  ) : (
                    /* Terminal / non-running rows: Re-run goes through
                       the Start-Run dialog pre-filled from this row.
                       (The previous "silent" re-run was removed — every
                       run now confirms via the dialog.) */
                    <Menu.Item
                      leftSection={<IconAdjustments size={14} />}
                      data-testid={`executions-row-${row.id}-rerun`}
                      onClick={(e) => {
                        e.stopPropagation();
                        onRerunRow(row);
                      }}
                    >
                      Re-run
                    </Menu.Item>
                  )}
                </Menu.Dropdown>
              </Menu>
            </Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  );
}
