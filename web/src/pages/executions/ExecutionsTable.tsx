/**
 * Run table for the Executions list page.
 *
 * Columns: # · [Scenario] · Status · Mode · Subscriber · Progress ·
 * Duration · Started — sorted by Started descending. The Scenario
 * column is only shown in the "All runs" view (when no specific
 * scenario is selected). Row click navigates to /executions/<id>.
 *
 * Pure presentation surface; the parent feeds in a pre-filtered
 * `executions` array. Sorting and progress formatting come from
 * `runTableHelpers.ts`, where they're unit-tested.
 */
import { Badge, Group, Table, Text } from '@mantine/core';
import { useMemo } from 'react';
import { useNavigate } from 'react-router';

import type { ExecutionSummary } from '../../api/resources/executions';
import { relativeTime } from '../../utils/relativeTime';

import {
  STATE_COLOR,
  STATE_LABEL,
  formatDuration,
  formatProgress,
  groupByBatch,
  modeLabel,
  sortByStartedDesc,
} from './runTableHelpers';

interface ExecutionsTableProps {
  executions: readonly ExecutionSummary[];
  /** When true, render the additional "Scenario" column ("All runs" view). */
  showScenarioColumn: boolean;
}

export function ExecutionsTable({
  executions,
  showScenarioColumn,
}: ExecutionsTableProps) {
  const navigate = useNavigate();

  const sorted = useMemo(() => sortByStartedDesc(executions), [executions]);
  const runsByBatch = useMemo(() => groupByBatch(sorted), [sorted]);

  if (sorted.length === 0) {
    return (
      <Text c="dimmed" size="sm" ta="center" py="lg">
        No runs match the current filters.
      </Text>
    );
  }

  return (
    <Table highlightOnHover data-testid="executions-table-grid">
      <Table.Thead>
        <Table.Tr>
          <Table.Th style={{ width: 56 }}>#</Table.Th>
          {showScenarioColumn && <Table.Th>Scenario</Table.Th>}
          <Table.Th>Status</Table.Th>
          <Table.Th>Mode</Table.Th>
          <Table.Th>Subscriber</Table.Th>
          <Table.Th>Progress</Table.Th>
          <Table.Th>Duration</Table.Th>
          <Table.Th>Started</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {sorted.map((row) => (
          <Table.Tr
            key={row.id}
            data-testid={`executions-row-${row.id}`}
            style={{ cursor: 'pointer' }}
            onClick={() => navigate(`/executions/${encodeURIComponent(row.id)}`)}
          >
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
              <Badge
                variant="outline"
                size="sm"
                data-testid={`executions-row-${row.id}-mode`}
              >
                {modeLabel(row.mode)}
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
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  );
}
