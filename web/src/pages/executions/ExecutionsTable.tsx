/**
 * Run table for the Executions list page.
 *
 * Columns: # · [Scenario] · Status · Mode · Subscriber · Progress ·
 * Duration · Started · [Actions] — sorted by Started descending. The
 * Scenario column is only shown in the "All runs" view (when no
 * specific scenario is selected). Row click navigates to
 * /executions/<id>; the kebab in the actions column fires a Re-run
 * intent and never bubbles a navigation.
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
} from '@mantine/core';
import { IconDots, IconPlayerPlay } from '@tabler/icons-react';
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
  /**
   * Fired when the user picks Re-run from a row's kebab. The parent
   * decides whether to launch silently (Interactive) or open a confirm
   * modal (Continuous).
   */
  onRerunRow(row: ExecutionSummary): void;
}

export function ExecutionsTable({
  executions,
  showScenarioColumn,
  onRerunRow,
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
          <Table.Th aria-label="Actions" style={{ width: 48 }} />
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
                  <Menu.Item
                    leftSection={<IconPlayerPlay size={14} />}
                    data-testid={`executions-row-${row.id}-rerun`}
                    onClick={(e) => {
                      e.stopPropagation();
                      onRerunRow(row);
                    }}
                  >
                    Re-run
                  </Menu.Item>
                </Menu.Dropdown>
              </Menu>
            </Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  );
}
