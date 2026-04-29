/**
 * Left pane — step progress list.
 *
 * One row per scenario step. Each row carries:
 *   - the step's short label (e.g. CCR-I)
 *   - a type chip (request | consume | wait | pause)
 *   - a status icon: ✓ success, ✗ failure, ◍ running, ▶ paused (current
 *     cursor on a paused run), ◐ pending
 *   - duration (mm:ss) for completed steps
 *
 * The current cursor's row is highlighted. Clicking a completed step
 * fires `store.viewHistorical(stepIndex)` so the right pane (Task 6)
 * can load the historical CCR + last-response. Clicks on pending /
 * future steps are no-ops — the cursor only advances on Send CCR /
 * Skip (Task 7).
 *
 * Until the first snapshot lands the store has `steps: []` and
 * `totalSteps: 0`. The pane renders a skeleton for that case; once a
 * snapshot is ingested the store carries `totalSteps` and the
 * `StepRecord[]` so the layout is stable.
 *
 * The pure click / state / formatting helpers live in
 * `./progressPaneLogic` so they're unit-testable without a DOM.
 */
import {
  Badge,
  Group,
  Skeleton,
  Stack,
  Text,
  Title,
  UnstyledButton,
} from '@mantine/core';
import {
  IconCheck,
  IconCircleDashed,
  IconLoader2,
  IconPlayerPause,
  IconX,
} from '@tabler/icons-react';
import { useCallback } from 'react';

import type { StepRecord } from '../../api/resources/executions';

import {
  clickAction,
  computeRowState,
  formatDuration,
  isStepCompleted,
  kindColor,
  kindLabel,
  type StepRowState,
} from './progressPaneLogic';
import { useExecutionStore } from './useDebuggerStore';

export function ProgressPane() {
  const totalSteps = useExecutionStore((s) => s.totalSteps);
  const steps = useExecutionStore((s) => s.steps);
  const cursor = useExecutionStore((s) => s.cursor);
  const historicalIndex = useExecutionStore((s) => s.historicalIndex);
  const runState = useExecutionStore((s) => s.state);
  const viewHistorical = useExecutionStore((s) => s.viewHistorical);

  const handleClick = useCallback(
    (stepIndex: number) => {
      const action = clickAction(steps[stepIndex], stepIndex);
      if (!action) return;
      viewHistorical(action.stepIndex);
    },
    [steps, viewHistorical],
  );

  if (totalSteps === 0) {
    return (
      <Stack gap="xs" data-testid="debugger-progress-pane">
        <Title order={5}>Progress</Title>
        <Skeleton height={32} />
        <Skeleton height={32} />
        <Skeleton height={32} />
      </Stack>
    );
  }

  const rows: ProgressRowModel[] = [];
  for (let i = 0; i < totalSteps; i += 1) {
    const record = steps[i];
    rows.push({
      index: i,
      record,
      isCursor: i === cursor,
      isHistoricalSelected: historicalIndex === i,
      runState,
    });
  }

  return (
    <Stack gap="xs" data-testid="debugger-progress-pane">
      <Title order={5}>Progress</Title>
      <Stack gap={4}>
        {rows.map((row) => (
          <ProgressRow
            key={row.index}
            row={row}
            onClick={() => handleClick(row.index)}
          />
        ))}
      </Stack>
    </Stack>
  );
}

interface ProgressRowModel {
  index: number;
  record: StepRecord | undefined;
  isCursor: boolean;
  isHistoricalSelected: boolean;
  runState: string;
}

interface ProgressRowProps {
  row: ProgressRowModel;
  onClick: () => void;
}

function ProgressRow({ row, onClick }: ProgressRowProps) {
  const { record, isCursor, isHistoricalSelected, runState, index } = row;
  const completed = isStepCompleted(record);
  const label = record?.label ?? `Step ${index + 1}`;
  const kind = record?.kind ?? 'request';
  const stateForIcon: StepRowState = computeRowState(record, isCursor, runState);

  const background = isHistoricalSelected
    ? 'var(--mantine-color-blue-light)'
    : isCursor
      ? runState === 'paused'
        ? 'var(--mantine-color-yellow-light)'
        : 'var(--mantine-color-blue-light)'
      : 'transparent';

  return (
    <UnstyledButton
      onClick={onClick}
      disabled={!completed}
      data-testid={`debugger-progress-row-${index}`}
      data-state={stateForIcon}
      data-cursor={isCursor || undefined}
      style={{
        padding: '6px 8px',
        borderRadius: 4,
        background,
        cursor: completed ? 'pointer' : 'default',
        opacity: stateForIcon === 'pending' && !isCursor ? 0.65 : 1,
      }}
    >
      <Group gap="xs" wrap="nowrap" justify="space-between">
        <Group gap="xs" wrap="nowrap" miw={0}>
          <StatusIcon state={stateForIcon} />
          <Text size="sm" fw={isCursor ? 600 : 400} truncate>
            {index + 1}. {label}
          </Text>
          <Badge size="xs" variant="light" color={kindColor(kind)}>
            {kindLabel(kind)}
          </Badge>
        </Group>
        {completed && record?.durationMs !== undefined && (
          <Text size="xs" c="dimmed">
            {formatDuration(record.durationMs)}
          </Text>
        )}
      </Group>
    </UnstyledButton>
  );
}

interface StatusIconProps {
  state: StepRowState;
}

function StatusIcon({ state }: StatusIconProps) {
  switch (state) {
    case 'success':
      return <IconCheck size={14} color="var(--mantine-color-teal-7)" />;
    case 'failure':
    case 'error':
      return <IconX size={14} color="var(--mantine-color-red-7)" />;
    case 'skipped':
      return (
        <IconCircleDashed size={14} color="var(--mantine-color-gray-6)" />
      );
    case 'running':
      return <IconLoader2 size={14} color="var(--mantine-color-blue-6)" />;
    case 'paused':
      return (
        <IconPlayerPause size={14} color="var(--mantine-color-yellow-7)" />
      );
    case 'pending':
    default:
      return (
        <IconCircleDashed size={14} color="var(--mantine-color-gray-5)" />
      );
  }
}
